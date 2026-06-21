package worldcup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const espnScoreboardURL = "https://site.api.espn.com/apis/site/v2/sports/soccer/fifa.world/scoreboard"

var espnToEntry = map[string]string{
	"United States":       "USA",
	"Curaçao":             "Curacao",
	"Cape Verde":          "Cabo Verde",
	"Bosnia-Herzegovina":  "Bosnia",
	"Türkiye":             "Turkey",
}

func espnTeamName(name string) string {
	if mapped, ok := espnToEntry[name]; ok {
		return mapped
	}
	return name
}

type espnScoreboard struct {
	Events []espnEvent `json:"events"`
}

type espnEvent struct {
	Name         string         `json:"name"`
	Competitions []espnComp     `json:"competitions"`
}

type espnComp struct {
	Competitors []espnCompetitor `json:"competitors"`
	Status      espnStatus       `json:"status"`
	Details     []espnDetail     `json:"details"`
}

type espnDetail struct {
	Type   espnDetailType `json:"type"`
	Team   espnDetailTeam `json:"team"`
	Yellow bool           `json:"yellowCard"`
	Red    bool           `json:"redCard"`
}

type espnDetailType struct {
	Text string `json:"text"`
}

type espnDetailTeam struct {
	ID string `json:"id"`
}

type espnCompetitor struct {
	HomeAway string   `json:"homeAway"`
	Score    string   `json:"score"`
	Team     espnTeam `json:"team"`
}

type espnTeam struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

type espnStatus struct {
	DisplayClock string         `json:"displayClock"`
	Type         espnStatusType `json:"type"`
}

type espnStatusType struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

// FetchLiveMatchesESPN calls the ESPN scoreboard API and returns matches
// currently in play. Returns (matches, 99, error) — the 99 is a dummy
// quota value since ESPN has no rate limit header.
func FetchLiveMatchesESPN() ([]LiveMatch, int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(espnScoreboardURL)
	if err != nil {
		return nil, 0, fmt.Errorf("espn: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("espn: status %d", resp.StatusCode)
	}

	var payload espnScoreboard
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, fmt.Errorf("espn: decode: %w", err)
	}

	var out []LiveMatch
	for _, event := range payload.Events {
		if len(event.Competitions) == 0 {
			continue
		}
		comp := event.Competitions[0]
		status := comp.Status.Type.Name
		// Use state field ("in" = in progress) rather than enumerating status names.
		if comp.Status.Type.State != "in" {
			continue
		}

		var homeTeam, awayTeam, homeID, awayID string
		var homeScore, awayScore int
		for _, c := range comp.Competitors {
			score, _ := strconv.Atoi(c.Score)
			name := espnTeamName(c.Team.DisplayName)
			if c.HomeAway == "home" {
				homeTeam = name
				homeScore = score
				homeID = c.Team.ID
			} else {
				awayTeam = name
				awayScore = score
				awayID = c.Team.ID
			}
		}

		// Count cards from details.
		var homeYellow, homeRed, awayYellow, awayRed int
		for _, d := range comp.Details {
			if d.Yellow {
				if d.Team.ID == homeID {
					homeYellow++
				} else if d.Team.ID == awayID {
					awayYellow++
				}
			}
			if d.Red {
				if d.Team.ID == homeID {
					homeRed++
				} else if d.Team.ID == awayID {
					awayRed++
				}
			}
		}

		minute := parseESPNClock(comp.Status.DisplayClock)

		liveStatus := "IN_PLAY"
		if status == "STATUS_HALFTIME" || comp.Status.Type.Detail == "HT" {
			liveStatus = "PAUSED"
		}

		out = append(out, LiveMatch{
			HomeTeam:   homeTeam,
			AwayTeam:   awayTeam,
			HomeScore:  homeScore,
			AwayScore:  awayScore,
			Status:     liveStatus,
			Minute:     minute,
			HomeYellow: homeYellow,
			HomeRed:    homeRed,
			AwayYellow: awayYellow,
			AwayRed:    awayRed,
		})
	}

	return out, 99, nil
}

func parseESPNClock(clock string) int {
	// e.g. "45'+2'", "67'", "90'+3'"
	s := strings.TrimRight(clock, "'")
	if i := strings.Index(s, "+"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "'")
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
