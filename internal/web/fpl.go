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
	Matchup *fplMatchupView // nil when neither team is known
}

// fplMatchupView compares the two teams in a fixture; it opens under the
// fixture when tapped. Only the differences between the teams decide the
// match, so the panel is built around them: captains (or one line if they
// cancel out), differentials paired off like a scoreboard, transfers, then
// the shared players and benches in small print.
type fplMatchupView struct {
	Note        string // set for a pre-deadline preview
	Mode        string // "preview", "live" or "final"
	Home, Away  fplMatchupSide
	SameCaptain *fplMatchPlayer // both captained this player with the same multiplier
	Rows        []fplDuelRow    // differentials, paired off
	Shared      []fplMatchPlayer
}

type fplMatchupSide struct {
	Name      string
	Chip      string
	Captain   *fplMatchPlayer
	OnlyPts   int // points from captain (unless both had the same) and differentials
	ToPlay    int // players who still have a game to come
	Playing   int // players on the pitch now
	Transfers []fplTransferView
	Hit       int
	BenchPts  int
	Bench     []fplMatchPlayer
}

// fplDuelRow is a row of the matchup: a pair of differentials, or (in a
// preview) a position heading.
type fplDuelRow struct {
	Header     string
	Home, Away *fplMatchPlayer
}

// fplMatchPlayer is a player in the matchup panel.
type fplMatchPlayer struct {
	Name       string
	Club       string
	Pos        string // "gk", "def", "mid" or "fwd"
	Points     int    // points counted for the manager (× multiplier)
	Multiplier int
	Status     string           // "played", "live", "upcoming", "dnp" or "blank"
	PosLabel   string           // "GK", "DEF", "MID", "FWD"
	When       string           // "v CHE (H)", "Sat 15:00 v CHE (H)", "Didn't play", …
	Next       []fplNextFixture // previews: kick-off and opponent, tinted by difficulty
	Events     []fplStatEvent
	Bonus      int
	Flag       string // "Injured", "75%", … for previews
	FlagOut    bool
}

// fplNextFixture is an upcoming game in a preview; FDR (1 easy – 5 hard)
// tints the opponent.
type fplNextFixture struct {
	Kick string
	Vs   string
	FDR  int
}

// fplStatEvent is something a player did: ⚽ ×2, 🟨, …
type fplStatEvent struct {
	Icon  string
	Count int
	Title string
}

type fplTransferView struct {
	In, Out fplMatchPlayer
	Verdict string // "win" if the new player outscored the old one, "loss" if not
}

type fplEventView struct {
	ID       int
	Name     string
	Deadline time.Time
	Status   string // see fpl.Event.Status
	Live     bool   // points are provisional live scores
	Fixtures []fplFixtureView
}

// fplAwardColumns is the order of the season tally's columns: fame, then
// shame.
var fplAwardColumns = []fplAwardColumn{
	{"🚀", "Chip of the Week", "Best score by anyone playing a chip", false},
	{"🍀", "Jammy Win", "Won their H2H with the week's lowest winning score", false},
	{"💔", "Robbed", "Lost their H2H with the week's highest losing score", true},
	{"🪑", "Bench of Shame", "Most points left on the bench", true},
	{"🫠", "Captain Calamity", "Lowest-scoring captain of the week", true},
	{"💸", "Hit That Didn't Pay", "Took a transfer hit and lost by no more than the hit", true},
	{"🧯", "Chip Flop", "Worst score by anyone playing a chip", true},
}

type fplAwardColumn struct {
	Icon  string
	Title string
	Desc  string
	Shame bool
}

// fplAwardTally is a manager's awards across the season; Counts lines up
// with fplAwardColumns.
type fplAwardTally struct {
	Entry  int
	Name   string
	Counts []int
	Fame   int
	Shame  int
}

// fplMoneyCell is a manager's season money in the league table.
type fplMoneyCell struct {
	Value string
	Sign  int
}

