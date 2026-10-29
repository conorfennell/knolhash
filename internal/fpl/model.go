package fpl

import (
	"strings"
	"time"
)

// LeagueID is Kevin's head-to-head league ("Taking the trophy off of James").
const LeagueID = 428253

// Manager is a real entry in the league.
type Manager struct {
	Entry       int
	PlayerName  string
	TeamName    string
	DisplayName string // first name, plus surname initial when first names clash
}

// Side is one half of an H2H match or a standings row. Entry is 0 for the
// AVERAGE ghost team that FPL adds to leagues with an odd number of entries.
type Side struct {
	Entry    int
	Name     string // display name ("AVERAGE" for the ghost team)
	TeamName string
	Points   int
}

// IsAverage reports whether this side is FPL's AVERAGE ghost team.
func (s Side) IsAverage() bool { return s.Entry == 0 }

// Match is a single H2H fixture in a gameweek.
type Match struct {
	ID    int
	Event int
	Home  Side
	Away  Side
}

// Standing is a row of the FPL H2H league table.
type Standing struct {
	Rank      int
	Side      Side
	Won       int
	Drawn     int
	Lost      int
	Total     int // H2H league points (3 per win, 1 per draw)
	PointsFor int
}

// Event is an FPL gameweek.
type Event struct {
	ID          int
	Name        string
	Deadline    time.Time
	Finished    bool
	DataChecked bool
	IsCurrent   bool
	IsNext      bool
}

// InProgress reports whether the gameweek has started but not finished.
func (e Event) InProgress() bool { return e.IsCurrent && !e.Finished }

// Settled reports whether FPL has finished and confirmed the gameweek's
// scores (bonus points added), so results will no longer change.
func (e Event) Settled() bool { return e.Finished && e.DataChecked }

// Status is "live", "settled", "unconfirmed" (finished but bonus points not
// yet confirmed) or "upcoming".
func (e Event) Status() string {
	switch {
	case e.InProgress():
		return "live"
	case e.Settled():
		return "settled"
	case e.Finished:
		return "unconfirmed"
	}
	return "upcoming"
}

// LeagueData is everything fetched from the FPL API in one refresh. Values
// returned by Cache.Get share slices with the cache, so treat them as
// read-only.
type LeagueData struct {
	LeagueName string
	Managers   []Manager
	Standings  []Standing
	Matches    []Match // all H2H matches, ordered by event; join to Events by ID for status
	Events     []Event
	Players    map[int]Player // element id → availability
	FetchedAt  time.Time
}

// CurrentEvent returns the gameweek FPL marks as current, if any.
func (d LeagueData) CurrentEvent() (Event, bool) {
	for _, e := range d.Events {
		if e.IsCurrent {
			return e, true
		}
	}
	return Event{}, false
}

// NextEvent returns the gameweek FPL marks as next, if any.
func (d LeagueData) NextEvent() (Event, bool) {
	for _, e := range d.Events {
		if e.IsNext {
			return e, true
		}
	}
	return Event{}, false
}

// MatchesFor returns the matches in the given gameweek.
func (d LeagueData) MatchesFor(event int) []Match {
	var out []Match
	for _, m := range d.Matches {
		if m.Event == event {
			out = append(out, m)
		}
	}
	return out
}

// displayNames maps entry id → short display name. Managers are known by
// first name; where two share a first name the surname initial is added
// ("James V", "James P"), and the full name is used if that still clashes.
func displayNames(players map[int]string) map[int]string {
	count := func(f func(string) string) map[string]int {
		c := map[string]int{}
		for _, full := range players {
			c[f(full)]++
		}
		return c
	}
	firstCount := count(firstName)
	initialCount := count(firstAndInitial)
	out := make(map[int]string, len(players))
	for entry, full := range players {
		switch {
		case firstCount[firstName(full)] == 1:
			out[entry] = firstName(full)
		case initialCount[firstAndInitial(full)] == 1:
			out[entry] = firstAndInitial(full)
		default:
			out[entry] = titleWords(full)
		}
	}
	return out
}

func firstName(full string) string {
	parts := strings.Fields(full)
	if len(parts) == 0 {
		return full
	}
	return capitalise(parts[0])
}

func firstAndInitial(full string) string {
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return firstName(full)
	}
	last := []rune(parts[len(parts)-1])
	return firstName(full) + " " + strings.ToUpper(string(last[0]))
}

func titleWords(full string) string {
	parts := strings.Fields(full)
	for i, p := range parts {
		parts[i] = capitalise(p)
	}
	return strings.Join(parts, " ")
}

// capitalise upper-cases the first letter, leaving the rest alone so names
// like "McCarthy" survive.
func capitalise(word string) string {
	r := []rune(word)
	if len(r) == 0 {
		return word
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
