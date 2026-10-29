package fpl

import (
	"fmt"
	"sort"
)

// Money rules, from Kevin's 2025/26 sheet with edge cases agreed on
// 29 Oct 2026 (not yet confirmed by Kevin):
//
//   - H2H: loser pays winner €5; a draw moves nothing; no money against AVERAGE.
//   - Bottom two: managers scoring the lowest or second-lowest score of the
//     gameweek (ties included) pay €5 each.
//   - Top scorer: the highest scorer(s) split the bottom-two payments equally.
//   - Cap: each half (GW1–19, GW20–38), an H2H loss is limited to the
//     loser's headroom: their net half total before the gameweek (all money,
//     as in the sheet's "Can lose" column) minus −€50, floored at €0. The
//     winner gains the reduced amount. Bottom-two payments ignore the cap,
//     so a total can end up below −€50.
//   - AVERAGE never counts for top or bottom.
//   - Only gameweeks FPL has settled (finished and bonus confirmed) count.
const (
	h2hStake    Money = 500
	bottomStake Money = 500
	halfCap     Money = -5000
	halfSplit         = 19 // last gameweek of the first half
)

// Money is an amount in euro cents.
type Money int

func (m Money) String() string {
	sign := ""
	if m < 0 {
		sign, m = "−", -m
	}
	if m%100 == 0 {
		return fmt.Sprintf("%s€%d", sign, m/100)
	}
	return fmt.Sprintf("%s€%d.%02d", sign, m/100, m%100)
}

// Half returns 0 for GW1–19 and 1 for GW20–38.
func Half(event int) int {
	if event <= halfSplit {
		return 0
	}
	return 1
}

// ManagerWeek is one manager's money for one gameweek.
type ManagerWeek struct {
	Entry    int
	Points   int
	H2H      Money // + won, − lost against their opponent
	Bottom   Money // − when in the bottom two
	Top      Money // + share of the bottom-two pot
	Capped   bool  // the H2H loss was reduced by the −€50 cap
	Opponent int   // entry id, 0 for AVERAGE
}

// Total is the manager's net money for the gameweek.
func (w ManagerWeek) Total() Money { return w.H2H + w.Bottom + w.Top }

// GameweekLedger is the money for one settled gameweek.
type GameweekLedger struct {
	Event      int
	Weeks      map[int]ManagerWeek // by entry id
	TopScorers []int               // entry ids, sorted
	BottomTwo  []int               // entry ids, sorted
}

// Pot is the bottom-two payments shared by the top scorer(s).
func (g GameweekLedger) Pot() Money {
	var pot Money
	for _, w := range g.Weeks {
		pot += w.Top
	}
	return pot
}

// LedgerRow is a manager's running totals.
type LedgerRow struct {
	Manager Manager
	Halves  [2]Money
}

// Season is the manager's total across both halves.
func (r LedgerRow) Season() Money { return r.Halves[0] + r.Halves[1] }

// Ledger is the league's money, rebuilt from match results.
type Ledger struct {
	Gameweeks []GameweekLedger // settled gameweeks in ascending order
	Rows      []LedgerRow      // sorted by season total, best first
}

// Gameweek returns the ledger for an event, if it is settled.
func (l Ledger) Gameweek(event int) (GameweekLedger, bool) {
	for _, g := range l.Gameweeks {
		if g.Event == event {
			return g, true
		}
	}
	return GameweekLedger{}, false
}

// BuildLedger computes the money for every settled gameweek.
func BuildLedger(data LeagueData) Ledger {
	totals := map[int]*[2]Money{}
	for _, m := range data.Managers {
		totals[m.Entry] = &[2]Money{}
	}
	total := func(entry int) *[2]Money {
		t, ok := totals[entry]
		if !ok {
			t = &[2]Money{}
			totals[entry] = t
		}
		return t
	}

	var ledger Ledger
	for _, e := range data.Events {
		if !e.Settled() {
			continue
		}
		matches := data.MatchesFor(e.ID)
		if len(matches) == 0 {
			continue
		}
		g := settleGameweek(e.ID, matches, func(entry int) Money { return total(entry)[Half(e.ID)] })
		for entry, w := range g.Weeks {
			total(entry)[Half(e.ID)] += w.Total()
		}
		ledger.Gameweeks = append(ledger.Gameweeks, g)
	}

	byEntry := map[int]Manager{}
	for _, m := range data.Managers {
		byEntry[m.Entry] = m
	}
	for entry, t := range totals {
		m, ok := byEntry[entry]
		if !ok {
			m = Manager{Entry: entry}
		}
		ledger.Rows = append(ledger.Rows, LedgerRow{Manager: m, Halves: *t})
	}
	sort.Slice(ledger.Rows, func(i, j int) bool {
		a, b := ledger.Rows[i], ledger.Rows[j]
		if a.Season() != b.Season() {
			return a.Season() > b.Season()
		}
		if a.Manager.DisplayName != b.Manager.DisplayName {
			return a.Manager.DisplayName < b.Manager.DisplayName
		}
		return a.Manager.Entry < b.Manager.Entry
	})
	return ledger
}

// settleGameweek applies the money rules to one gameweek. halfTotal gives a
// manager's running total for the current half before this gameweek.
func settleGameweek(event int, matches []Match, halfTotal func(entry int) Money) GameweekLedger {
	g := GameweekLedger{Event: event, Weeks: map[int]ManagerWeek{}}
	for _, m := range matches {
		for _, pair := range [2][2]Side{{m.Home, m.Away}, {m.Away, m.Home}} {
			me, them := pair[0], pair[1]
			if !me.IsAverage() {
				g.Weeks[me.Entry] = ManagerWeek{Entry: me.Entry, Points: me.Points, Opponent: them.Entry}
			}
		}
	}

	// H2H, with the loser's payment limited by the cap.
	for _, m := range matches {
		if m.Home.IsAverage() || m.Away.IsAverage() || m.Home.Points == m.Away.Points {
			continue
		}
		winner, loser := m.Home.Entry, m.Away.Entry
		if m.Away.Points > m.Home.Points {
			winner, loser = loser, winner
		}
		stake := min(h2hStake, max(0, halfTotal(loser)-halfCap))
		w, l := g.Weeks[winner], g.Weeks[loser]
		w.H2H += stake
		l.H2H -= stake
		l.Capped = stake < h2hStake
		g.Weeks[winner], g.Weeks[loser] = w, l
	}

	if len(g.Weeks) < 2 {
		return g
	}

	var scores []int
	for _, w := range g.Weeks {
		scores = append(scores, w.Points)
	}
	sort.Ints(scores)
	lowest, secondLowest, highest := scores[0], scores[1], scores[len(scores)-1]

	var pot Money
	for entry, w := range g.Weeks {
		if w.Points == lowest || w.Points == secondLowest {
			w.Bottom = -bottomStake
			pot += bottomStake
			g.Weeks[entry] = w
			g.BottomTwo = append(g.BottomTwo, entry)
		}
		if w.Points == highest {
			g.TopScorers = append(g.TopScorers, entry)
		}
	}
	sort.Ints(g.BottomTwo)
	sort.Ints(g.TopScorers)

	// Split the pot; leftover cents go to the lowest entry ids so the
	// ledger still sums to zero.
	share, rem := pot/Money(len(g.TopScorers)), pot%Money(len(g.TopScorers))
	for i, entry := range g.TopScorers {
		w := g.Weeks[entry]
		w.Top = share
		if Money(i) < rem {
			w.Top++
		}
		g.Weeks[entry] = w
	}
	return g
}
