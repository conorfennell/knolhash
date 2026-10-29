package fpl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func awardsByTitle(as []Award) map[string]Award {
	out := map[string]Award{}
	for _, a := range as {
		out[a.Title] = a
	}
	return out
}

func winnerEntries(a Award) []int {
	var out []int
	for _, w := range a.Winners {
		out = append(out, w.Entry)
	}
	return out
}

func TestGameweekAwards(t *testing.T) {
	players := map[int]Player{10: {Name: "Haaland"}, 11: {Name: "Salah"}, 12: {Name: "Palmer"}}
	matches := week(5,
		[4]int{1, 40, 2, 39}, // 1 jammy win on 40; 2 lost by 1 after a −4 hit
		[4]int{3, 75, 4, 72}, // 4 robbed on 72
		[4]int{5, 90, 0, 50}, // 5 v AVERAGE: AVERAGE never wins anything
	)
	h := GameweekHistory{
		Event:  5,
		Points: map[int]int{10: 2, 11: 12, 12: 15},
		Picks: map[int]Picks{
			1: {Captain: 10, Vice: 11, Players: []Pick{{10, 2}, {11, 1}}, PointsOnBench: 3},
			2: {Captain: 12, Vice: 10, TransferCost: 4, Players: []Pick{{12, 2}, {10, 1}}, PointsOnBench: 21},
			3: {Chip: "3xc", Captain: 12, Players: []Pick{{12, 3}}},
			4: {Chip: "bboost", Captain: 11, Players: []Pick{{11, 2}}, PointsOnBench: 30}, // bench counted under BB
			5: {Captain: 11, Players: []Pick{{11, 2}}},
		},
	}
	got := awardsByTitle(GameweekAwards(matches, h, players))

	check := func(title string, want []int, detail string) {
		t.Helper()
		a, ok := got[title]
		if !ok {
			t.Errorf("no %q award", title)
			return
		}
		if !equalInts(winnerEntries(a), want) {
			t.Errorf("%s winners = %v, want %v", title, winnerEntries(a), want)
		}
		if detail != "" && a.Winners[0].Detail != detail {
			t.Errorf("%s detail = %q, want %q", title, a.Winners[0].Detail, detail)
		}
	}
	check("Jammy Win", []int{1}, "won with 40")
	check("Robbed", []int{4}, "lost with 72")
	check("Bench of Shame", []int{2}, "21 pts on the bench")
	check("Captain Calamity", []int{1}, "Haaland (C) 2 pts, vice Salah got 12")
	check("Hit That Didn't Pay", []int{2}, "took a −4 hit, lost by 1")
	check("Chip of the Week", []int{3}, "TC, 75 pts")
	check("Chip Flop", []int{4}, "BB, 72 pts")

	for _, a := range got {
		for _, w := range a.Winners {
			if w.Entry == 0 {
				t.Errorf("AVERAGE won %s", a.Title)
			}
		}
	}
}

func TestGameweekAwardsQuietWeek(t *testing.T) {
	// No chips, no hits, nothing on the bench: only the always-on awards.
	matches := week(1, [4]int{1, 60, 2, 50})
	h := GameweekHistory{Points: map[int]int{10: 6}, Picks: map[int]Picks{
		1: {Captain: 10, Players: []Pick{{10, 2}}},
		2: {Captain: 10, Players: []Pick{{10, 2}}},
	}}
	got := awardsByTitle(GameweekAwards(matches, h, nil))
	for _, title := range []string{"Chip of the Week", "Chip Flop", "Bench of Shame", "Hit That Didn't Pay"} {
		if _, ok := got[title]; ok {
			t.Errorf("unexpected %q award in a quiet week", title)
		}
	}
	// Both captained the same 6-pointer: a shared calamity.
	if !equalInts(winnerEntries(got["Captain Calamity"]), []int{1, 2}) {
		t.Errorf("captain calamity = %+v, want shared", got["Captain Calamity"])
	}
	if got["Captain Calamity"].Winners[0].Detail != "? (C) 6 pts" {
		t.Errorf("unknown player detail = %q", got["Captain Calamity"].Winners[0].Detail)
	}
}

func TestGameweekAwardsChipFlopNeedsADifference(t *testing.T) {
	matches := week(1, [4]int{1, 60, 2, 60})
	h := GameweekHistory{Picks: map[int]Picks{1: {Chip: "wildcard"}, 2: {Chip: "freehit"}}}
	got := awardsByTitle(GameweekAwards(matches, h, nil))
	if _, ok := got["Chip Flop"]; ok {
		t.Error("chip flop given when both chips scored the same")
	}
	if !equalInts(winnerEntries(got["Chip of the Week"]), []int{1, 2}) {
		t.Errorf("chip of the week = %+v, want both", got["Chip of the Week"])
	}
}

func TestEffectiveCaptainFollowsArmband(t *testing.T) {
	// Captain 10 didn't play, FPL moved the ×2 to vice 11.
	p := Picks{Captain: 10, Vice: 11, Players: []Pick{{10, 1}, {11, 2}}}
	if got := effectiveCaptain(p); got != 11 {
		t.Errorf("effective captain = %d, want vice 11", got)
	}
}

func TestHistoryCacheFetchesSettledGameweeksOnce(t *testing.T) {
	calls := map[string]int{}
	mux := http.NewServeMux()
	serve := func(path, file string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			calls[path]++
			b, _ := os.ReadFile("testdata/" + file)
			w.Write(b)
		})
	}
	serve("/event/8/live/", "live_event8.json")
	serve("/entry/5134711/event/8/picks/", "picks_5134711_event8.json")
	serve("/entry/2196128/event/8/picks/", "picks_2196128_event8.json")
	srv := httptest.NewServer(mux)
	defer srv.Close()

	league := &Cache{lastAttempt: time.Now(), data: LeagueData{
		Managers: []Manager{{Entry: 5134711}, {Entry: 2196128}},
		Events:   []Event{{ID: 8, Finished: true, DataChecked: true}, {ID: 9, IsNext: true}},
		Matches:  []Match{{Event: 8, Home: Side{Entry: 5134711, Points: 39}, Away: Side{Entry: 2196128, Points: 11}}},
	}}
	c := NewHistoryCache(&Client{BaseURL: srv.URL, HTTP: srv.Client()}, league)
	c.poll()
	c.poll()

	h, ok := c.Get()[8]
	if !ok || len(h.Picks) != 2 || h.Picks[2196128].Captain == 0 || h.Picks[2196128].PointsOnBench != 1 {
		t.Fatalf("GW8 history = %+v", h)
	}
	if _, ok := c.Get()[9]; ok {
		t.Error("unsettled GW9 fetched")
	}
	if calls["/event/8/live/"] != 1 {
		t.Errorf("GW8 live fetched %d times, want once", calls["/event/8/live/"])
	}
}
