package web

import (
	"testing"

	"github.com/conorfennell/knolhash/internal/fpl"
)

func TestBuildFPLEventLive(t *testing.T) {
	data := fpl.LeagueData{
		Managers: []fpl.Manager{{Entry: 1, DisplayName: "Kevin"}, {Entry: 2, DisplayName: "Paul"}, {Entry: 3, DisplayName: "Ian"}},
		Events:   []fpl.Event{{ID: 9, Name: "Gameweek 9", IsCurrent: true}},
		Matches: []fpl.Match{
			{Event: 9, Home: fpl.Side{Entry: 1, Name: "Kevin", Points: 0}, Away: fpl.Side{Entry: 2, Name: "Paul", Points: 0}},
			{Event: 9, Home: fpl.Side{Name: "AVERAGE", Points: 41}, Away: fpl.Side{Entry: 3, Name: "Ian", Points: 0}},
		},
	}
	live := fpl.LiveData{
		Event:  9,
		Scores: map[int]int{1: 62, 2: 48}, // Ian's picks failed to load
		Chips:  map[int]map[int]string{1: {9: "3xc"}, 2: {4: "bboost"}},
	}
	ev := buildFPLEvent(data, fpl.BuildLedger(data), live, data.Events[0])

	if !ev.Live || ev.Status != "live" {
		t.Fatalf("event = %+v, want live", ev)
	}
	kp := ev.Fixtures[0]
	if kp.Home.Points != 62 || kp.Away.Points != 48 || kp.Home.Outcome != "win" || kp.Away.Outcome != "loss" {
		t.Errorf("Kevin v Paul = %+v", kp)
	}
	if kp.Home.Chip != "TC" || kp.Away.Chip != "" {
		t.Errorf("chips = %q / %q, want TC only for this gameweek", kp.Home.Chip, kp.Away.Chip)
	}
	if kp.Home.Money != "" {
		t.Errorf("live gameweek shows money %q before it is settled", kp.Home.Money)
	}
	avg := ev.Fixtures[1]
	if avg.Home.Points != 41 || avg.Away.Points != 0 {
		t.Errorf("AVERAGE v Ian = %+v, want FPL-reported points kept", avg)
	}
}

func TestBuildFPLAvailability(t *testing.T) {
	pct := func(n int) *int { return &n }
	data := fpl.LeagueData{
		Managers: []fpl.Manager{{Entry: 1, DisplayName: "Kevin"}, {Entry: 2, DisplayName: "Paul"}, {Entry: 3, DisplayName: "Ian"}},
		Players: map[int]fpl.Player{
			10: {Name: "Saka", Team: "ARS", Status: "i", News: "Thigh"},
			11: {Name: "Rice", Team: "ARS", Status: "d", Chance: pct(75)},
			12: {Name: "Fit", Status: "a"},
		},
	}
	live := fpl.LiveData{PicksEvent: 8, Picks: map[int]fpl.Picks{
		1: {Players: []fpl.Pick{{Element: 11, Multiplier: 1}, {Element: 10, Multiplier: 2}}},
		2: {Players: []fpl.Pick{{Element: 12, Multiplier: 2}}}, // all fit → omitted
		// 3: picks failed to load
	}}
	got := buildFPLAvailability(data, live)
	if len(got) != 2 {
		t.Fatalf("availability = %+v, want Kevin and Ian", got)
	}
	k := got[0]
	if k.Name != "Kevin" || len(k.Concerns) != 2 || k.Concerns[0].Player != "Saka" || k.Concerns[0].Severity != "out" || k.Concerns[1].Severity != "doubt" || k.Concerns[1].Label != "75%" {
		t.Errorf("Kevin = %+v", k)
	}
	if got[1].Name != "Ian" || !got[1].Missing {
		t.Errorf("Ian = %+v, want flagged as missing", got[1])
	}
}

func TestBuildFPLEventNotLiveIgnoresLiveScores(t *testing.T) {
	data := fpl.LeagueData{
		Events:  []fpl.Event{{ID: 8, IsCurrent: true, Finished: true, DataChecked: true}},
		Matches: []fpl.Match{{Event: 8, Home: fpl.Side{Entry: 1, Points: 50}, Away: fpl.Side{Entry: 2, Points: 40}}},
	}
	live := fpl.LiveData{Event: 0, Scores: map[int]int{1: 99}}
	ev := buildFPLEvent(data, fpl.BuildLedger(data), live, data.Events[0])
	if ev.Live || ev.Fixtures[0].Home.Points != 50 {
		t.Errorf("settled gameweek used live scores: %+v", ev)
	}
}
