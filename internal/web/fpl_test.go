package web

import (
	"strings"
	"testing"
	"time"

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
		if i.Icon == fplIconStreak && (i.Detail != "A: 3 H2H wins in a row" || i.Count != 3 || i.Kind != "form-w") {
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
	data.Standings = []fpl.Standing{{Rank: 1, Side: fpl.Side{Entry: 2}}, {Rank: 2, Side: fpl.Side{Entry: 3}}, {Rank: 3, Side: fpl.Side{Entry: 1}}}
	got := buildFPLAwardTally(data, awards)
	if len(got) != 2 || got[0].Name != "Paul" || got[1].Name != "Kevin" {
		t.Fatalf("tally = %+v, want league order: Paul (1st) then Kevin (3rd); Ian has no awards", got)
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

func TestFPLApplyMatchups(t *testing.T) {
	kick := time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC)
	two := 2
	data := fpl.LeagueData{
		Players: map[int]fpl.Player{
			1: {Name: "Haaland", TeamID: 10, Team: "MCI", Position: 4, Status: "a"},
			2: {Name: "Salah", TeamID: 20, Team: "LIV", Position: 3, Status: "a"},
			3: {Name: "Isak", TeamID: 20, Team: "LIV", Position: 4, Status: "a"},
			4: {Name: "Saka", TeamID: 30, Team: "ARS", Position: 3, Status: "i"},
			5: {Name: "Wissa", TeamID: 40, Team: "NEW", Position: 4, Status: "a"},
			6: {Name: "Raya", TeamID: 30, Team: "ARS", Position: 1, Status: "a"},
		},
		Clubs: map[int]string{10: "MCI", 20: "LIV", 30: "ARS", 40: "NEW", 50: "CHE"},
		Fixtures: []fpl.Fixture{
			{Event: 8, Home: 10, Away: 20, Started: true, Finished: true, HomeScore: &two, AwayScore: &two},
			{Event: 8, Home: 30, Away: 50, Started: true, Minutes: 63},
			{Event: 9, Home: 50, Away: 10, Kickoff: kick, HomeDifficulty: 3, AwayDifficulty: 4},
			{Event: 9, Home: 20, Away: 30, Kickoff: kick, HomeDifficulty: 5, AwayDifficulty: 2},
		},
	}
	// A: captain Haaland ×2, Isak, Raya (bench). B: captain Salah ×3, Haaland, Saka.
	picksA := fpl.Picks{Captain: 1, TransferCost: 4, Players: []fpl.Pick{{Element: 1, Multiplier: 2}, {Element: 3, Multiplier: 1}, {Element: 6, Multiplier: 0}}}
	picksB := fpl.Picks{Captain: 2, Chip: "3xc", Players: []fpl.Pick{{Element: 2, Multiplier: 3}, {Element: 1, Multiplier: 1}, {Element: 4, Multiplier: 1}}}
	fixtures := func(id int, status string) *fplEventView {
		return &fplEventView{ID: id, Status: status, Fixtures: []fplFixtureView{
			{Home: fplSideView{Entry: 1, Name: "Luke"}, Away: fplSideView{Entry: 2, Name: "James P"}},
			{Home: fplSideView{Name: "AVERAGE", IsAverage: true}, Away: fplSideView{Entry: 3, Name: "Oisin"}},
		}}
	}

	// Settled GW8: stats explain the points; Saka's Arsenal game is still on.
	ev := fixtures(8, "settled")
	history := map[int]fpl.GameweekHistory{8: {
		Picks: map[int]fpl.Picks{1: picksA, 2: picksB},
		Stats: map[int]fpl.PlayerStats{
			1: {Points: 10, Minutes: 90, Goals: 2, Bonus: 3},
			2: {Points: 6, Minutes: 90, Assists: 1, YellowCards: 1},
			3: {Points: 7, Minutes: 90, Goals: 1},
			4: {Points: 2, Minutes: 60},
			5: {Points: 1, Minutes: 30},
			6: {Points: 5, Minutes: 90, CleanSheets: 1, Saves: 6},
		},
	}}
	live := fpl.LiveData{Transfers: map[int][]fpl.Transfer{1: {{Event: 8, In: 3, Out: 5}, {Event: 3, In: 9, Out: 9}}}}
	fplApplyMatchups(ev, data, live, history)
	m := ev.Fixtures[0].Matchup
	if m == nil || m.Mode != "final" || m.Note != "" || m.SameCaptain != nil {
		t.Fatalf("settled matchup = %+v", m)
	}
	if c := m.Home.Captain; c == nil || c.Name != "Haaland" || c.Points != 20 || c.Status != "played" || c.When != "v LIV (H)" || c.Bonus != 3 ||
		len(c.Events) != 1 || c.Events[0].Icon != "⚽" || c.Events[0].Count != 2 {
		t.Errorf("home captain = %+v", c)
	}
	if c := m.Away.Captain; c == nil || c.Points != 18 || c.Multiplier != 3 || m.Away.Chip != "TC" {
		t.Errorf("away captain = %+v", c)
	}
	// Captains are listed apart, so only Isak and Saka are differential rows.
	if len(m.Rows) != 1 || m.Rows[0].Home.Name != "Isak" || m.Rows[0].Away.Name != "Saka" || m.Rows[0].Away.Status != "live" || m.Rows[0].Away.When != "v CHE (H) 63'" {
		t.Errorf("rows = %+v / %+v", m.Rows[0].Home, m.Rows[0].Away)
	}
	if m.Home.OnlyPts != 27 || m.Away.OnlyPts != 20 || m.Away.Playing != 1 {
		t.Errorf("differential points = %d v %d, away playing %d", m.Home.OnlyPts, m.Away.OnlyPts, m.Away.Playing)
	}
	if len(m.Home.Transfers) != 1 || m.Home.Transfers[0].In.Name != "Isak" || m.Home.Transfers[0].Out.Name != "Wissa" || m.Home.Transfers[0].Verdict != "win" || m.Home.Hit != 4 {
		t.Errorf("transfers = %+v hit %d", m.Home.Transfers, m.Home.Hit)
	}
	if m.Home.BenchPts != 5 || len(m.Home.Bench) != 1 || len(m.Home.Bench[0].Events) != 2 {
		t.Errorf("home bench = %d %+v, want Raya with clean sheet and saves", m.Home.BenchPts, m.Home.Bench)
	}
	if len(m.Shared) != 0 {
		t.Errorf("shared = %+v, want Haaland left out as a captain", m.Shared)
	}
	if ev.Fixtures[1].Matchup != nil {
		t.Error("AVERAGE fixture got a matchup")
	}

	// Upcoming GW9 preview: last week's teams, next fixtures with difficulty,
	// injury flags, no points, transfers or hits.
	ev = fixtures(9, "upcoming")
	live = fpl.LiveData{PicksEvent: 8, Picks: map[int]fpl.Picks{1: picksA, 2: picksB}, Transfers: live.Transfers}
	fplApplyMatchups(ev, data, live, nil)
	m = ev.Fixtures[0].Matchup
	if m == nil || m.Mode != "preview" || m.Note == "" || m.Home.Hit != 0 || len(m.Home.Transfers) != 0 {
		t.Fatalf("preview matchup = %+v", m)
	}
	if c := m.Home.Captain; c.Status != "upcoming" || len(c.Next) != 1 || c.Next[0] != (fplNextFixture{Kick: "Sat 15:00", Vs: "v CHE (A)", FDR: 4}) {
		t.Errorf("preview captain = %+v", c)
	}
	// Preview rows are grouped by position: FWD (Isak) and MID (Saka).
	if len(m.Rows) != 4 || m.Rows[0].Header != "MID" || m.Rows[1].Away == nil || m.Rows[1].Home != nil || m.Rows[2].Header != "FWD" || m.Rows[3].Home.Name != "Isak" {
		t.Errorf("preview rows = %+v", m.Rows)
	}
	if saka := m.Rows[1].Away; saka.Flag != "Injured" || !saka.FlagOut || saka.Next[0].FDR != 2 || len(saka.Events) != 0 {
		t.Errorf("preview Saka = %+v", saka)
	}
	if m.Home.ToPlay != 2 {
		t.Errorf("home to play = %d, want captain and Isak", m.Home.ToPlay)
	}

	// Same captain, same multiplier: one line, and not repeated as shared.
	same := fpl.Picks{Captain: 1, Players: []fpl.Pick{{Element: 1, Multiplier: 2}, {Element: 3, Multiplier: 1}}}
	ev = fixtures(8, "settled")
	history[8].Picks[1], history[8].Picks[2] = same, same
	fplApplyMatchups(ev, data, fpl.LiveData{}, history)
	if m := ev.Fixtures[0].Matchup; m.SameCaptain == nil || m.SameCaptain.Name != "Haaland" || m.Home.Captain != nil || len(m.Rows) != 0 || len(m.Shared) != 1 {
		t.Errorf("same-captain matchup = %+v", m)
	}

	// Nothing known: no panel.
	ev = fixtures(9, "settled")
	fplApplyMatchups(ev, data, fpl.LiveData{}, nil)
	if ev.Fixtures[0].Matchup != nil {
		t.Error("matchup built without any teams")
	}
}

func TestFPLKits(t *testing.T) {
	sprite := string(fplKitSprite())
	for club := range fplKits {
		if !strings.Contains(sprite, `id="kit-`+club+`"`) {
			t.Errorf("sprite missing %s", club)
		}
	}
	if !strings.Contains(sprite, `id="kit-NEW-stripes"`) || strings.Contains(sprite, `id="kit-ARS-stripes"`) {
		t.Error("stripe patterns wrong: Newcastle should have one, Arsenal not")
	}
	if got := string(fplKitIcon("ARS")); !strings.Contains(got, `href="#kit-ARS"`) {
		t.Errorf("ARS icon = %s", got)
	}
	if got := string(fplKitIcon("XYZ")); got != `<span class="mc-club">XYZ</span>` {
		t.Errorf("unknown club = %s, want the code as text", got)
	}
}
