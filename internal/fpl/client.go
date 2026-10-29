package fpl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

const defaultBaseURL = "https://fantasy.premierleague.com/api"

// maxPages guards the paging loops against an API that never clears has_next.
const maxPages = 20

// FPL rejects requests without a browser-like User-Agent.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"

// Client fetches league data from the public FPL API. No login is needed.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a Client for the live FPL API.
func NewClient() *Client {
	return &Client{BaseURL: defaultBaseURL, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// ── Raw API shapes ──────────────────────────────────────────────────────

type apiStandings struct {
	League struct {
		Name string `json:"name"`
	} `json:"league"`
	Standings struct {
		HasNext bool `json:"has_next"`
		Results []struct {
			Rank       int    `json:"rank"`
			LastRank   int    `json:"last_rank"`
			Entry      *int   `json:"entry"`
			PlayerName string `json:"player_name"`
			EntryName  string `json:"entry_name"`
			Won        int    `json:"matches_won"`
			Drawn      int    `json:"matches_drawn"`
			Lost       int    `json:"matches_lost"`
			Total      int    `json:"total"`
			PointsFor  int    `json:"points_for"`
		} `json:"results"`
	} `json:"standings"`
}

type apiMatches struct {
	HasNext bool `json:"has_next"`
	Results []struct {
		ID           int    `json:"id"`
		Event        int    `json:"event"`
		Entry1       *int   `json:"entry_1_entry"`
		Entry1Name   string `json:"entry_1_name"`
		Entry1Player string `json:"entry_1_player_name"`
		Entry1Points int    `json:"entry_1_points"`
		Entry2       *int   `json:"entry_2_entry"`
		Entry2Name   string `json:"entry_2_name"`
		Entry2Player string `json:"entry_2_player_name"`
		Entry2Points int    `json:"entry_2_points"`
	} `json:"results"`
}

type apiBootstrap struct {
	Teams []struct {
		ID        int    `json:"id"`
		ShortName string `json:"short_name"`
	} `json:"teams"`
	Elements []struct {
		ID          int    `json:"id"`
		WebName     string `json:"web_name"`
		Team        int    `json:"team"`
		ElementType int    `json:"element_type"`
		Status      string `json:"status"`
		Chance      *int   `json:"chance_of_playing_next_round"`
		News        string `json:"news"`
	} `json:"elements"`
	Events []struct {
		ID          int       `json:"id"`
		Name        string    `json:"name"`
		Deadline    time.Time `json:"deadline_time"`
		Finished    bool      `json:"finished"`
		DataChecked bool      `json:"data_checked"`
		IsCurrent   bool      `json:"is_current"`
		IsNext      bool      `json:"is_next"`
	} `json:"events"`
}

func (c *Client) getJSON(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("fpl: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fpl: GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("fpl: decode %s: %w", path, err)
	}
	return nil
}

// FetchLeague fetches standings, gameweeks and every H2H match up to and
// including the next gameweek.
func (c *Client) FetchLeague(leagueID int) (LeagueData, error) {
	var boot apiBootstrap
	if err := c.getJSON("/bootstrap-static/", &boot); err != nil {
		return LeagueData{}, err
	}
	data := LeagueData{}
	lastEvent := 0
	for _, e := range boot.Events {
		data.Events = append(data.Events, Event{
			ID: e.ID, Name: e.Name, Deadline: e.Deadline,
			Finished: e.Finished, DataChecked: e.DataChecked,
			IsCurrent: e.IsCurrent, IsNext: e.IsNext,
		})
		if e.IsCurrent || e.IsNext {
			lastEvent = max(lastEvent, e.ID)
		}
	}

	sort.Slice(data.Events, func(i, j int) bool { return data.Events[i].ID < data.Events[j].ID })

	clubs := map[int]string{}
	for _, t := range boot.Teams {
		clubs[t.ID] = t.ShortName
	}
	data.Players = make(map[int]Player, len(boot.Elements))
	for _, e := range boot.Elements {
		data.Players[e.ID] = Player{ID: e.ID, Name: e.WebName, Team: clubs[e.Team], Status: e.Status, Chance: e.Chance, News: e.News, Position: e.ElementType}
	}

	players := map[int]string{}
	teams := map[int]string{}
	var rawStandings apiStandings
	for page := 1; page <= maxPages; page++ {
		var st apiStandings
		if err := c.getJSON(fmt.Sprintf("/leagues-h2h/%d/standings/?page_standings=%d", leagueID, page), &st); err != nil {
			return LeagueData{}, err
		}
		if page == 1 {
			rawStandings.League = st.League
		}
		rawStandings.Standings.Results = append(rawStandings.Standings.Results, st.Standings.Results...)
		if !st.Standings.HasNext {
			break
		}
	}
	data.LeagueName = rawStandings.League.Name
	for _, r := range rawStandings.Standings.Results {
		if r.Entry != nil {
			players[*r.Entry] = r.PlayerName
			teams[*r.Entry] = r.EntryName
		}
	}

	for event := 1; event <= lastEvent; event++ {
		for page := 1; page <= maxPages; page++ {
			var m apiMatches
			if err := c.getJSON(fmt.Sprintf("/leagues-h2h-matches/league/%d/?event=%d&page=%d", leagueID, event, page), &m); err != nil {
				return LeagueData{}, err
			}
			for _, r := range m.Results {
				// Managers who left the league still appear in old matches.
				for _, s := range []struct {
					entry        *int
					player, team string
				}{{r.Entry1, r.Entry1Player, r.Entry1Name}, {r.Entry2, r.Entry2Player, r.Entry2Name}} {
					if s.entry != nil {
						if _, ok := players[*s.entry]; !ok {
							players[*s.entry] = s.player
							teams[*s.entry] = s.team
						}
					}
				}
				data.Matches = append(data.Matches, Match{
					ID: r.ID, Event: r.Event,
					Home: Side{Entry: deref(r.Entry1), Points: r.Entry1Points},
					Away: Side{Entry: deref(r.Entry2), Points: r.Entry2Points},
				})
			}
			if !m.HasNext {
				break
			}
		}
	}

	names := displayNames(players)
	for entry, full := range players {
		data.Managers = append(data.Managers, Manager{Entry: entry, PlayerName: full, TeamName: teams[entry], DisplayName: names[entry]})
	}
	sort.Slice(data.Managers, func(i, j int) bool { return data.Managers[i].DisplayName < data.Managers[j].DisplayName })

	side := func(entry, points int) Side {
		if entry == 0 {
			return Side{Name: "AVERAGE", TeamName: "AVERAGE", Points: points}
		}
		return Side{Entry: entry, Name: names[entry], TeamName: teams[entry], Points: points}
	}
	for i, m := range data.Matches {
		data.Matches[i].Home = side(m.Home.Entry, m.Home.Points)
		data.Matches[i].Away = side(m.Away.Entry, m.Away.Points)
	}
	for _, r := range rawStandings.Standings.Results {
		data.Standings = append(data.Standings, Standing{
			Rank: r.Rank, LastRank: r.LastRank, Side: side(deref(r.Entry), 0),
			Won: r.Won, Drawn: r.Drawn, Lost: r.Lost, Total: r.Total, PointsFor: r.PointsFor,
		})
	}
	data.FetchedAt = time.Now()
	return data, nil
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
