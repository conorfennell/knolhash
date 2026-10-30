package fpl

import (
	"sort"
	"testing"
)

// week builds a settled gameweek's matches from (home, homePts, away, awayPts)
// tuples; entry 0 is AVERAGE.
func week(event int, fixtures ...[4]int) []Match {
	var out []Match
	for _, f := range fixtures {
		out = append(out, Match{Event: event, Home: Side{Entry: f[0], Points: f[1]}, Away: Side{Entry: f[2], Points: f[3]}})
	}
	return out
}

func league(managers []int, matches ...[]Match) LeagueData {
	d := LeagueData{}
	for _, id := range managers {
		d.Managers = append(d.Managers, Manager{Entry: id, DisplayName: string(rune('A' + id - 1))})
	}
	seen := map[int]bool{}
	for _, ms := range matches {
		for _, m := range ms {
			if !seen[m.Event] {
				seen[m.Event] = true
				d.Events = append(d.Events, Event{ID: m.Event, Finished: true, DataChecked: true})
			}
		}
		d.Matches = append(d.Matches, ms...)
	}
	return d
}

func totals(l Ledger) map[int][2]Money {
	out := map[int][2]Money{}
	for _, r := range l.Rows {
		out[r.Manager.Entry] = r.Halves
	}
	return out
}

// assertZeroSum checks no money is made or lost: each gameweek's payments
// balance against the AVERAGE pot.
func assertZeroSum(t *testing.T, l Ledger) {
	t.Helper()
	for _, g := range l.Gameweeks {
		var sum Money
		for _, w := range g.Weeks {
			sum += w.Total()
		}
		if sum+g.PotAfter-g.PotBefore != 0 {
			t.Errorf("GW%d: managers %v, pot %v → %v; want balanced", g.Event, sum, g.PotBefore, g.PotAfter)
		}
	}
}

