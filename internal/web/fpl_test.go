package web

import (
	"strings"
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
	ev := buildFPLEvent(data, fpl.BuildLedger(data), live, nil, data.Events[0])

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
	got := buildFPLAvailability(data, live, nil)
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
	ev := buildFPLEvent(data, fpl.BuildLedger(data), live, nil, data.Events[0])
	if ev.Live || ev.Fixtures[0].Home.Points != 50 {
		t.Errorf("settled gameweek used live scores: %+v", ev)
	}
}

func TestBuildFPLIcons(t *testing.T) {
	settled := func(id int) fpl.Event { return fpl.Event{ID: id, Finished: true, DataChecked: true} }
	side := func(entry, pts int) fpl.Side { return fpl.Side{Entry: entry, Points: pts} }
	// Entry 1 wins three in a row; entry 4 loses everything.
	data := fpl.LeagueData{
		Managers: []fpl.Manager{{Entry: 1, DisplayName: "A"}, {Entry: 2, DisplayName: "B"}, {Entry: 3, DisplayName: "C"}, {Entry: 4, DisplayName: "D"}},
		Events:   []fpl.Event{settled(1), settled(2), settled(3)},
		Standings: []fpl.Standing{
			{Rank: 1, Side: side(1, 0), Total: 9, PointsFor: 240},
			{Rank: 2, Side: fpl.Side{Name: "AVERAGE"}, Total: 6},
			{Rank: 3, Side: side(2, 0), Total: 3},
		},
		Players: map[int]fpl.Player{10: {Status: "i"}},
		Matches: []fpl.Match{
			{Event: 1, Home: side(1, 80), Away: side(4, 20)}, {Event: 1, Home: side(2, 50), Away: side(3, 40)},
			{Event: 2, Home: side(1, 70), Away: side(3, 60)}, {Event: 2, Home: side(2, 50), Away: side(4, 45)},
			{Event: 3, Home: side(1, 90), Away: side(2, 30)}, {Event: 3, Home: side(3, 60), Away: side(4, 10)},
		},
	}
	live := fpl.LiveData{Picks: map[int]fpl.Picks{4: {Players: []fpl.Pick{{Element: 10, Multiplier: 1}}}}}
	icons := buildFPLIcons(data, fpl.BuildLedger(data), live)

	for entry, want := range map[int][]string{
		1: {fplIconLeague, fplIconMoney, fplIconStar, fplIconStreak, fplIconNaming},
		4: {fplIconSpoon, fplIconInjured, fplIconSlump},
	} {
		for _, icon := range want {
			if !strings.Contains(icons[entry], icon) {
				t.Errorf("entry %d icons %q missing %s", entry, icons[entry], icon)
			}
		}
	}
	// Entry 2 went W, W, L and entry 3 L, L, W: neither has a three-game run.
	for _, e := range []int{2, 3} {
		if strings.Contains(icons[e], fplIconStreak) || strings.Contains(icons[e], fplIconSlump) {
			t.Errorf("entry %d icons %q, want no streak", e, icons[e])
		}
	}
	if strings.Contains(icons[0], fplIconLeague) {
		t.Errorf("unexpected icons: %v", icons)
	}
}