type fplTemplateData struct {
	LeagueName      string
	Standings       []fpl.Standing
	Upcoming        *fplEventView
	Latest          *fplEventView
	Money           map[int]fplMoneyCell    // entry → season money
	AveragePot      string                  // AVERAGE pot waiting for a winner, "" when empty
	Past            []*fplEventView         // settled gameweeks, newest first
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
		history := s.fplHistory.Get()
		awards := buildFPLAwards(data, history)

		td := fplTemplateData{
			LeagueName:      data.LeagueName,
			Standings:       data.Standings,
			Money:           buildFPLMoney(ledger),
			AveragePot:      fplPot(ledger.Pot),
			Icons:           icons,
			AwardTally:      buildFPLAwardTally(data, awards),
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
			fplApplyMatchups(td.Latest, data, live, history)
		}
		if next, ok := data.NextEvent(); ok && (td.Latest == nil || td.Latest.Status != "live") {
			td.Upcoming = buildFPLEvent(data, ledger, live, icons, next)
			fplApplyMatchups(td.Upcoming, data, live, history)
		}
		for i := len(data.Events) - 1; i >= 0; i-- {
			if e := data.Events[i]; e.Settled() && (td.Latest == nil || e.ID != td.Latest.ID) {
				ev := buildFPLEvent(data, ledger, live, icons, e)
				fplApplyAwards(ev, data, awards[e.ID])
				fplApplyMatchups(ev, data, live, history)
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

// fplApplyMatchups adds the team comparison to each fixture of a
// gameweek. Settled gameweeks use the stored teams; the current gameweek
// uses the live teams once its deadline has passed; before a deadline the
// teams are secret, so the previous gameweek's teams stand in as a preview.
func fplApplyMatchups(ev *fplEventView, data fpl.LeagueData, live fpl.LiveData, history map[int]fpl.GameweekHistory) {
	var picks map[int]fpl.Picks
	var stats map[int]fpl.PlayerStats
	mode := "final"
	switch h, settled := history[ev.ID]; {
	case settled:
		picks, stats = h.Picks, h.Stats
	case live.PicksEvent == ev.ID:
		picks, mode = live.Picks, "live"
		if live.Event == ev.ID {
			stats = live.Stats
		}
	case ev.Status == "upcoming" && live.PicksEvent != 0:
		picks, mode = live.Picks, "preview"
	default:
		return
	}
	points := fpl.PointsOf(stats)
	if mode == "preview" {
		points = nil
	}
	pb := fplPlayerBuilder{data: data, event: ev.ID, stats: stats, preview: mode == "preview"}

	for i := range ev.Fixtures {
		f := &ev.Fixtures[i]
		if f.Home.IsAverage || f.Away.IsAverage {
			continue
		}
		a, okA := picks[f.Home.Entry]
		b, okB := picks[f.Away.Entry]
		if !okA || !okB {
			continue
		}
		var tA, tB []fpl.Transfer
		if mode != "preview" {
			tA, tB = fpl.TransfersFor(live.Transfers[f.Home.Entry], ev.ID), fpl.TransfersFor(live.Transfers[f.Away.Entry], ev.ID)
		}
		m := fpl.BuildMatchup(a, b, points, tA, tB)
		v := &fplMatchupView{Mode: mode}
		if mode == "preview" {
			v.Note = fmt.Sprintf("Preview from GW%d teams · GW%d teams are secret until the deadline", live.PicksEvent, ev.ID)
		}
		if m.A.Captain.Element != 0 && m.A.Captain.Element == m.B.Captain.Element && m.A.Captain.Multiplier == m.B.Captain.Multiplier {
			c := pb.player(m.A.Captain.Element, m.A.Captain.Multiplier)
			v.SameCaptain = &c
		}
		side := func(name string, s fpl.MatchupSide) fplMatchupSide {
			out := fplMatchupSide{Name: name, Chip: fpl.ChipLabel(s.Chip)}
			if mode != "preview" {
				out.Hit = s.TransferCost
			}
			count := func(p fplMatchPlayer) {
				out.OnlyPts += p.Points
				switch p.Status {
				case "upcoming":
					out.ToPlay++
				case "live":
					out.Playing++
				}
			}
			if s.Captain.Element != 0 && v.SameCaptain == nil {
				c := pb.player(s.Captain.Element, s.Captain.Multiplier)
				out.Captain = &c
				count(c)
			}
			for _, p := range s.Only {
				if out.Captain != nil && p.Element == s.Captain.Element {
					continue // already counted as captain
				}
				count(pb.player(p.Element, p.Multiplier))
			}
			for _, p := range s.Bench {
				bp := pb.player(p.Element, 1)
				out.BenchPts += bp.Points
				out.Bench = append(out.Bench, bp)
			}
			for j := range s.TransfersIn {
				t := fplTransferView{In: pb.player(s.TransfersIn[j], 1), Out: pb.player(s.TransfersOut[j], 1)}
				if stats != nil {
					switch {
					case t.In.Points > t.Out.Points:
						t.Verdict = "win"
					case t.In.Points < t.Out.Points:
						t.Verdict = "loss"
					}
				}
				out.Transfers = append(out.Transfers, t)
			}
			return out
		}
		v.Home, v.Away = side(f.Home.Name, m.A), side(f.Away.Name, m.B)

		// The captain is listed on its own, so leave it out of the rows.
		only := func(s fpl.MatchupSide) []fplMatchPlayer {
			var out []fplMatchPlayer
			for _, p := range s.Only {
				if p.Element == s.Captain.Element && v.SameCaptain == nil {
					continue
				}
				out = append(out, pb.player(p.Element, p.Multiplier))
			}
			if mode == "preview" {
				sort.SliceStable(out, func(i, j int) bool { return fplPosOrder[out[i].Pos] < fplPosOrder[out[j].Pos] })
			}
			return out
		}
		homeOnly, awayOnly := only(m.A), only(m.B)
		if mode == "preview" {
			// No points to rank by yet, so line positions up against each other.
			for _, pos := range []string{"gk", "def", "mid", "fwd"} {
				h, a := fplByPos(homeOnly, pos), fplByPos(awayOnly, pos)
				if len(h)+len(a) == 0 {
					continue
				}
				v.Rows = append(v.Rows, fplDuelRow{Header: fplPosLabels[pos]})
				v.Rows = append(v.Rows, fplPair(h, a)...)
			}
		} else {
			v.Rows = fplPair(homeOnly, awayOnly)
		}
		for _, p := range m.Shared {
			// Captains have their own line, shared or not.
			if p.Element == m.A.Captain.Element || p.Element == m.B.Captain.Element {
				continue
			}
			v.Shared = append(v.Shared, pb.player(p.Element, 1))
		}
		f.Matchup = v
	}
}

// fplPair lines two lists up row by row; the shorter side gets gaps.
func fplPair(home, away []fplMatchPlayer) []fplDuelRow {
	var rows []fplDuelRow
	for r := 0; r < max(len(home), len(away)); r++ {
		var row fplDuelRow
		if r < len(home) {
			row.Home = &home[r]
		}
		if r < len(away) {
			row.Away = &away[r]
		}
		rows = append(rows, row)
	}
	return rows
}

func fplByPos(players []fplMatchPlayer, pos string) []fplMatchPlayer {
	var out []fplMatchPlayer
	for _, p := range players {
		if p.Pos == pos {
			out = append(out, p)
		}
	}
	return out
}

var fplPosLabels = map[string]string{"gk": "GK", "def": "DEF", "mid": "MID", "fwd": "FWD"}

// fplPlayerBuilder turns an element id into a matchup player for one
// gameweek: what they scored and did, and whether their game is done.
type fplPlayerBuilder struct {
	data    fpl.LeagueData
	event   int
	stats   map[int]fpl.PlayerStats
	preview bool
}

var fplDublin = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Dublin")
	if err != nil {
		return time.UTC
	}
	return loc
}()

func (b fplPlayerBuilder) player(el, mult int) fplMatchPlayer {
	p, ok := b.data.Players[el]
	if !ok {
		return fplMatchPlayer{Name: "?", Multiplier: mult, Status: "blank"}
	}
	st := b.stats[el]
	out := fplMatchPlayer{
		Name: p.Name, Club: p.Team, Pos: fplPositions[p.Position], PosLabel: fplPosLabels[fplPositions[p.Position]], Multiplier: mult,
		Points: st.Points * mult, Bonus: st.Bonus,
	}
	if !b.preview {
		out.Events = fplStatEvents(st, out.Pos)
	}
	if b.preview && p.Unavailable() {
		out.Flag, out.FlagOut = p.StatusLabel(), p.Out()
	}

	fixtures := b.data.TeamFixtures(p.TeamID, b.event)
	if len(fixtures) == 0 {
		out.Status, out.When = "blank", "No game"
		return out
	}
	var parts []string
	allDone, anyLive := true, false
	for _, f := range fixtures {
		opp, ha, fdr := f.Away, "H", f.HomeDifficulty
		if f.Away == p.TeamID {
			opp, ha, fdr = f.Home, "A", f.AwayDifficulty
		}
		vs := fmt.Sprintf("v %s (%s)", b.data.Clubs[opp], ha)
		if !f.Started {
			out.Next = append(out.Next, fplNextFixture{Kick: f.Kickoff.In(fplDublin).Format("Mon 15:04"), Vs: vs, FDR: fdr})
		}
		switch {
		case f.Finished:
			parts = append(parts, vs)
		case f.Started:
			anyLive, allDone = true, false
			parts = append(parts, fmt.Sprintf("%s %d'", vs, f.Minutes))
		default:
			allDone = false
			parts = append(parts, f.Kickoff.In(fplDublin).Format("Mon 15:04")+" "+vs)
		}
	}
	out.When = strings.Join(parts, " · ")
	switch {
	case anyLive:
		out.Status = "live"
	case !allDone:
		out.Status = "upcoming"
	case st.Minutes == 0:
		out.Status, out.When = "dnp", "Didn't play · "+out.When
	default:
		out.Status = "played"
	}
	return out
}

// fplStatEvents turns a player's stats into the icons shown beside them.
func fplStatEvents(s fpl.PlayerStats, pos string) []fplStatEvent {
	var out []fplStatEvent
	add := func(icon string, n int, title string) {
		if n > 0 {
			out = append(out, fplStatEvent{Icon: icon, Count: n, Title: title})
		}
	}
	add("⚽", s.Goals, "Goal")
	add("🅰️", s.Assists, "Assist")
	if pos == "gk" || pos == "def" {
		add("🧱", s.CleanSheets, "Clean sheet")
	}
	if pos == "gk" {
		add("🧤", s.Saves/3, "3 saves")
	}
	add("🥅", s.PenaltiesSaved, "Penalty saved")
	add("❌", s.PenaltiesMissed, "Penalty missed")
	add("🙈", s.OwnGoals, "Own goal")
	add("🟨", s.YellowCards, "Yellow card")
	add("🟥", s.RedCards, "Red card")
	return out
}

var fplPositions = map[int]string{1: "gk", 2: "def", 3: "mid", 4: "fwd"}
var fplPosOrder = map[string]int{"gk": 0, "def": 1, "mid": 2, "fwd": 3}

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

// buildFPLAwardTally counts each manager's awards over the season, in
// league-table order so it lines up with the table below. Managers with no
// awards are left out.
func buildFPLAwardTally(data fpl.LeagueData, awards map[int][]fpl.Award) []fplAwardTally {
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
		t := fplAwardTally{Entry: m.Entry, Name: m.DisplayName, Counts: c}
		for i, n := range c {
			if fplAwardColumns[i].Shame {
				t.Shame += n
			} else {
				t.Fame += n
			}
		}
		out = append(out, t)
	}
	rank := map[int]int{}
	for _, s := range data.Standings {
		rank[s.Side.Entry] = s.Rank
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, okI := rank[out[i].Entry]
		rj, okJ := rank[out[j].Entry]
		if okI != okJ {
			return okI // managers in the table first
		}
		if ri != rj {
			return ri < rj
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
	Icon   string // identifies the status (one of the fplIcon constants)
	Kind   string // badge style: "leader", "money", "spoon", "motw", "form-w", "form-l", "naming", "injury"
	Count  int    // run length for form badges
	Title  string
	Detail string // "Eoghan: top of the H2H table on 21 pts"
	Bad    bool
}

// fplIconKinds maps each status to its league-table style badge.
var fplIconKinds = map[string]string{
	fplIconLeague:  "leader",
	fplIconMoney:   "money",
	fplIconSpoon:   "spoon",
	fplIconStar:    "motw",
	fplIconStreak:  "form-w",
	fplIconSlump:   "form-l",
	fplIconNaming:  "naming",
	fplIconInjured: "injury",
}

// buildFPLIcons gives each manager their current status icons. Like the
// World Cup page, icons reflect standings now, not a particular gameweek.
func buildFPLIcons(data fpl.LeagueData, l fpl.Ledger, live fpl.LiveData) map[int][]fplStatusIcon {
	names := fplNameMap(data)
	icons := map[int][]fplStatusIcon{}
	add := func(entry int, icon, title, detail string, bad bool) {
		icons[entry] = append(icons[entry], fplStatusIcon{Icon: icon, Kind: fplIconKinds[icon], Title: title, Detail: names[entry] + ": " + detail, Bad: bad})
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
		n := streaks[m.Entry]
		switch {
		case n >= 3:
			add(m.Entry, fplIconStreak, "On fire", fmt.Sprintf("%d H2H wins in a row", n), false)
		case n <= -3:
			add(m.Entry, fplIconSlump, "Ice cold", fmt.Sprintf("%d H2H losses in a row", -n), true)
		}
		if l := len(icons[m.Entry]); l > 0 && (n >= 3 || n <= -3) {
			icons[m.Entry][l-1].Count = max(n, -n)
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

func buildFPLMoney(l fpl.Ledger) map[int]fplMoneyCell {
	out := make(map[int]fplMoneyCell, len(l.Rows))
	for _, r := range l.Rows {
		out[r.Manager.Entry] = fplMoneyCell{Value: r.Season().String(), Sign: fplSign(r.Season())}
	}
	return out
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
		ev.Fixtures = append(ev.Fixtures, fplFixtureView{Home: home, Away: away})
	}
	return ev
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
