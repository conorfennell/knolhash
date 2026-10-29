package fpl

import (
	"log/slog"
	"sync"
	"time"
)

const historyPollInterval = 10 * time.Minute

// HistoryCache keeps each settled gameweek's teams and player points for
// the weekly awards. Settled gameweeks never change, so each is fetched once
// (about one call per manager plus one) and kept for the life of the process.
type HistoryCache struct {
	client *Client
	league *Cache

	mu   sync.RWMutex
	byGW map[int]GameweekHistory // complete gameweeks only; never mutated once stored
}

// NewHistoryCache creates a HistoryCache that reads settled gameweeks and
// managers from league.
func NewHistoryCache(client *Client, league *Cache) *HistoryCache {
	return &HistoryCache{client: client, league: league, byGW: map[int]GameweekHistory{}}
}

// Get returns the stored gameweeks by event.
func (c *HistoryCache) Get() map[int]GameweekHistory {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[int]GameweekHistory, len(c.byGW))
	for k, v := range c.byGW {
		out[k] = v
	}
	return out
}

// Start launches the background goroutine that backfills settled gameweeks.
func (c *HistoryCache) Start() {
	go func() {
		for {
			c.poll()
			time.Sleep(historyPollInterval)
		}
	}()
	slog.Info("fpl: awards history started")
}

func (c *HistoryCache) poll() {
	league := c.league.Get()
	if len(league.Managers) == 0 {
		return
	}
	for _, e := range league.Events {
		if !e.Settled() {
			continue
		}
		c.mu.RLock()
		_, done := c.byGW[e.ID]
		c.mu.RUnlock()
		if done {
			continue
		}
		h, err := c.fetch(league, e.ID)
		if err != nil {
			// Try again next poll; partial gameweeks are never stored, so an
			// award can't be given on missing teams.
			slog.Warn("fpl: awards history fetch failed", "event", e.ID, "error", err)
			continue
		}
		c.mu.Lock()
		c.byGW[e.ID] = h
		c.mu.Unlock()
	}
}

func (c *HistoryCache) fetch(league LeagueData, event int) (GameweekHistory, error) {
	points, err := c.client.FetchLivePoints(event)
	if err != nil {
		return GameweekHistory{}, err
	}
	h := GameweekHistory{Event: event, Picks: map[int]Picks{}, Points: points}
	for _, m := range league.MatchesFor(event) {
		for _, s := range []Side{m.Home, m.Away} {
			if s.IsAverage() {
				continue
			}
			p, err := c.client.FetchPicks(s.Entry, event)
			if err != nil {
				return GameweekHistory{}, err
			}
			h.Picks[s.Entry] = p
		}
	}
	return h, nil
}