func TestSettleGameweek(t *testing.T) {
	zero := func(int) Money { return 0 }
	at := func(entry int, total Money) func(int) Money {
		return func(e int) Money {
			if e == entry {
				return total
			}
			return 0
		}
	}
	cases := []struct {
		name       string
		matches    []Match
		halfTotal  func(int) Money
		potIn      Money
		want       map[int]Money // net per entry
		wantPot    Money         // AVERAGE pot after the gameweek
		wantTop    []int
		wantBottom []int
		wantCapped []int
	}{
		{
			name:       "plain week",
			matches:    week(1, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}),
			want:       map[int]Money{1: 500 + 1000, 2: -500 - 500, 3: 500, 4: -500 - 500},
			wantTop:    []int{1},
			wantBottom: []int{2, 4},
		},
		{
			name:       "H2H draw moves nothing",
			matches:    week(1, [4]int{1, 50, 2, 50}, [4]int{3, 70, 4, 30}, [4]int{5, 40, 6, 60}),
			want:       map[int]Money{1: 0, 2: 0, 3: 500 + 1000, 4: -500 - 500, 5: -500 - 500, 6: 500},
			wantTop:    []int{3},
			wantBottom: []int{4, 5},
		},
		{
			name:       "two-way top tie splits the pot",
			matches:    week(1, [4]int{1, 90, 2, 40}, [4]int{3, 90, 4, 30}),
			want:       map[int]Money{1: 500 + 500, 2: -500 - 500, 3: 500 + 500, 4: -500 - 500},
			wantTop:    []int{1, 3},
			wantBottom: []int{2, 4},
		},
		{
			name:       "three joint last share €10",
			matches:    week(1, [4]int{1, 90, 2, 30}, [4]int{3, 30, 4, 30}, [4]int{5, 60, 6, 70}),
			want:       map[int]Money{1: 500 + 1000, 2: -500 - 334, 3: -333, 4: -333, 5: -500, 6: 500},
			wantTop:    []int{1},
			wantBottom: []int{2, 3, 4},
		},
		{
			name:       "one last, two joint second-last pay €5, €2.50, €2.50",
			matches:    week(1, [4]int{1, 20, 2, 40}, [4]int{3, 40, 4, 90}),
			want:       map[int]Money{1: -500 - 500, 2: 500 - 250, 3: -500 - 250, 4: 500 + 1000},
			wantTop:    []int{4},
			wantBottom: []int{1, 2, 3},
		},
		{
			name:       "three-way top tie splits €10 with spare cent",
			matches:    week(1, [4]int{1, 80, 2, 10}, [4]int{3, 80, 4, 20}, [4]int{5, 80, 6, 50}),
			want:       map[int]Money{1: 500 + 334, 2: -500 - 500, 3: 500 + 333, 4: -500 - 500, 5: 500 + 333, 6: -500},
			wantTop:    []int{1, 3, 5},
			wantBottom: []int{2, 4},
		},
		{
			name:       "everyone tied: all share the €10 both ways, net zero",
			matches:    week(1, [4]int{1, 50, 2, 50}, [4]int{3, 50, 4, 50}),
			want:       map[int]Money{1: 0, 2: 0, 3: 0, 4: 0},
			wantTop:    []int{1, 2, 3, 4},
			wantBottom: []int{1, 2, 3, 4},
		},
		{
			name:       "losing to AVERAGE pays €5 into the pot; AVERAGE ignored for top and bottom",
			matches:    week(1, [4]int{0, 99, 1, 50}, [4]int{2, 10, 3, 70}, [4]int{4, 60, 5, 40}),
			want:       map[int]Money{1: -500, 2: -500 - 500, 3: 500 + 1000, 4: 500, 5: -500 - 500},
			wantPot:    500,
			wantTop:    []int{3},
			wantBottom: []int{2, 5},
		},
		{
			name:       "beating AVERAGE wins the whole pot",
			matches:    week(4, [4]int{1, 70, 0, 40}, [4]int{2, 50, 3, 30}),
			potIn:      1500,
			want:       map[int]Money{1: 1500 + 1000, 2: 500 - 500, 3: -500 - 500},
			wantPot:    0,
			wantTop:    []int{1},
			wantBottom: []int{2, 3},
		},
		{
			name:       "drawing with AVERAGE leaves the pot alone",
			matches:    week(4, [4]int{1, 40, 0, 40}, [4]int{2, 50, 3, 30}),
			potIn:      1000,
			want:       map[int]Money{1: -500, 2: 500 + 1000, 3: -500 - 500},
			wantPot:    1000,
			wantTop:    []int{2},
			wantBottom: []int{1, 3},
		},
		{
			name:       "loser near the cap pays only what's left, then nothing for bottom two",
			matches:    week(5, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}),
			halfTotal:  at(2, -4800),
			want:       map[int]Money{1: 200 + 500, 2: -200, 3: 500, 4: -500 - 500},
			wantTop:    []int{1},
			wantBottom: []int{2, 4},
			wantCapped: []int{2},
		},
		{
			name:       "loser at the cap pays nothing",
			matches:    week(5, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}),
			halfTotal:  at(2, -5000),
			want:       map[int]Money{1: 500, 2: 0, 3: 500, 4: -500 - 500},
			wantTop:    []int{1},
			wantBottom: []int{2, 4},
			wantCapped: []int{2},
		},
		{
			// Kevin's example: at −€50, second-lowest beats the lowest, wins €5
			// in the H2H and pays it straight back for being second-lowest.
			name:       "cap applies after the H2H win",
			matches:    week(5, [4]int{2, 40, 1, 20}, [4]int{3, 60, 4, 70}),
			halfTotal:  at(2, -5000),
			want:       map[int]Money{1: -500 - 500, 2: 500 - 500, 3: -500, 4: 500 + 1000},
			wantTop:    []int{4},
			wantBottom: []int{1, 2},
		},
		{
			name:       "losing to AVERAGE at the cap pays nothing into the pot",
			matches:    week(5, [4]int{0, 60, 1, 40}, [4]int{2, 70, 3, 50}),
			halfTotal:  at(1, -5000),
			potIn:      500,
			want:       map[int]Money{1: 0, 2: 500 + 500, 3: -500 - 500},
			wantPot:    500,
			wantTop:    []int{2},
			wantBottom: []int{1, 3},
			wantCapped: []int{1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ht := c.halfTotal
			if ht == nil {
				ht = zero
			}
			g := settleGameweek(c.matches[0].Event, c.matches, ht, c.potIn)
			for entry, want := range c.want {
				if got := g.Weeks[entry].Total(); got != want {
					t.Errorf("entry %d: got %v, want %v (%+v)", entry, got, want, g.Weeks[entry])
				}
			}
			var sum Money
			for _, w := range g.Weeks {
				sum += w.Total()
			}
			if sum+g.PotAfter-g.PotBefore != 0 {
				t.Errorf("gameweek money doesn't balance: managers %v, pot %v → %v", sum, g.PotBefore, g.PotAfter)
			}
			if g.PotAfter != c.wantPot {
				t.Errorf("pot after = %v, want %v", g.PotAfter, c.wantPot)
			}
			if !equalInts(g.TopScorers, c.wantTop) {
				t.Errorf("top scorers = %v, want %v", g.TopScorers, c.wantTop)
			}
			if !equalInts(g.BottomTwo, c.wantBottom) {
				t.Errorf("bottom two = %v, want %v", g.BottomTwo, c.wantBottom)
			}
			var capped []int
			for entry, w := range g.Weeks {
				if w.Capped {
					capped = append(capped, entry)
				}
			}
			sort.Ints(capped)
			if !equalInts(capped, c.wantCapped) {
				t.Errorf("capped = %v, want %v", capped, c.wantCapped)
			}
		})
	}
}

