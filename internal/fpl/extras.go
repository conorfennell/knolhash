package fpl

import (
	"fmt"
	"sort"
)

// NamingRight is the season's biggest H2H winning margin. The winner names
// the loser's team next season.
type NamingRight struct {
	Event  int
	Winner Side
	Loser  Side
	Margin int
}

// BiggestMargin returns the biggest H2H winning margin in settled
// gameweeks, ignoring matches against AVERAGE. On a tie the earlier match
// keeps the rights.
func BiggestMargin(data LeagueData) (NamingRight, bool) {
	settled := map[int]bool{}
	for _, e := range data.Events {
		settled[e.ID] = e.Settled()
	}
	var best NamingRight
	found := false
	for _, m := range data.Matches {
		if !settled[m.Event] || m.Home.IsAverage() || m.Away.IsAverage() || m.Home.Points == m.Away.Points {
			continue
		}
		w, l := m.Home, m.Away
		if l.Points > w.Points {
			w, l = l, w
		}
		margin := w.Points - l.Points
		if !found || margin > best.Margin || (margin == best.Margin && m.Event < best.Event) {
			best, found = NamingRight{Event: m.Event, Winner: w, Loser: l, Margin: margin}, true
		}
	}
	return best, found
}

// Player is an FPL player's availability.
type Player struct {
	ID       int
	Name     string // web name, e.g. "Saka"
	Team     string // short club name, e.g. "ARS"
	Status   string // a available, d doubtful, i injured, s suspended, u unavailable, n ineligible
	Chance   *int   // chance of playing next round, nil when unknown
	News     string
	Position int // 1 GK, 2 DEF, 3 MID, 4 FWD
}

// Unavailable reports whether the player is anything less than fully fit.
func (p Player) Unavailable() bool {
	return p.Status != "a" || (p.Chance != nil && *p.Chance < 100)
}

// Out reports whether the player is ruled out (injured, suspended or
// unavailable) rather than just doubtful.
func (p Player) Out() bool {
	switch p.Status {
	case "i", "s", "u", "n":
		return true
	}
	return false
}

// StatusLabel is a short human label for the player's status.
func (p Player) StatusLabel() string {
	switch p.Status {
	case "i":
		return "Injured"
	case "s":
		return "Suspended"
	case "u", "n":
		return "Unavailable"
	}
	if p.Chance != nil {
		return fmt.Sprintf("%d%%", *p.Chance)
	}
	return "Doubtful"
}

// Concern is an unavailable or doubtful player in a manager's team.
type Concern struct {
	Player Player
	Bench  bool
}

// Concerns lists the unavailable or doubtful players in a team, starters
// first, most serious first.
func Concerns(p Picks, players map[int]Player) []Concern {
	var out []Concern
	for _, pk := range p.Players {
		pl, ok := players[pk.Element]
		if !ok || !pl.Unavailable() {
			continue
		}
		out = append(out, Concern{Player: pl, Bench: pk.Multiplier == 0 && p.Chip != "bboost"})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bench != out[j].Bench {
			return !out[i].Bench
		}
		return chance(out[i].Player) < chance(out[j].Player)
	})
	return out
}

func chance(p Player) int {
	switch {
	case p.Out():
		return 0
	case p.Chance != nil:
		return *p.Chance
	}
	return 50
}
