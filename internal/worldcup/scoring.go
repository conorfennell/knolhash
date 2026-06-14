package worldcup

import "sort"

// MatchSnapshot holds per-entry totals and prize rankings after one match.
type MatchSnapshot struct {
	Label       string         // "France vs Spain"
	GoalValues  map[string]int // entry → cumulative goals
	GoalRanks   map[string]int // entry → rank for Most Goals (1 = leading)
	PointValues map[string]int // entry → points accumulated
	LeastRanks  map[string]int // entry → rank for Least Points (1 = fewest pts)
	MostRanks   map[string]int // entry → rank for Most Points (1 = most pts)
}

// ComputeHistory replays all played matches in chronological order and returns
// one MatchSnapshot per match. Returns nil if no matches have been played.
func ComputeHistory(matches []Match, entries []Entry, data TournamentData) []MatchSnapshot {
	var played []Match
	for _, m := range matches {
		if m.Played {
			played = append(played, m)
		}
	}
	if len(played) == 0 {
		return nil
	}
	sort.Slice(played, func(i, j int) bool {
		a, b := played[i].KickOff, played[j].KickOff
		if a.IsZero() && b.IsZero() {
			return played[i].HomeTeam < played[j].HomeTeam
		}
		if a.IsZero() {
			return false
		}
		if b.IsZero() {
			return true
		}
		return a.Before(b)
	})

	teamGoals := make(map[string]int)
	var snapshots []MatchSnapshot

	for _, m := range played {
		teamGoals[m.HomeTeam] += m.HomeScore
		teamGoals[m.AwayTeam] += m.AwayScore

		// Build synthetic data: preserve FinalPlace/ThirdPlaceGroupRank from
		// current data but override GoalsFor with accumulated match-by-match goals.
		synthetic := TournamentData{Teams: make(map[string]TeamState, len(data.Teams))}
		for name, state := range data.Teams {
			s := state
			s.GoalsFor = teamGoals[name]
			synthetic.Teams[name] = s
		}
		for name, g := range teamGoals {
			if _, ok := synthetic.Teams[name]; !ok {
				synthetic.Teams[name] = TeamState{Name: name, GoalsFor: g}
			}
		}

		results := ScoreAll(entries, synthetic)

		// Goal-sorted order for Most Goals prize.
		goalSorted := make([]EntryResult, len(results))
		copy(goalSorted, results)
		sort.Slice(goalSorted, func(i, j int) bool {
			if goalSorted[i].TotalGoals != goalSorted[j].TotalGoals {
				return goalSorted[i].TotalGoals > goalSorted[j].TotalGoals
			}
			return goalSorted[i].Entry.Name < goalSorted[j].Entry.Name
		})

		snap := MatchSnapshot{
			Label:       m.HomeTeam + " vs " + m.AwayTeam,
			GoalValues:  make(map[string]int, len(results)),
			GoalRanks:   make(map[string]int, len(results)),
			PointValues: make(map[string]int, len(results)),
			LeastRanks:  make(map[string]int, len(results)),
			MostRanks:   make(map[string]int, len(results)),
		}
		for i, r := range goalSorted {
			snap.GoalValues[r.Entry.Name] = r.TotalGoals
			snap.GoalRanks[r.Entry.Name] = i + 1
		}
		// results is sorted ascending (fewest points first = leading Least Points).
		for i, r := range results {
			snap.PointValues[r.Entry.Name] = r.TotalPoints
			snap.LeastRanks[r.Entry.Name] = i + 1
			snap.MostRanks[r.Entry.Name] = len(results) - i
		}

		snapshots = append(snapshots, snap)
	}
	return snapshots
}

// RecentMatches returns the last n played matches, most recent first.
func RecentMatches(matches []Match, n int) []Match {
	var played []Match
	for _, m := range matches {
		if m.Played {
			played = append(played, m)
		}
	}
	sort.Slice(played, func(i, j int) bool {
		a, b := played[i].KickOff, played[j].KickOff
		if a.IsZero() && b.IsZero() {
			return played[i].HomeTeam < played[j].HomeTeam
		}
		if a.IsZero() {
			return false
		}
		if b.IsZero() {
			return true
		}
		return a.After(b)
	})
	if len(played) > n {
		return played[:n]
	}
	return played
}

