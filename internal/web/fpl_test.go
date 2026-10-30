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
		Players: map[int]fpl.Player{10: {Name: "Saka", Status: "i"}},
		Matches: []fpl.Match{
			{Event: 1, Home: side(1, 80), Away: side(4, 20)}, {Event: 1, Home: side(2, 50), Away: side(3, 40)},
			{Event: 2, Home: side(1, 70), Away: side(3, 60)}, {Event: 2, Home: side(2, 50), Away: side(4, 45)},
			{Event: 3, Home: side(1, 90), Away: side(2, 30)}, {Event: 3, Home: side(3, 60), Away: side(4, 10)},
		},
	}
	live := fpl.LiveData{Picks: map[int]fpl.Picks{4: {Players: []fpl.Pick{{Element: 10, Multiplier: 1}}}}}
	icons := buildFPLIcons(data, fpl.BuildLedger(data), live)

	has := func(entry int, icon string) bool {
		for _, i := range icons[entry] {
			if i.Icon == icon {
				return true
			}
		}
		return false
	}
	for entry, want := range map[int][]string{
		1: {fplIconLeague, fplIconMoney, fplIconStar, fplIconStreak, fplIconNaming},
		4: {fplIconSpoon, fplIconInjured, fplIconSlump},
	} {
		for _, icon := range want {
			if !has(entry, icon) {
				t.Errorf("entry %d icons %+v missing %s", entry, icons[entry], icon)
			}
		}
	}
	// Icons explain themselves, and bad news is flagged as such.
	for _, i := range icons[4] {
		if len(i.Detail) < 4 || i.Detail[:3] != "D: " {
			t.Errorf("entry 4 icon %+v", i)
		}
		if i.Icon == fplIconInjured && i.Detail != "D: Saka" {
			t.Errorf("injured detail = %q, want the ruled-out player named", i.Detail)
		}
		if (i.Icon == fplIconSpoon || i.Icon == fplIconSlump || i.Icon == fplIconInjured) && !i.Bad {
			t.Errorf("%s should be marked bad", i.Icon)
		}
	}
	for _, i := range icons[1] {
		if i.Bad {
			t.Errorf("entry 1 icon %s marked bad", i.Icon)
		}
		if i.Icon == fplIconStreak && i.Detail != "A: 3 H2H wins in a row" {
			t.Errorf("streak detail = %q", i.Detail)
		}
	}
	// Entry 2 went W, W, L and entry 3 L, L, W: neither has a three-game run.
	for _, e := range []int{2, 3} {
		if has(e, fplIconStreak) || has(e, fplIconSlump) {
			t.Errorf("entry %d icons %+v, want no streak", e, icons[e])
		}
	}
	if has(0, fplIconLeague) {
		t.Errorf("unexpected icons: %v", icons)
	}
}

func TestBuildFPLAwardTally(t *testing.T) {
	data := fpl.LeagueData{
		Managers: []fpl.Manager{{Entry: 1, DisplayName: "Kevin"}, {Entry: 2, DisplayName: "Paul"}, {Entry: 3, DisplayName: "Ian"}},
		Events:   []fpl.Event{{ID: 1}, {ID: 2}},
	}
	awards := map[int][]fpl.Award{
		1: {{Icon: "🍀", Title: "Jammy Win", Winners: []fpl.AwardWinner{{Entry: 2}}}, {Icon: "🪑", Title: "Bench of Shame", Shame: true, Winners: []fpl.AwardWinner{{Entry: 1}}}},
		2: {{Icon: "🪑", Title: "Bench of Shame", Shame: true, Winners: []fpl.AwardWinner{{Entry: 1}}}},
	}
	got := buildFPLAwardTally(data, awards, nil)
	if len(got) != 2 || got[0].Name != "Paul" || got[1].Name != "Kevin" {
		t.Fatalf("tally = %+v, want Paul (fame) then Kevin (shame); Ian has none", got)
	}
	if k := got[1]; k.Shame != 2 || k.Counts[3] != 2 || k.Counts[1] != 0 {
		t.Errorf("Kevin = %+v, want Bench of Shame ×2 in column 4", k)
	}
	if p := got[0]; p.Fame != 1 || p.Counts[1] != 1 {
		t.Errorf("Paul = %+v, want one Jammy Win", p)
	}
}

func TestFPLApplyAwardsBadgesFixtureRows(t *testing.T) {
	data := fpl.LeagueData{Managers: []fpl.Manager{{Entry: 1, DisplayName: "Kevin"}, {Entry: 2, DisplayName: "Paul"}}}
	ev := &fplEventView{Fixtures: []fplFixtureView{
		{Home: fplSideView{Entry: 1, Name: "Kevin"}, Away: fplSideView{Entry: 2, Name: "Paul"}},
		{Home: fplSideView{Name: "AVERAGE", IsAverage: true}, Away: fplSideView{Entry: 3}},
	}}
	fplApplyAwards(ev, data, []fpl.Award{
		{Icon: "🫠", Title: "Captain Calamity", Shame: true, Winners: []fpl.AwardWinner{{Entry: 1, Detail: "Groß (C) 0 pts"}}},
		{Icon: "🍀", Title: "Jammy Win", Winners: []fpl.AwardWinner{{Entry: 1, Detail: "won with 35"}, {Entry: 0}}},
	})
	k := ev.Fixtures[0].Home.Awards
	if len(k) != 2 || k[0].Icon != "🫠" || k[0].Title != "Captain Calamity" || k[0].Detail != "Kevin: Groß (C) 0 pts" || !k[0].Shame || k[1].Icon != "🍀" {
		t.Errorf("Kevin badges = %+v", k)
	}
	if len(ev.Fixtures[0].Away.Awards) != 0 || len(ev.Fixtures[1].Home.Awards) != 0 {
		t.Errorf("unexpected badges: %+v", ev.Fixtures)
	}
}

func TestBuildFPLRivalry(t *testing.T) {
	r := buildFPLRivalry(fpl.Rivalry{BWins: 1, Draws: 1, Meetings: []fpl.Meeting{{Event: 1, APoints: 40, BPoints: 55}, {Event: 5, APoints: 50, BPoints: 50}}}, "Kevin", "Paul")
	want := "Season: Kevin 0 · Paul 1 · 1 drawn\nGW1: Kevin 40 – 55 Paul\nGW5: Kevin 50 – 50 Paul\nBiggest win: Paul by 15 (GW1)"
	if r.Summary != "0–1" || r.Detail != want || r.Title != "⚔️ Kevin v Paul" {
		t.Errorf("rivalry = %+v\nwant detail:\n%s", r, want)
	}
}
