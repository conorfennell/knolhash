package fpl

import "time"

// Fixture is a Premier League match.
type Fixture struct {
	Event          int
	Home, Away     int // team ids
	Kickoff        time.Time
	Started        bool
	Finished       bool
	Minutes        int
	HomeScore      *int
	AwayScore      *int
	HomeDifficulty int // FPL difficulty (1 easy – 5 hard) for the home side
	AwayDifficulty int
}

type apiFixture struct {
	Event               *int      `json:"event"`
	TeamH               int       `json:"team_h"`
	TeamA               int       `json:"team_a"`
	Kickoff             time.Time `json:"kickoff_time"`
	Started             bool      `json:"started"`
	Finished            bool      `json:"finished"`
	FinishedProvisional bool      `json:"finished_provisional"`
	Minutes             int       `json:"minutes"`
	TeamHScore          *int      `json:"team_h_score"`
	TeamAScore          *int      `json:"team_a_score"`
	TeamHDifficulty     int       `json:"team_h_difficulty"`
	TeamADifficulty     int       `json:"team_a_difficulty"`
}

// TeamFixtures returns a club's matches in a gameweek: none in a blank
// gameweek, two in a double.
func (d LeagueData) TeamFixtures(team, event int) []Fixture {
	var out []Fixture
	for _, f := range d.Fixtures {
		if f.Event == event && (f.Home == team || f.Away == team) {
			out = append(out, f)
		}
	}
	return out
}