// UpcomingMatches returns the next n unplayed matches, soonest first.
func UpcomingMatches(matches []Match, n int) []Match {
	var upcoming []Match
	for _, m := range matches {
		if !m.Played {
			upcoming = append(upcoming, m)
		}
	}
	sort.Slice(upcoming, func(i, j int) bool {
		a, b := upcoming[i].KickOff, upcoming[j].KickOff
		if a.IsZero() && b.IsZero() {
			return upcoming[i].HomeTeam < upcoming[j].HomeTeam
		}
		if a.IsZero() {
			return false
		}
		if b.IsZero() {
			return true
		}
		return a.Before(b)
	})
	if len(upcoming) > n {
		return upcoming[:n]
	}
	return upcoming
}


// TopByMostPoints returns top n entries sorted by most points descending
// (worst performers first). results must already be sorted ascending (ScoreAll output).
func TopByMostPoints(results []EntryResult, n int) []EntryResult {
	out := make([]EntryResult, 0, n)
	for i := len(results) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, results[i])
	}
	return out
}

// TopByMostGoals returns top n entries sorted by most goals descending,
// tiebroken by fewest points then alphabetical name.
func TopByMostGoals(results []EntryResult, n int) []EntryResult {
	sorted := make([]EntryResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TotalGoals != sorted[j].TotalGoals {
			return sorted[i].TotalGoals > sorted[j].TotalGoals
		}
		if sorted[i].TotalPoints != sorted[j].TotalPoints {
			return sorted[i].TotalPoints < sorted[j].TotalPoints
		}
		return sorted[i].Entry.Name < sorted[j].Entry.Name
	})
	if len(sorted) > n {
		return sorted[:n]
	}
	return sorted
}

// TopOverallWinner returns entries in contention for the Overall Winner prize.
// If a champion is known (FinalPlace==1), returns entries containing that team.
// Otherwise returns entries with at least one team currently leading its group
// (GroupPosition==1, not yet eliminated), ordered by current rank (best first).
func TopOverallWinner(results []EntryResult, data TournamentData, n int) []EntryResult {
	var champion string
	for name, state := range data.Teams {
		if state.FinalPlace == 1 {
			champion = name
			break
		}
	}
	if champion != "" {
		var out []EntryResult
		for _, r := range results {
			for _, t := range r.Entry.Teams {
				if t == champion {
					out = append(out, r)
					break
				}
			}
		}
		return out
	}

	if len(results) > n {
		return results[:n]
	}
	return results
}

// PlacementPoints returns the points for a final tournament placement.
// Lower is better (golf scoring). Returns 0 for place=0 (still active).
func PlacementPoints(place int) int {
	switch {
	case place == 0:
		return 0
	case place == 1:
		return 1
	case place == 2:
		return 2
	case place == 3:
		return 3
	case place == 4:
		return 4
	case place >= 5 && place <= 8:
		return 6
	case place >= 9 && place <= 16:
		return 12
	case place >= 17 && place <= 32:
		return 25
	case place == 33:
		return 33
	case place == 34:
		return 34
	case place == 35:
		return 35
	case place == 36:
		return 36
	case place >= 37 && place <= 48:
		return 42
	default:
		return 0
	}
}

// ThirdPlaceGroupBonus returns the additional points for a 3rd-placed group team
// that advanced to the knockouts. rank is 1 (best) through 8 (worst of the
// advancing 8). Returns 0 when rank is 0 (not applicable).
func ThirdPlaceGroupBonus(rank int) int {
	if rank < 1 || rank > 8 {
		return 0
	}
	return rank
}

// EstimatedGroupPoints returns a rough point estimate for a team that has no
// confirmed result yet (FinalPlace==0, ThirdPlaceGroupRank==0). It uses the
// team's current group position as a floor:
//   - 1st or 2nd: 25 pts (minimum once they lose in R32)
//   - 3rd (unranked): 25 pts (most 3rd-place teams advance to R32)
//   - 4th (mid-group, not yet finalized): 42 pts
//   - unknown: 0
func EstimatedGroupPoints(state TeamState) int {
	switch state.GroupPosition {
	case 1, 2, 3:
		return 25
	case 4:
		return 42
	default:
		return 0
	}
}

