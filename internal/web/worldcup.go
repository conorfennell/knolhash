package web

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/worldcup"
)

type entryViewData struct {
	Entry           worldcup.Entry
	Result          worldcup.EntryResult
	Rank            int
	TotalEntries    int
	AllResults      []worldcup.EntryResult
	Prizes          worldcup.PrizeSummary
	TeamMatches     [4][]worldcup.Match
	Teams           map[string]worldcup.TeamState
	FetchedAt       time.Time
	NextRefreshUnix int64
	TotalPot        int
	LeastPtsRank    int
	LeastPtsGap     int
	MostPtsRank     int
	MostPtsGap      int
	MostGoalsRank   int
	MostGoalsGap    int
}

type worldcupTemplateData struct {
	Results         []worldcup.EntryResult
	TopBest         []worldcup.EntryResult
	TopWorst        []worldcup.EntryResult
	TopGoals        []worldcup.EntryResult
	TopLeaders      []worldcup.EntryResult
	Teams           map[string]worldcup.TeamState
	Entries         []worldcup.Entry
	Prizes          worldcup.PrizeSummary
	FetchedAt       time.Time
	TotalPot        int
	RecentMatches   []worldcup.Match
	UpcomingMatches []worldcup.Match
	History         []worldcup.MatchSnapshot
	EntryNames      []string
	NextRefreshUnix int64
	DangerEntries   []worldcup.EntryResult
}

func (s *Server) handleGetWorldcup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := s.worldcupCache.Get()
		results := worldcup.ScoreAll(worldcup.Entries, data)
		prizes := worldcup.ComputePrizes(results, data, worldcup.TotalPot)

		entryNames := make([]string, len(results))
		for i, r := range results {
			entryNames[i] = r.Entry.Name
		}

		var dangerEntries []worldcup.EntryResult
		for _, r := range results {
			for _, state := range r.TeamStates {
				if state.GroupPosition == 4 && state.FinalPlace == 0 && state.Played > 0 {
					dangerEntries = append(dangerEntries, r)
					break
				}
			}
		}

		td := worldcupTemplateData{
			Results:         results,
			TopBest:         results,
			TopWorst:        worldcup.TopByMostPoints(results, len(results)),
			TopGoals:        worldcup.TopByMostGoals(results, len(results)),
			TopLeaders:      worldcup.TopOverallWinner(results, data, len(results)),
			Teams:           data.Teams,
			Entries:         worldcup.Entries,
			Prizes:          prizes,
			FetchedAt:       data.FetchedAt,
			TotalPot:        worldcup.TotalPot,
			RecentMatches:   worldcup.RecentMatches(data.Matches, 10),
			UpcomingMatches: worldcup.UpcomingMatches(data.Matches, 10),
			History:         worldcup.ComputeHistory(data.Matches, worldcup.Entries, data),
			EntryNames:      entryNames,
			NextRefreshUnix: data.FetchedAt.Add(15 * time.Minute).Unix(),
			DangerEntries:   dangerEntries,
		}
		if err := s.templates.ExecuteTemplate(w, "worldcup", td); err != nil {
			slog.Error("worldcup: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func entrySlug(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

func (s *Server) handleGetWorldcupEntry() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/worldcup/entry/")
		slug = strings.Trim(slug, "/")

		var entry worldcup.Entry
		found := false
		for _, e := range worldcup.Entries {
			if entrySlug(e.Name) == slug {
				entry = e
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}

		data := s.worldcupCache.Get()
		results := worldcup.ScoreAll(worldcup.Entries, data)
		prizes := worldcup.ComputePrizes(results, data, worldcup.TotalPot)

		// Find this entry's result and rank.
		var result worldcup.EntryResult
		rank := 1
		for i, res := range results {
			if res.Entry.Name == entry.Name {
				result = res
				rank = i + 1
				break
			}
		}

		// Per-team upcoming fixtures (next 3 each), already sorted chronologically.
		upcoming := worldcup.UpcomingMatches(data.Matches, len(data.Matches))
		var teamMatches [4][]worldcup.Match
		for j, teamName := range entry.Teams {
			for _, m := range upcoming {
				if m.HomeTeam == teamName || m.AwayTeam == teamName {
					teamMatches[j] = append(teamMatches[j], m)
					if len(teamMatches[j]) >= 3 {
						break
					}
				}
			}
		}

		// Prize rank + gap for Least Points (lower = better; results sorted ascending).
		leastPtsRank := rank
		leastPtsGap := result.TotalPoints - results[0].TotalPoints

		// Prize rank + gap for Most Points (higher = worse; results sorted ascending so last = worst).
		worstPts := results[len(results)-1].TotalPoints
		mostPtsRank := len(results) - rank + 1
		mostPtsGap := worstPts - result.TotalPoints

		// Prize rank + gap for Most Goals.
		goalsSorted := make([]worldcup.EntryResult, len(results))
		copy(goalsSorted, results)
		sort.Slice(goalsSorted, func(i, j int) bool {
			return goalsSorted[i].TotalGoals > goalsSorted[j].TotalGoals
		})
		mostGoalsRank := 1
		for i, res := range goalsSorted {
			if res.Entry.Name == entry.Name {
				mostGoalsRank = i + 1
				break
			}
		}
		mostGoalsGap := goalsSorted[0].TotalGoals - result.TotalGoals

		td := entryViewData{
			Entry:           entry,
			Result:          result,
			Rank:            rank,
			TotalEntries:    len(results),
			AllResults:      results,
			Prizes:          prizes,
			TeamMatches:     teamMatches,
			Teams:           data.Teams,
			FetchedAt:       data.FetchedAt,
			NextRefreshUnix: data.FetchedAt.Add(15 * time.Minute).Unix(),
			TotalPot:        worldcup.TotalPot,
			LeastPtsRank:    leastPtsRank,
			LeastPtsGap:     leastPtsGap,
			MostPtsRank:     mostPtsRank,
			MostPtsGap:      mostPtsGap,
			MostGoalsRank:   mostGoalsRank,
			MostGoalsGap:    mostGoalsGap,
		}
		if err := s.templates.ExecuteTemplate(w, "worldcup_entry", td); err != nil {
			slog.Error("worldcup entry: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
