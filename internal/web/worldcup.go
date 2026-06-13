package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/conorfennell/knolhash/internal/worldcup"
)

type worldcupTemplateData struct {
	Results    []worldcup.EntryResult
	TopBest    []worldcup.EntryResult // top 5 fewest points (Least Points prize)
	TopWorst   []worldcup.EntryResult // top 5 most points (Most Points prize)
	TopGoals   []worldcup.EntryResult // top 5 most goals (Most Goals prize)
	TopLeaders []worldcup.EntryResult // top 5 for Overall Winner prize
	Teams      map[string]worldcup.TeamState
	Prizes     worldcup.PrizeSummary
	FetchedAt  time.Time
	TotalPot   int
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

		td := worldcupTemplateData{
			Results:    results,
			TopBest:    results[:topN],
			TopWorst:   worldcup.TopByMostPoints(results, 5),
			TopGoals:   worldcup.TopByMostGoals(results, 5),
			TopLeaders: worldcup.TopOverallWinner(results, data, 5),
			Teams:      data.Teams,
			Prizes:     prizes,
			FetchedAt:  data.FetchedAt,
			TotalPot:   worldcup.TotalPot,
		}
		if err := s.templates.ExecuteTemplate(w, "worldcup", td); err != nil {
			slog.Error("worldcup: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
