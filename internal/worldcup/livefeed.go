package worldcup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const fdoBaseURL = "https://api.football-data.org/v4"

// LiveMatch holds the current in-play state of a match from football-data.org.
type LiveMatch struct {
	HomeTeam  string
	AwayTeam  string
	HomeScore int
	AwayScore int
	Status    string // IN_PLAY, PAUSED
	Minute    int
}

// fdoToEntry maps football-data.org team names that differ from our canonical names.
// Verified against GET /v4/competitions/WC/teams on 2026-06-14.
var fdoToEntry = map[string]string{
	"Curaçao":            "Curacao",
	"United States":      "USA",
	"Cape Verde Islands": "Cabo Verde",
	"Bosnia-Herzegovina": "Bosnia",
}

func fdoTeamName(name string) string {
	if mapped, ok := fdoToEntry[name]; ok {
		return mapped
	}
	return name
}

type fdoResponse struct {
	Matches []fdoMatch `json:"matches"`
}

type fdoMatch struct {
	Status   string   `json:"status"`
	Minute   int      `json:"minute"`
	HomeTeam fdoTeam  `json:"homeTeam"`
	AwayTeam fdoTeam  `json:"awayTeam"`
	Score    fdoScore `json:"score"`
}

type fdoTeam struct {
	Name string `json:"name"`
}

type fdoScore struct {
	FullTime fdoGoals `json:"fullTime"`
}

type fdoGoals struct {
	Home *int `json:"home"`
	Away *int `json:"away"`
}

// FetchLiveMatches calls football-data.org and returns matches currently
// IN_PLAY or PAUSED (half time). Also returns remaining requests this minute
// so the caller can throttle if needed.
func FetchLiveMatches(apiKey string) ([]LiveMatch, int, error) {
	req, err := http.NewRequest("GET", fdoBaseURL+"/competitions/WC/matches?status=LIVE", nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Auth-Token", apiKey)
	req.Header.Set("User-Agent", "knolhash-worldcup/1.0 (sweepstake; personal)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("football-data.org: %w", err)
	}
	defer resp.Body.Close()

	remaining := 10
	if v := resp.Header.Get("X-Requests-Available-Minute"); v != "" {
		if n, err2 := strconv.Atoi(v); err2 == nil {
			remaining = n
		}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, 0, fmt.Errorf("football-data.org: rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, remaining, fmt.Errorf("football-data.org: status %d", resp.StatusCode)
	}

	var payload fdoResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, remaining, fmt.Errorf("football-data.org: decode: %w", err)
	}

	var out []LiveMatch
	for _, m := range payload.Matches {
		if m.Status != "IN_PLAY" && m.Status != "PAUSED" {
			continue
		}
		homeScore, awayScore := 0, 0
		if m.Score.FullTime.Home != nil {
			homeScore = *m.Score.FullTime.Home
		}
		if m.Score.FullTime.Away != nil {
			awayScore = *m.Score.FullTime.Away
		}
		out = append(out, LiveMatch{
			HomeTeam:  fdoTeamName(m.HomeTeam.Name),
			AwayTeam:  fdoTeamName(m.AwayTeam.Name),
			HomeScore: homeScore,
			AwayScore: awayScore,
			Status:    m.Status,
			Minute:    m.Minute,
		})
	}

	return out, remaining, nil
}
