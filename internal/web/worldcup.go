package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/conorfennell/knolhash/internal/worldcup"
)

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

		topN := 5
		if topN > len(results) {
			topN = len(results)
		}

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
			TopBest:         results[:topN],
			TopWorst:        worldcup.TopByMostPoints(results, 5),
			TopGoals:        worldcup.TopByMostGoals(results, 5),
			TopLeaders:      worldcup.TopOverallWinner(results, data, 5),
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
