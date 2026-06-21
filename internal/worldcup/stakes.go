package worldcup

// StakeScenario is the projected top-3 Least Points leaderboard under one outcome.
type StakeScenario struct {
	Label   string // "Spain win" or "Saudi Arabia win"
	TopBest []EntryResult
}

// MatchStake shows the projected leaderboard impact of each outcome in an upcoming match.
type MatchStake struct {
	HomeTeam    string
	AwayTeam    string
	HomeWin     StakeScenario
	AwayWin     StakeScenario
}

// ComputeMatchStakes returns one MatchStake per upcoming match that involves
// at least one sweepstake entry's team, with simulated leaderboard outcomes.
func ComputeMatchStakes(matches []Match, data TournamentData) []MatchStake {
	var upcoming []Match
	for _, m := range matches {
		if !m.Played {
			upcoming = append(upcoming, m)
		}
	}
	// Only show first 4 upcoming matches to keep it focused.
	if len(upcoming) > 4 {
		upcoming = upcoming[:4]
	}

	var out []MatchStake
	for _, m := range upcoming {
		homeWin := simulateOutcome(m.HomeTeam, m.AwayTeam, data)
		awayWin := simulateOutcome(m.AwayTeam, m.HomeTeam, data)

		// Only include if at least one entry has a team in this match.
		if !anyEntryHasTeam(m.HomeTeam) && !anyEntryHasTeam(m.AwayTeam) {
			continue
		}

		out = append(out, MatchStake{
			HomeTeam: m.HomeTeam,
			AwayTeam: m.AwayTeam,
			HomeWin:  StakeScenario{Label: m.HomeTeam + " win", TopBest: homeWin},
			AwayWin:  StakeScenario{Label: m.AwayTeam + " win", TopBest: awayWin},
		})
	}
	return out
}

// simulateOutcome returns the top-5 Least Points results assuming winner wins
// and loser loses. Group positions are adjusted: if either team was 4th they
// may move to 3rd (or vice versa), which changes their estimated points by 17.
func simulateOutcome(winner, loser string, data TournamentData) []EntryResult {
	// Deep copy teams map.
	teams := make(map[string]TeamState, len(data.Teams))
	for k, v := range data.Teams {
		teams[k] = v
	}

	// Adjust winner: if currently 4th, promote to 3rd.
	if ws, ok := teams[winner]; ok && ws.GroupPosition == 4 && ws.FinalPlace == 0 {
		ws.GroupPosition = 3
		teams[winner] = ws
	}
	// Adjust loser: if currently 3rd or better, may drop to 4th.
	if ls, ok := teams[loser]; ok && ls.GroupPosition == 3 && ls.FinalPlace == 0 {
		ls.GroupPosition = 4
		teams[loser] = ls
	}

	simData := TournamentData{Teams: teams, Matches: data.Matches, FetchedAt: data.FetchedAt}
	results := ScoreAll(Entries, simData)
	if len(results) > 5 {
		results = results[:5]
	}
	return results
}

func anyEntryHasTeam(team string) bool {
	for _, e := range Entries {
		for _, t := range e.Teams {
			if t == team {
				return true
			}
		}
	}
	return false
}
