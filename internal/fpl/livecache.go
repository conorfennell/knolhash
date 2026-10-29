package fpl

import (
	"log/slog"
	"sync"
	"time"
)

const (
	// The loop wakes every minute; between gameweeks it only reads the
	// league cache, so it notices a gameweek starting within a minute
	// without calling FPL.
	pollInterval = 1 * time.Minute
	// Chip history changes at most once a gameweek.
	chipsPollInterval = 10 * time.Minute
)

// LiveData is what changes during and around a gameweek: provisional live
// scores, each manager's team for the current gameweek and chip history.
// It is kept apart from LeagueData so the ledger only ever sees settled
// match points. Treat it as read-only.
type LiveData struct {
	Event      int                    // in-progress gameweek, 0 when none
	Scores     map[int]int            // entry → provisional score for Event
	PicksEvent int                    // gameweek the picks below are for
	Picks      map[int]Picks          // entry → team for PicksEvent
	Chips      map[int]map[int]string // entry → event → raw chip name
	FetchedAt  time.Time
}

// ScoreFor is a side's provisional score in the live gameweek. ok is false
// for AVERAGE and for managers whose team couldn't be fetched; callers fall
// back to the points FPL reports.
func (d LiveData) ScoreFor(s Side) (score int, ok bool) {
	if s.IsAverage() {
		return 0, false
	}
	score, ok = d.Scores[s.Entry]
	return score, ok
}

// Chip returns the chip a manager played in a gameweek, or "".
func (d LiveData) Chip(entry, event int) string {
	return d.Chips[entry][event]
}

// LiveCache polls FPL for live scores, teams, chips and availability.
type LiveCache struct {
	client *Client
	league *Cache

	mu   sync.RWMutex
	data LiveData

	// Owned by the polling goroutine.
	picks      map[int]Picks
	picksEvent int
	chipsAt    time.Time
	chips      map[int]map[int]string
}

// NewLiveCache creates a LiveCache that reads the league's managers and
// current gameweek from league.
func NewLiveCache(client *Client, league *Cache) *LiveCache {
	return &LiveCache{client: client, league: league, chips: map[int]map[int]string{}}
}

// Get returns the latest live data.
func (c *LiveCache) Get() LiveData {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

// Start launches the background polling goroutine.
func (c *LiveCache) Start() {
	go func() {
		for {
			c.poll()
			time.Sleep(pollInterval)
		}
	}()
	slog.Info("fpl: live feed started")
}

func (c *LiveCache) poll() {
	league := c.league.Get()

	// Wait for the league cache to know the managers before marking
	// chips as fetched.
	if len(league.Managers) > 0 && time.Since(c.chipsAt) >= chipsPollInterval {
		c.refreshChips(league)
		c.chipsAt = time.Now()
	}

	cur, hasCurrent := league.CurrentEvent()
	if hasCurrent && time.Now().After(cur.Deadline) {
		c.refreshPicks(league, cur.ID)
	}

	data := LiveData{
		PicksEvent: c.picksEvent,
		Picks:      copyPicks(c.picks),
		FetchedAt:  time.Now(),
	}
	if hasCurrent && cur.InProgress() {
		if points, err := c.client.FetchLivePoints(cur.ID); err != nil {
			slog.Warn("fpl: live poll failed", "event", cur.ID, "error", err)
			// Keep the last good scores for this gameweek rather than
			// flipping the page back to FPL's points for a minute.
			if prev := c.Get(); prev.Event == cur.ID {
				data.Event, data.Scores = prev.Event, prev.Scores
			}
		} else {
			data.Event, data.Scores = cur.ID, map[int]int{}
			for entry, p := range c.picks {
				data.Scores[entry] = LiveScore(p, points)
			}
		}
	}
	data.Chips = copyChips(c.chips)

	c.mu.Lock()
	c.data = data
	c.mu.Unlock()
}

func (c *LiveCache) refreshChips(league LeagueData) {
	for _, m := range league.Managers {
		chips, err := c.client.FetchChips(m.Entry)
		if err != nil {
			slog.Warn("fpl: chips fetch failed", "entry", m.Entry, "error", err)
			continue
		}
		// Merge, so a chip learned from picks isn't dropped if /history/
		// hasn't caught up yet.
		for e, chip := range chips {
			c.setChip(m.Entry, e, chip)
		}
	}
}

// refreshPicks fetches each manager's team for the current gameweek. Teams
// are fixed once the deadline has passed, so each is fetched once.
func (c *LiveCache) refreshPicks(league LeagueData, event int) {
	if c.picksEvent != event {
		c.picks, c.picksEvent = map[int]Picks{}, event
	}
	for _, m := range league.Managers {
		if _, ok := c.picks[m.Entry]; ok {
			continue
		}
		p, err := c.client.FetchPicks(m.Entry, event)
		if err != nil {
			slog.Warn("fpl: picks fetch failed", "entry", m.Entry, "error", err)
			continue
		}
		c.picks[m.Entry] = p
		if p.Chip != "" {
			c.setChip(m.Entry, event, p.Chip)
		}
	}
}

func (c *LiveCache) setChip(entry, event int, chip string) {
	if c.chips[entry] == nil {
		c.chips[entry] = map[int]string{}
	}
	c.chips[entry][event] = chip
}

func copyPicks(in map[int]Picks) map[int]Picks {
	out := make(map[int]Picks, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyChips(in map[int]map[int]string) map[int]map[int]string {
	out := make(map[int]map[int]string, len(in))
	for entry, byEvent := range in {
		cp := make(map[int]string, len(byEvent))
		for e, chip := range byEvent {
			cp[e] = chip
		}
		out[entry] = cp
	}
	return out
}