func TestBuildLedgerCapAndHalves(t *testing.T) {
	// Entry 2 loses every week with the lowest score: −€10 a week, landing
	// exactly on −€50 after GW19. The half resets at GW20, so the losses
	// there are uncapped.
	var matches [][]Match
	for gw := 15; gw <= 21; gw++ {
		matches = append(matches, week(gw, [4]int{1, 80, 2, 10}, [4]int{3, 60, 4, 50}))
	}
	l := BuildLedger(league([]int{1, 2, 3, 4}, matches...))
	assertZeroSum(t, l)

	got := totals(l)[2]
	// GW15–19: −10 each → −€50 at the cap.
	if got[0] != -5000 {
		t.Errorf("entry 2 first half = %v, want −€50", got[0])
	}
	// GW20–21 in the new half: −€10 each.
	if got[1] != -2000 {
		t.Errorf("entry 2 second half = %v, want −€20", got[1])
	}

	g, _ := l.Gameweek(19)
	// Entry 2 was at −€40 before GW19, so the H2H loss of €5 still fits and
	// the bottom-two €5 takes them to exactly −€50.
	if w := g.Weeks[2]; w.Capped || w.H2H != -500 {
		t.Errorf("GW19 entry 2 = %+v, want uncapped −€5 H2H", w)
	}
}

func TestBuildLedgerCapBitesWithinHalf(t *testing.T) {
	// Six losing weeks with the lowest score in the first half: by the
	// sixth week entry 2 is at −€50 and pays nothing more.
	var matches [][]Match
	for gw := 1; gw <= 6; gw++ {
		matches = append(matches, week(gw, [4]int{1, 80, 2, 10}, [4]int{3, 60, 4, 50}))
	}
	l := BuildLedger(league([]int{1, 2, 3, 4}, matches...))
	assertZeroSum(t, l)
	g, _ := l.Gameweek(6)
	if w := g.Weeks[2]; !w.Capped || w.H2H != 0 || w.Bottom != 0 {
		t.Errorf("GW6 entry 2 = %+v, want capped: no H2H or bottom-two payment", w)
	}
	if got := totals(l)[2][0]; got != -5000 {
		t.Errorf("entry 2 first half = %v, want −€50 (the cap holds)", got)
	}
}

func TestBuildLedgerSkipsUnsettledGameweeks(t *testing.T) {
	d := league([]int{1, 2, 3, 4},
		week(1, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}),
		week(2, [4]int{1, 80, 2, 40}, [4]int{3, 60, 4, 50}),
	)
	d.Events[1].DataChecked = false // GW2 finished, bonus not confirmed
	l := BuildLedger(d)
	if len(l.Gameweeks) != 1 || l.Gameweeks[0].Event != 1 {
		t.Fatalf("settled gameweeks = %+v, want only GW1", l.Gameweeks)
	}
	if l.Rows[0].Manager.Entry != 1 || l.Rows[0].Season() != 1500 {
		t.Errorf("leader = %+v, want entry 1 on €15", l.Rows[0])
	}
}

func TestBuildLedgerRecordedLeague(t *testing.T) {
	srv := fakeFPL(t)
	data, err := (&Client{BaseURL: srv.URL, HTTP: srv.Client()}).FetchLeague(LeagueID)
	if err != nil {
		t.Fatal(err)
	}
	l := BuildLedger(data)
	assertZeroSum(t, l)
	// testdata has GW1 and GW8 (settled) and GW9 (upcoming).
	if len(l.Gameweeks) != 2 {
		t.Fatalf("settled gameweeks = %d, want 2", len(l.Gameweeks))
	}
	if len(l.Rows) != 11 {
		t.Errorf("ledger rows = %d, want 11 managers", len(l.Rows))
	}
	// GW8, checked by hand: Luke top on 53 (H2H win + €10 pot), Kevin (11)
	// and Paul (32) bottom two and both lost, Oisin lost to AVERAGE.
	g, _ := l.Gameweek(8)
	for entry, want := range map[int]Money{5593872: 1500, 2196128: -1000, 2202445: -1000, 4899183: -500, 2206892: 500} {
		if got := g.Weeks[entry].Total(); got != want {
			t.Errorf("GW8 entry %d = %v, want %v", entry, got, want)
		}
	}
}

func TestMoneyString(t *testing.T) {
	for m, want := range map[Money]string{0: "€0", 1000: "€10", -500: "−€5", 1250: "€12.50", -833: "−€8.33"} {
		if got := m.String(); got != want {
			t.Errorf("Money(%d) = %q, want %q", m, got, want)
		}
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
