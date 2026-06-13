package worldcup

import "sort"

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

	seen := make(map[string]bool)
	var out []EntryResult
	for _, r := range results {
		if seen[r.Entry.Name] {
			continue
		}
		for _, teamName := range r.Entry.Teams {
			if state, ok := data.Teams[teamName]; ok && state.GroupPosition == 1 && state.FinalPlace == 0 {
				out = append(out, r)
				seen[r.Entry.Name] = true
				break
			}
		}
	}
	if len(out) > n {
		return out[:n]
	}
	return out
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

// ScoreEntry computes total points and goals for a single entry based on
// current tournament data.
func ScoreEntry(entry Entry, data TournamentData) EntryResult {
	result := EntryResult{Entry: entry}
	for i, teamName := range entry.Teams {
		state, ok := data.Teams[teamName]
		if !ok {
			state = TeamState{Name: teamName}
		}
		result.TeamStates[i] = state
		result.TotalPoints += PlacementPoints(state.FinalPlace) + ThirdPlaceGroupBonus(state.ThirdPlaceGroupRank)
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
		if a.TotalGoals != b.TotalGoals {
			return a.TotalGoals < b.TotalGoals
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