// ScoreEntry computes total points and goals for a single entry based on
// current tournament data. When no actual result exists yet, it falls back to
// estimatedGroupPoints so every team with a known group position contributes
// something to the leaderboard estimate.
func ScoreEntry(entry Entry, data TournamentData) EntryResult {
	result := EntryResult{Entry: entry}
	for i, teamName := range entry.Teams {
		state, ok := data.Teams[teamName]
		if !ok {
			state = TeamState{Name: teamName}
		}
		result.TeamStates[i] = state
		pts := PlacementPoints(state.FinalPlace) + ThirdPlaceGroupBonus(state.ThirdPlaceGroupRank)
		if pts == 0 {
			pts = EstimatedGroupPoints(state)
		}
		result.TotalPoints += pts
		result.TotalGoals += state.GoalsFor
	}
	return result
}

// ScoreAll scores every entry and returns results sorted ascending by total
// points (fewest points = best). Ties broken by fewest total goals, then
// alphabetically by entry name.
func ScoreAll(entries []Entry, data TournamentData) []EntryResult {
	results := make([]EntryResult, len(entries))
	for i, e := range entries {
		results[i] = ScoreEntry(e, data)
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.TotalPoints != b.TotalPoints {
			return a.TotalPoints < b.TotalPoints
		}
		return a.Entry.Name < b.Entry.Name
	})
	return results
}

// ComputePrizes calculates the four prize amounts given the current standings.
//
// Prize structure:
//   - Most points (worst combined score): fixed €5
//   - Most goals (most combined goals):   fixed €5
//   - Least points (best combined score): €5 + 50% of variable pool
//   - Overall winner (entry with the World Cup champion team): 50% of variable pool
//
// Variable pool = totalPot − 3×5 (the three fixed €5 slices).
func ComputePrizes(results []EntryResult, data TournamentData, totalPot int) PrizeSummary {
	if len(results) == 0 {
		return PrizeSummary{}
	}

	variablePool := float64(totalPot) - 15.0
	leastPrize := 5.0 + variablePool*0.5
	winnerPrize := variablePool * 0.5

	// results is sorted ascending by points; index 0 = best, last = worst.
	best := results[0]
	worst := results[len(results)-1]
	mostGoals := findMostGoals(results)
	overallWinnerName := findOverallWinner(results, data)

	return PrizeSummary{
		MostPoints:    PrizeResult{EntryName: worst.Entry.Name, Amount: 5.0},
		MostGoals:     PrizeResult{EntryName: mostGoals.Entry.Name, Amount: 5.0},
		LeastPoints:   PrizeResult{EntryName: best.Entry.Name, Amount: leastPrize},
		OverallWinner: PrizeResult{EntryName: overallWinnerName, Amount: winnerPrize},
	}
}

// findMostGoals returns the entry with the highest total goals. Ties broken
// by fewest points (best player with most goals wins), then alphabetically.
func findMostGoals(results []EntryResult) EntryResult {
	best := results[0]
	for _, r := range results[1:] {
		if r.TotalGoals > best.TotalGoals {
			best = r
		} else if r.TotalGoals == best.TotalGoals {
			if r.TotalPoints < best.TotalPoints {
				best = r
			} else if r.TotalPoints == best.TotalPoints && r.Entry.Name < best.Entry.Name {
				best = r
			}
		}
	}
	return best
}

// findOverallWinner finds the entry (or entries) that contain the team with
// FinalPlace==1 (the World Cup winner). Returns "TBD" until determined.
// If multiple entries share the champion team, returns their names joined with " / ".
func findOverallWinner(results []EntryResult, data TournamentData) string {
	var champion string
	for name, state := range data.Teams {
		if state.FinalPlace == 1 {
			champion = name
			break
		}
	}
	if champion == "" {
		return "TBD"
	}

	var winners []string
	for _, r := range results {
		for _, teamName := range r.Entry.Teams {
			if teamName == champion {
				winners = append(winners, r.Entry.Name)
				break
			}
		}
	}
	if len(winners) == 0 {
		return "TBD"
	}
	result := winners[0]
	for _, w := range winners[1:] {
		result += " / " + w
	}
	return result
}
