package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/fpl"
)

type fplSideView struct {
	Entry     int
	Name      string
	Icons     []fplStatusIcon
	TeamName  string
	Points    int
	Outcome   string // "win", "loss", "draw" or "" when not yet played
	IsAverage bool
	Money     string          // net € for the gameweek, "" until the gameweek is settled
	MoneySign int             // -1, 0, 1 for styling
	Top       bool            // gameweek top scorer
	Bottom    bool            // in the gameweek's bottom two
	Capped    bool            // a loss reduced by the −€50 cap
	Pot       string          // "paid" into or "won" the AVERAGE pot, "" otherwise
	Chip      string          // chip badge ("TC", "BB", …), "" when none
	Awards    []fplAwardBadge // this gameweek's awards
}

// fplAwardBadge is an award on a manager's fixture row; tapping it shows
// the title and detail.
type fplAwardBadge struct {
	Icon   string
	Title  string
	Detail string // "Kevin: Groß (C) 0 pts"
	Shame  bool
}

type fplFixtureView struct {
	Home    fplSideView
	Away    fplSideView
	Rivalry *fplRivalryView // nil against AVERAGE
}

// fplRivalryView is the season's head-to-head record shown under a
// fixture's score; tapping it shows the meetings.
type fplRivalryView struct {
	Summary string // "0–1" from the home side, "1st" before they've met
	Title   string // "⚔️ Kevin v Paul"
	Detail  string // one line per fact
}

type fplEventView struct {
	ID       int
	Name     string
	Deadline time.Time
	Status   string // see fpl.Event.Status
	Live     bool   // points are provisional live scores
	Fixtures []fplFixtureView
	Summary  string // "Top: … · Bottom two: …" once settled
}

// fplAwardColumns is the order of the season tally's columns: fame, then
// shame.
var fplAwardColumns = []fplAwardColumn{
	{"🚀", "Chip of the Week", false},
	{"🍀", "Jammy Win", false},
	{"💔", "Robbed", true},
	{"🪑", "Bench of Shame", true},
	{"🫠", "Captain Calamity", true},
	{"💸", "Hit That Didn't Pay", true},
	{"🧯", "Chip Flop", true},
}

type fplAwardColumn struct {
	Icon  string
	Title string
	Shame bool
}

// fplAwardTally is a manager's awards across the season; Counts lines up
// with fplAwardColumns.
type fplAwardTally struct {
	Name   string
	Icons  []fplStatusIcon
	Counts []int
	Fame   int
	Shame  int
}

type fplMoneyRow struct {
	Rank     int
	Move     int // places climbed since the previous gameweek
	Name     string
	Icons    []fplStatusIcon
	TeamName string
	Halves   [2]string
	Season   string
	Sign     int
}

type fplConcernView struct {
	Player   string
	Club     string
	Label    string
	News     string
	Bench    bool
	Severity string // "out" or "doubt"
}

type fplAvailabilityView struct {
	Name     string
	Icons    []fplStatusIcon
	TeamName string
	Concerns []fplConcernView
	Missing  bool // team couldn't be fetched
}

type fplNamingView struct {
	Event  int
	Winner string
	Loser  string
	Score  string
	Margin int
}

type fplTemplateData struct {
	LeagueName      string
	Standings       []fpl.Standing
	Upcoming        *fplEventView
	Latest          *fplEventView
	Money           []fplMoneyRow
	CurrentHalf     int
	AveragePot      string          // AVERAGE pot waiting for a winner, "" when empty
	Past            []*fplEventView // settled gameweeks, newest first
	NamingRights    *fplNamingView
	Availability    []fplAvailabilityView
	TeamsEvent      int                     // gameweek the availability teams are from
	Icons           map[int][]fplStatusIcon // entry → status icons, as on the World Cup page
	AwardTally      []fplAwardTally
	AwardColumns    []fplAwardColumn
	FetchedAt       time.Time
	NextRefreshUnix int64
	Commit          string
	BuildDate       string
}

