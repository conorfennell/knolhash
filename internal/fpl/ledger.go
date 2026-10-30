package fpl

import (
	"fmt"
	"sort"
)

// Money rules, from Kevin's 2025/26 sheet, confirmed by Kevin on
// 30 Oct 2026:
//
//   - H2H: loser pays winner €5; a draw moves nothing.
//   - AVERAGE: losing to AVERAGE puts €5 into a rolling AVERAGE pot; the
//     next manager to beat AVERAGE wins the whole pot. A draw moves nothing.
//   - Bottom two: the lowest and second-lowest places of the gameweek pay
//     €5 each, so the pot is €10. Managers tied across those places share
//     them: one last and two joint second-last pay €5, €2.50 and €2.50;
//     three joint last pay €3.33 each.
//   - Top scorer: the highest scorer(s) split the bottom-two payments.
//   - Cap: each half (GW1–19, GW20–38) no manager goes below −€50. It
//     applies as the gameweek is worked out: H2H (and AVERAGE) first, then
//     the bottom two, each loss limited to what's left above −€50. Winners
//     and the top scorer receive what was actually paid.
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
	H2H      Money // + won, − lost against a real opponent
	Average  Money // − paid into the AVERAGE pot, + won it
	Bottom   Money // − share of the bottom-two places
	Top      Money // + share of the bottom-two pot
	Capped   bool  // a loss was reduced by the −€50 cap
	Opponent int   // entry id, 0 for AVERAGE
}

// Total is the manager's net money for the gameweek.
func (w ManagerWeek) Total() Money { return w.H2H + w.Average + w.Bottom + w.Top }

// GameweekLedger is the money for one settled gameweek.
type GameweekLedger struct {
	Event      int
	Weeks      map[int]ManagerWeek // by entry id
	TopScorers []int               // entry ids, sorted
	BottomTwo  []int               // entry ids, sorted
	PotBefore  Money               // AVERAGE pot going into the gameweek
	PotAfter   Money               // AVERAGE pot after it
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
	Pot       Money            // AVERAGE pot still waiting for a winner
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

// Ranks gives each manager's money-table position. Equal season totals
// share a rank (1, 1, 3, …).
func (l Ledger) Ranks() map[int]int {
	ranks := make(map[int]int, len(l.Rows))
	for i, r := range l.Rows {
		ranks[r.Manager.Entry] = i + 1
		if i > 0 && r.Season() == l.Rows[i-1].Season() {
			ranks[r.Manager.Entry] = ranks[l.Rows[i-1].Manager.Entry]
		}
	}
	return ranks
}

// MoneyMoves compares each manager's money-table position with the table
// before the latest settled gameweek: positive means they climbed. It is
// empty until two gameweeks have settled.
func MoneyMoves(data LeagueData) map[int]int {
	now := BuildLedger(data)
	if len(now.Gameweeks) < 2 {
		return map[int]int{}
	}
	last := now.Gameweeks[len(now.Gameweeks)-1].Event
	before := data
	before.Events = make([]Event, len(data.Events))
	copy(before.Events, data.Events)
	for i := range before.Events {
		if before.Events[i].ID == last {
			before.Events[i].DataChecked = false
		}
	}
	prev, cur := BuildLedger(before).Ranks(), now.Ranks()
	moves := make(map[int]int, len(cur))
	for entry, r := range cur {
		if p, ok := prev[entry]; ok {
			moves[entry] = p - r
		}
	}
	return moves
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
		g := settleGameweek(e.ID, matches, func(entry int) Money { return total(entry)[Half(e.ID)] }, ledger.Pot)
		ledger.Pot = g.PotAfter
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
// manager's running total for the current half before this gameweek; pot is
// the AVERAGE pot carried in.
func settleGameweek(event int, matches []Match, halfTotal func(entry int) Money, pot Money) GameweekLedger {
	g := GameweekLedger{Event: event, Weeks: map[int]ManagerWeek{}, PotBefore: pot, PotAfter: pot}
	for _, m := range matches {
		for _, pair := range [2][2]Side{{m.Home, m.Away}, {m.Away, m.Home}} {
			me, them := pair[0], pair[1]
			if !me.IsAverage() {
				g.Weeks[me.Entry] = ManagerWeek{Entry: me.Entry, Points: me.Points, Opponent: them.Entry}
			}
		}
	}

	// pay takes up to amount from a manager, limited by the cap given what
	// they've already won or lost this gameweek, and returns what was paid.
	pay := func(entry int, amount Money) Money {
		w := g.Weeks[entry]
		paid := min(amount, max(0, halfTotal(entry)+w.Total()-halfCap))
		if paid < amount {
			w.Capped = true
			g.Weeks[entry] = w
		}
		return paid
	}

	// H2H and AVERAGE first.
	for _, m := range matches {
		if m.Home.Points == m.Away.Points {
			continue
		}
		winner, loser := m.Home, m.Away
		if m.Away.Points > m.Home.Points {
			winner, loser = loser, winner
		}
		switch {
		case loser.IsAverage():
			w := g.Weeks[winner.Entry]
			w.Average += g.PotAfter
			g.Weeks[winner.Entry] = w
			g.PotAfter = 0
		case winner.IsAverage():
			paid := pay(loser.Entry, h2hStake)
			l := g.Weeks[loser.Entry]
			l.Average -= paid
			g.Weeks[loser.Entry] = l
			g.PotAfter += paid
		default:
			paid := pay(loser.Entry, h2hStake)
			w, l := g.Weeks[winner.Entry], g.Weeks[loser.Entry]
			w.H2H += paid
			l.H2H -= paid
			g.Weeks[winner.Entry], g.Weeks[loser.Entry] = w, l
		}
	}

	if len(g.Weeks) < 2 {
		return g
	}

	// Bottom two: the lowest two places carry €5 each; tied managers share
	// the places their tie covers.
	entries := make([]int, 0, len(g.Weeks))
	for entry := range g.Weeks {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := g.Weeks[entries[i]], g.Weeks[entries[j]]
		if a.Points != b.Points {
			return a.Points < b.Points
		}
		return a.Entry < b.Entry
	})
	var bottomPot Money
	for start := 0; start < 2 && start < len(entries); {
		end := start
		for end < len(entries) && g.Weeks[entries[end]].Points == g.Weeks[entries[start]].Points {
			end++
		}
		places := min(end, 2) - start
		group := entries[start:end]
		for i, share := range splitMoney(Money(places)*bottomStake, len(group)) {
			entry := group[i]
			paid := pay(entry, share)
			w := g.Weeks[entry]
			w.Bottom -= paid
			g.Weeks[entry] = w
			bottomPot += paid
			g.BottomTwo = append(g.BottomTwo, entry)
		}
		start = end
	}
	sort.Ints(g.BottomTwo)

	highest := g.Weeks[entries[len(entries)-1]].Points
	for _, entry := range entries {
		if g.Weeks[entry].Points == highest {
			g.TopScorers = append(g.TopScorers, entry)
		}
	}
	sort.Ints(g.TopScorers)
	for i, share := range splitMoney(bottomPot, len(g.TopScorers)) {
		w := g.Weeks[g.TopScorers[i]]
		w.Top += share
		g.Weeks[g.TopScorers[i]] = w
	}
	return g
}

// splitMoney divides an amount into n shares that add up exactly; spare
// cents go to the first shares.
func splitMoney(amount Money, n int) []Money {
	shares := make([]Money, n)
	for i := range shares {
		shares[i] = amount / Money(n)
		if Money(i) < amount%Money(n) {
			shares[i]++
		}
	}
	return shares
}
