package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/conorfennell/knolhash/internal/fpl"
)

type fplSideView struct {
	Name      string
	TeamName  string
	Points    int
	Outcome   string // "win", "loss", "draw" or "" when not yet played
	IsAverage bool
	Money     string // net € for the gameweek, "" until the gameweek is settled
	MoneySign int    // -1, 0, 1 for styling
	Top       bool   // gameweek top scorer
	Bottom    bool   // in the gameweek's bottom two
	Capped    bool   // H2H loss reduced by the −€50 cap
	Chip      string // chip badge ("TC", "BB", …), "" when none
}

type fplFixtureView struct {
	Home fplSideView
	Away fplSideView
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

type fplMoneyRow struct {
	Rank     int
	Name     string
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
	Past            []*fplEventView // settled gameweeks, newest first
	Summary         string          // WhatsApp post for the latest settled gameweek
	SummaryEvent    int
	NamingRights    *fplNamingView
	Availability    []fplAvailabilityView
	TeamsEvent      int // gameweek the availability teams are from
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

		td := fplTemplateData{
			LeagueName:      data.LeagueName,
			Standings:       data.Standings,
			Money:           buildFPLMoney(ledger),
			FetchedAt:       data.FetchedAt,
			NextRefreshUnix: s.fplCache.NextRefresh().Unix(),
			Commit:          shortCommit(s.commit),
			BuildDate:       shortBuildDate(s.buildDate),
		}
		// While a gameweek is in progress it is the only one worth showing at
		// the top. Between gameweeks, show next week's fixtures and last
		// week's results.
		if cur, ok := data.CurrentEvent(); ok {
			td.Latest = buildFPLEvent(data, ledger, live, cur)
			td.CurrentHalf = fpl.Half(cur.ID)
		}
		if next, ok := data.NextEvent(); ok && (td.Latest == nil || td.Latest.Status != "live") {
			td.Upcoming = buildFPLEvent(data, ledger, live, next)
		}
		if n := len(ledger.Gameweeks); n > 0 {
			td.SummaryEvent = ledger.Gameweeks[n-1].Event
			td.Summary, _ = fpl.WeeklySummary(data, ledger, td.SummaryEvent)
		}
		if nr, ok := fpl.BiggestMargin(data); ok {
			td.NamingRights = &fplNamingView{
				Event: nr.Event, Winner: nr.Winner.Name, Loser: nr.Loser.Name, Margin: nr.Margin,
				Score: fmt.Sprintf("%d – %d", nr.Winner.Points, nr.Loser.Points),
			}
		}
		td.Availability, td.TeamsEvent = buildFPLAvailability(data, live), live.PicksEvent
		for i := len(data.Events) - 1; i >= 0; i-- {
			if e := data.Events[i]; e.Settled() && (td.Latest == nil || e.ID != td.Latest.ID) {
				td.Past = append(td.Past, buildFPLEvent(data, ledger, live, e))
			}
		}

		if err := s.templates.ExecuteTemplate(w, "fpl", td); err != nil {
			slog.Error("fpl: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func buildFPLAvailability(data fpl.LeagueData, live fpl.LiveData) []fplAvailabilityView {
	var out []fplAvailabilityView
	for _, m := range data.Managers {
		v := fplAvailabilityView{Name: m.DisplayName, TeamName: m.TeamName}
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

func buildFPLMoney(l fpl.Ledger) []fplMoneyRow {
	var rows []fplMoneyRow
	for i, r := range l.Rows {
		rank := i + 1
		if i > 0 && r.Season() == l.Rows[i-1].Season() {
			rank = rows[i-1].Rank
		}
		rows = append(rows, fplMoneyRow{
			Rank:     rank,
			Name:     r.Manager.DisplayName,
			TeamName: r.Manager.TeamName,
			Halves:   [2]string{r.Halves[0].String(), r.Halves[1].String()},
			Season:   r.Season().String(),
			Sign:     fplSign(r.Season()),
		})
	}
	return rows
}

func buildFPLEvent(data fpl.LeagueData, l fpl.Ledger, live fpl.LiveData, e fpl.Event) *fplEventView {
	ev := &fplEventView{ID: e.ID, Name: e.Name, Deadline: e.Deadline, Status: e.Status()}
	ev.Live = ev.Status == "live" && live.Event == e.ID
	g, settled := l.Gameweek(e.ID)
	for _, m := range data.MatchesFor(e.ID) {
		home, away := fplSide(m.Home, g, settled), fplSide(m.Away, g, settled)
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
	if settled {
		ev.Summary = "Top: " + fplNames(data, g.TopScorers) + " · Bottom two: " + fplNames(data, g.BottomTwo)
	}
	return ev
}

func fplSide(s fpl.Side, g fpl.GameweekLedger, settled bool) fplSideView {
	v := fplSideView{Name: s.Name, TeamName: s.TeamName, Points: s.Points, IsAverage: s.IsAverage()}
	if w, ok := g.Weeks[s.Entry]; settled && ok {
		v.Money, v.MoneySign = w.Total().String(), fplSign(w.Total())
		v.Top, v.Bottom, v.Capped = w.Top > 0, w.Bottom < 0, w.Capped
	}
	return v
}

func fplLivePoints(s fpl.Side, live fpl.LiveData) int {
	if p, ok := live.ScoreFor(s); ok {
		return p
	}
	return s.Points
}

func fplNames(data fpl.LeagueData, entries []int) string {
	names := map[int]string{}
	for _, m := range data.Managers {
		names[m.Entry] = m.DisplayName
	}
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

func fplSign(m fpl.Money) int {
	switch {
	case m > 0:
		return 1
	case m < 0:
		return -1
	}
	return 0
}