func (s *Server) handleGetFPL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := s.fplCache.Get()
		live := s.fplLive.Get()
		ledger := fpl.BuildLedger(data)
		icons := buildFPLIcons(data, ledger, live)
		awards := buildFPLAwards(data, s.fplHistory.Get())

		td := fplTemplateData{
			LeagueName:      data.LeagueName,
			Standings:       data.Standings,
			Money:           buildFPLMoney(ledger, fpl.MoneyMoves(data), icons),
			AveragePot:      fplPot(ledger.Pot),
			Icons:           icons,
			AwardTally:      buildFPLAwardTally(data, awards, icons),
			AwardColumns:    fplAwardColumns,
			FetchedAt:       data.FetchedAt,
			NextRefreshUnix: s.fplCache.NextRefresh().Unix(),
			Commit:          shortCommit(s.commit),
			BuildDate:       shortBuildDate(s.buildDate),
		}
		// While a gameweek is in progress it is the only one worth showing at
		// the top. Between gameweeks, show next week's fixtures and last
		// week's results.
		if cur, ok := data.CurrentEvent(); ok {
			td.Latest = buildFPLEvent(data, ledger, live, icons, cur)
			fplApplyAwards(td.Latest, data, awards[cur.ID])
			td.CurrentHalf = fpl.Half(cur.ID)
		}
		if next, ok := data.NextEvent(); ok && (td.Latest == nil || td.Latest.Status != "live") {
			td.Upcoming = buildFPLEvent(data, ledger, live, icons, next)
		}
		if nr, ok := fpl.BiggestMargin(data); ok {
			td.NamingRights = &fplNamingView{
				Event: nr.Event, Winner: nr.Winner.Name, Loser: nr.Loser.Name, Margin: nr.Margin,
				Score: fmt.Sprintf("%d – %d", nr.Winner.Points, nr.Loser.Points),
			}
		}
		td.Availability, td.TeamsEvent = buildFPLAvailability(data, live, icons), live.PicksEvent
		for i := len(data.Events) - 1; i >= 0; i-- {
			if e := data.Events[i]; e.Settled() && (td.Latest == nil || e.ID != td.Latest.ID) {
				ev := buildFPLEvent(data, ledger, live, icons, e)
				fplApplyAwards(ev, data, awards[e.ID])
				td.Past = append(td.Past, ev)
			}
		}

		if err := s.templates.ExecuteTemplate(w, "fpl", td); err != nil {
			slog.Error("fpl: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// buildFPLAwards works out the weekly awards for every settled gameweek
// whose teams have been fetched.
func buildFPLAwards(data fpl.LeagueData, history map[int]fpl.GameweekHistory) map[int][]fpl.Award {
	out := map[int][]fpl.Award{}
	for _, e := range data.Events {
		if h, ok := history[e.ID]; ok && e.Settled() {
			out[e.ID] = fpl.GameweekAwards(data.MatchesFor(e.ID), h, data.Players)
		}
	}
	return out
}

// fplApplyAwards puts a gameweek's awards as badges on each winner's
// fixture row.
func fplApplyAwards(ev *fplEventView, data fpl.LeagueData, awards []fpl.Award) {
	names := fplNameMap(data)
	badges := map[int][]fplAwardBadge{}
	for _, a := range awards {
		for _, w := range a.Winners {
			badges[w.Entry] = append(badges[w.Entry], fplAwardBadge{Icon: a.Icon, Title: a.Title, Detail: names[w.Entry] + ": " + w.Detail, Shame: a.Shame})
		}
	}
	for i := range ev.Fixtures {
		f := &ev.Fixtures[i]
		if !f.Home.IsAverage {
			f.Home.Awards = badges[f.Home.Entry]
		}
		if !f.Away.IsAverage {
			f.Away.Awards = badges[f.Away.Entry]
		}
	}
}

// buildFPLAwardTally counts each manager's awards over the season, most
// decorated (fame minus shame) first. Managers with no awards are left out.
func buildFPLAwardTally(data fpl.LeagueData, awards map[int][]fpl.Award, icons map[int][]fplStatusIcon) []fplAwardTally {
	col := map[string]int{}
	for i, c := range fplAwardColumns {
		col[c.Title] = i
	}
	counts := map[int][]int{}
	for _, byEvent := range awards {
		for _, a := range byEvent {
			i, ok := col[a.Title]
			if !ok {
				continue
			}
			for _, w := range a.Winners {
				if counts[w.Entry] == nil {
					counts[w.Entry] = make([]int, len(fplAwardColumns))
				}
				counts[w.Entry][i]++
			}
		}
	}
	var out []fplAwardTally
	for _, m := range data.Managers {
		c, ok := counts[m.Entry]
		if !ok {
			continue
		}
		t := fplAwardTally{Name: m.DisplayName, Icons: icons[m.Entry], Counts: c}
		for i, n := range c {
			if fplAwardColumns[i].Shame {
				t.Shame += n
			} else {
				t.Fame += n
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := out[i].Fame-out[i].Shame, out[j].Fame-out[j].Shame; a != b {
			return a > b
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// FPL status icons shown beside managers' names, in display order.
const (
	fplIconLeague  = "👑"  // top of the H2H league table
	fplIconMoney   = "💰"  // top of the money table
	fplIconSpoon   = "🥄"  // bottom of the money table
	fplIconStar    = "⭐"  // top scorer in the last settled gameweek
	fplIconStreak  = "🔥"  // three or more H2H wins in a row
	fplIconSlump   = "🧊"  // three or more H2H losses in a row
	fplIconNaming  = "✍️" // holds the naming rights
	fplIconInjured = "🚑"  // a ruled-out player in the starting XI
)

// fplStatusIcon is a status icon beside a manager's name; tapping it shows
// what it means.
type fplStatusIcon struct {
	Icon   string
	Title  string
	Detail string // "Eoghan: top of the H2H table on 21 pts"
	Bad    bool
}

// buildFPLIcons gives each manager their current status icons. Like the
// World Cup page, icons reflect standings now, not a particular gameweek.
func buildFPLIcons(data fpl.LeagueData, l fpl.Ledger, live fpl.LiveData) map[int][]fplStatusIcon {
	names := fplNameMap(data)
	icons := map[int][]fplStatusIcon{}
	add := func(entry int, icon, title, detail string, bad bool) {
		icons[entry] = append(icons[entry], fplStatusIcon{Icon: icon, Title: title, Detail: names[entry] + ": " + detail, Bad: bad})
	}

	if len(data.Standings) > 0 {
		var top fpl.Standing
		for _, s := range data.Standings {
			if !s.Side.IsAverage() {
				top = s
				break
			}
		}
		for _, s := range data.Standings {
			if !s.Side.IsAverage() && s.Total == top.Total && s.PointsFor == top.PointsFor {
				add(s.Side.Entry, fplIconLeague, "League leader", fmt.Sprintf("top of the H2H table on %d pts", s.Total), false)
			}
		}
	}
	if len(l.Gameweeks) > 0 && len(l.Rows) > 1 {
		best, worst := l.Rows[0].Season(), l.Rows[len(l.Rows)-1].Season()
		if best != worst {
			for _, r := range l.Rows {
				switch r.Season() {
				case best:
					add(r.Manager.Entry, fplIconMoney, "Most money", r.Season().String()+" this season", false)
				case worst:
					add(r.Manager.Entry, fplIconSpoon, "Wooden spoon", r.Season().String()+" this season, bottom of the money table", true)
				}
			}
		}
		g := l.Gameweeks[len(l.Gameweeks)-1]
		for _, e := range g.TopScorers {
			add(e, fplIconStar, "Top scorer", fmt.Sprintf("highest score in Gameweek %d with %d", g.Event, g.Weeks[e].Points), false)
		}
	}
	streaks := fplStreaks(data)
	for _, m := range data.Managers {
		switch n := streaks[m.Entry]; {
		case n >= 3:
			add(m.Entry, fplIconStreak, "On fire", fmt.Sprintf("%d H2H wins in a row", n), false)
		case n <= -3:
			add(m.Entry, fplIconSlump, "Ice cold", fmt.Sprintf("%d H2H losses in a row", -n), true)
		}
	}
	if nr, ok := fpl.BiggestMargin(data); ok {
		add(nr.Winner.Entry, fplIconNaming, "Naming rights",
			fmt.Sprintf("names %s's team next year (won by %d in Gameweek %d)", nr.Loser.Name, nr.Margin, nr.Event), false)
	}
	for _, m := range data.Managers {
		picks, ok := live.Picks[m.Entry]
		if !ok {
			continue
		}
		var out []string
		for _, c := range fpl.Concerns(picks, data.Players) {
			if c.Player.Out() && !c.Bench {
				out = append(out, c.Player.Name)
			}
		}
		if len(out) > 0 {
			add(m.Entry, fplIconInjured, "Starter ruled out", strings.Join(out, ", "), true)
		}
	}
	return icons
}

// fplStreaks counts each manager's current run of consecutive H2H results,
// most recent settled gameweek backwards: positive for wins, negative for
// losses. A draw ends a run.
func fplStreaks(data fpl.LeagueData) map[int]int {
	streaks := map[int]int{}
	ended := map[int]bool{}
	for i := len(data.Events) - 1; i >= 0; i-- {
		e := data.Events[i]
		if !e.Settled() {
			continue
		}
		for _, m := range data.MatchesFor(e.ID) {
			for _, pair := range [2][2]fpl.Side{{m.Home, m.Away}, {m.Away, m.Home}} {
				me, them := pair[0], pair[1]
				if me.IsAverage() || ended[me.Entry] {
					continue
				}
				step := 0
				switch {
				case me.Points > them.Points:
					step = 1
				case me.Points < them.Points:
					step = -1
				}
				if step == 0 || streaks[me.Entry]*step < 0 {
					ended[me.Entry] = true
					continue
				}
				streaks[me.Entry] += step
			}
		}
	}
	return streaks
}

func buildFPLAvailability(data fpl.LeagueData, live fpl.LiveData, icons map[int][]fplStatusIcon) []fplAvailabilityView {
	var out []fplAvailabilityView
	for _, m := range data.Managers {
		v := fplAvailabilityView{Name: m.DisplayName, Icons: icons[m.Entry], TeamName: m.TeamName}
		picks, ok := live.Picks[m.Entry]
		if !ok {
			if live.PicksEvent != 0 {
				v.Missing = true
				out = append(out, v)
			}
			continue
		}
		for _, c := range fpl.Concerns(picks, data.Players) {
			sev := "doubt"
			if c.Player.Out() {
				sev = "out"
			}
			v.Concerns = append(v.Concerns, fplConcernView{
				Player: c.Player.Name, Club: c.Player.Team, Label: c.Player.StatusLabel(),
				News: c.Player.News, Bench: c.Bench, Severity: sev,
			})
		}
		if len(v.Concerns) > 0 {
			out = append(out, v)
		}
	}
	return out
}

func buildFPLMoney(l fpl.Ledger, moves map[int]int, icons map[int][]fplStatusIcon) []fplMoneyRow {
	var rows []fplMoneyRow
	ranks := l.Ranks()
	for _, r := range l.Rows {
		rows = append(rows, fplMoneyRow{
			Rank:     ranks[r.Manager.Entry],
			Move:     moves[r.Manager.Entry],
			Name:     r.Manager.DisplayName,
			Icons:    icons[r.Manager.Entry],
			TeamName: r.Manager.TeamName,
			Halves:   [2]string{r.Halves[0].String(), r.Halves[1].String()},
			Season:   r.Season().String(),
			Sign:     fplSign(r.Season()),
		})
	}
	return rows
}

func buildFPLEvent(data fpl.LeagueData, l fpl.Ledger, live fpl.LiveData, icons map[int][]fplStatusIcon, e fpl.Event) *fplEventView {
	ev := &fplEventView{ID: e.ID, Name: e.Name, Deadline: e.Deadline, Status: e.Status()}
	ev.Live = ev.Status == "live" && live.Event == e.ID
	g, settled := l.Gameweek(e.ID)
	for _, m := range data.MatchesFor(e.ID) {
		home, away := fplSide(m.Home, g, settled), fplSide(m.Away, g, settled)
		home.Icons, away.Icons = icons[m.Home.Entry], icons[m.Away.Entry]
		if ev.Live {
			home.Points, away.Points = fplLivePoints(m.Home, live), fplLivePoints(m.Away, live)
		}
		home.Chip = fpl.ChipLabel(live.Chip(m.Home.Entry, e.ID))
		away.Chip = fpl.ChipLabel(live.Chip(m.Away.Entry, e.ID))
		if ev.Status != "upcoming" {
			home.Outcome, away.Outcome = fplOutcome(home.Points, away.Points), fplOutcome(away.Points, home.Points)
		}
		f := fplFixtureView{Home: home, Away: away}
		if !m.Home.IsAverage() && !m.Away.IsAverage() {
			f.Rivalry = buildFPLRivalry(fpl.HeadToHead(data, m.Home.Entry, m.Away.Entry), m.Home.Name, m.Away.Name)
		}
		ev.Fixtures = append(ev.Fixtures, f)
	}
	if settled {
		ev.Summary = "Top: " + fplNames(data, g.TopScorers) + " · Bottom two: " + fplNames(data, g.BottomTwo)
	}
	return ev
}

func buildFPLRivalry(r fpl.Rivalry, a, b string) *fplRivalryView {
	v := &fplRivalryView{Title: "⚔️ " + a + " v " + b, Summary: fmt.Sprintf("%d–%d", r.AWins, r.BWins)}
	if len(r.Meetings) == 0 {
		v.Summary = "1st"
		v.Detail = "First meeting this season."
		return v
	}
	lines := []string{fmt.Sprintf("Season: %s %d · %s %d", a, r.AWins, b, r.BWins)}
	if r.Draws > 0 {
		lines[0] += fmt.Sprintf(" · %d drawn", r.Draws)
	}
	best, bestBy, bestGW := "", 0, 0
	for _, m := range r.Meetings {
		lines = append(lines, fmt.Sprintf("GW%d: %s %d – %d %s", m.Event, a, m.APoints, m.BPoints, b))
		if d := m.APoints - m.BPoints; d > bestBy || -d > bestBy {
			bestGW = m.Event
			if d > 0 {
				best, bestBy = a, d
			} else {
				best, bestBy = b, -d
			}
		}
	}
	if bestBy > 0 && len(r.Meetings) > 1 {
		lines = append(lines, fmt.Sprintf("Biggest win: %s by %d (GW%d)", best, bestBy, bestGW))
	}
	v.Detail = strings.Join(lines, "\n")
	return v
}

func fplSide(s fpl.Side, g fpl.GameweekLedger, settled bool) fplSideView {
	v := fplSideView{Entry: s.Entry, Name: s.Name, TeamName: s.TeamName, Points: s.Points, IsAverage: s.IsAverage()}
	if w, ok := g.Weeks[s.Entry]; settled && ok {
		v.Money, v.MoneySign = w.Total().String(), fplSign(w.Total())
		v.Top, v.Bottom, v.Capped = w.Top > 0, w.Bottom < 0, w.Capped
		switch {
		case w.Average > 0:
			v.Pot = "won"
		case w.Average < 0:
			v.Pot = "paid"
		}
	}
	return v
}

func fplLivePoints(s fpl.Side, live fpl.LiveData) int {
	if p, ok := live.ScoreFor(s); ok {
		return p
	}
	return s.Points
}

func fplNameMap(data fpl.LeagueData) map[int]string {
	names := make(map[int]string, len(data.Managers))
	for _, m := range data.Managers {
		names[m.Entry] = m.DisplayName
	}
	return names
}

func fplNames(data fpl.LeagueData, entries []int) string {
	names := fplNameMap(data)
	out := ""
	for i, e := range entries {
		if i > 0 {
			out += ", "
		}
		out += names[e]
	}
	return out
}

func fplOutcome(mine, theirs int) string {
	switch {
	case mine > theirs:
		return "win"
	case mine < theirs:
		return "loss"
	}
	return "draw"
}

func fplPot(m fpl.Money) string {
	if m == 0 {
		return ""
	}
	return m.String()
}

func fplSign(m fpl.Money) int {
	switch {
	case m > 0:
		return 1
	case m < 0:
		return -1
	}
	return 0
}
