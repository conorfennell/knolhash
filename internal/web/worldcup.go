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
	Entry            worldcup.Entry
	Result           worldcup.EntryResult
	Rank             int
	TotalEntries     int
	AllResults       []worldcup.EntryResult
	Prizes           worldcup.PrizeSummary
	CombinedUpcoming []worldcup.Match
	Teams            map[string]worldcup.TeamState
	LiveMatches      []worldcup.LiveMatch
	FetchedAt        time.Time
	NextRefreshUnix  int64
	TotalPot         int
	LeastPtsRank     int
	LeastPtsGap      int
	MostPtsRank      int
	MostPtsGap       int
	MostGoalsRank    int
	MostGoalsGap     int
}

type worldcupTemplateData struct {
	Results         []worldcup.EntryResult
	TopBest         []worldcup.EntryResult
	TopWorst        []worldcup.EntryResult
	TopGoals        []worldcup.EntryResult
	TopLeaders      []worldcup.EntryResult
	GroupLeaders    []worldcup.GroupLeader
	NoLeaderEntries []string
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
	LiveMatches     []worldcup.LiveMatch
	SideBets        []worldcup.ResolvedSideBet
	EntryIcons      map[string]string
	EntryClasses    map[string]string
	Commit          string
	BuildDate       string
}

// applyLiveGoals overlays current in-play goals onto a copy of TournamentData
// so the leaderboard moves in real time during a match.
// Skips any match that Wikipedia has already marked as played — those goals
// are already baked into TeamState.GoalsFor by the scraper.
func applyLiveGoals(data worldcup.TournamentData, lives []worldcup.LiveMatch) worldcup.TournamentData {
	if len(lives) == 0 {
		return data
	}
	completed := make(map[string]bool, len(data.Matches))
	for _, m := range data.Matches {
		if m.Played {
			completed[m.HomeTeam+"|||"+m.AwayTeam] = true
		}
	}
	merged := make(map[string]worldcup.TeamState, len(data.Teams))
	for k, v := range data.Teams {
		merged[k] = v
	}
	for _, lm := range lives {
		if completed[lm.HomeTeam+"|||"+lm.AwayTeam] {
			continue
		}
		if s, ok := merged[lm.HomeTeam]; ok {
			s.GoalsFor += lm.HomeScore
			merged[lm.HomeTeam] = s
		}
		if s, ok := merged[lm.AwayTeam]; ok {
			s.GoalsFor += lm.AwayScore
			merged[lm.AwayTeam] = s
		}
	}
	data.Teams = merged
	return data
}

func (s *Server) handleGetWorldcup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := s.worldcupCache.Get()
		lives := s.liveCache.Get()
		data = applyLiveGoals(data, lives)
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

		groupLeaders, noLeaderEntries := worldcup.ComputeGroupLeaders(results, data)
		td := worldcupTemplateData{
			Results:         results,
			TopBest:         results,
			TopWorst:        worldcup.TopByMostPoints(results, len(results)),
			TopGoals:        worldcup.TopByMostGoals(results, len(results)),
			TopLeaders:      worldcup.TopOverallWinner(results, data, len(results)),
			GroupLeaders:    groupLeaders,
			NoLeaderEntries: noLeaderEntries,
			Teams:           data.Teams,
			Entries:         worldcup.Entries,
			Prizes:          prizes,
			FetchedAt:       data.FetchedAt,
			TotalPot:        worldcup.TotalPot,
			RecentMatches:   worldcup.RecentMatches(data.Matches, 3),
			UpcomingMatches: worldcup.UpcomingMatches(data.Matches, 4),
			History:         worldcup.ComputeHistory(data.Matches, worldcup.Entries, data),
			EntryNames:      entryNames,
			NextRefreshUnix: data.FetchedAt.Add(1 * time.Minute).Unix(),
			DangerEntries:   dangerEntries,
			LiveMatches:     lives,
			SideBets:        worldcup.ResolveSideBets(worldcup.SideBets, data.Matches),
			EntryIcons:      buildEntryIcons(prizes),
			EntryClasses:    buildEntryClasses(prizes),
			Commit:          s.commit,
			BuildDate:       s.buildDate,
		}
		if err := s.templates.ExecuteTemplate(w, "worldcup", td); err != nil {
			slog.Error("worldcup: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

func buildEntryClasses(prizes worldcup.PrizeSummary) map[string]string {
	classes := map[string]string{}
	if n := prizes.LeastPoints.EntryName; n != "" && n != "TBD" {
		classes[n] = "leader-best"
	}
	if n := prizes.MostGoals.EntryName; n != "" && n != "TBD" {
		if classes[n] == "" {
			classes[n] = "leader-goals"
		}
	}
	if n := prizes.MostPoints.EntryName; n != "" && n != "TBD" {
		if classes[n] == "" {
			classes[n] = "leader-worst"
		}
	}
	return classes
}

func buildEntryIcons(prizes worldcup.PrizeSummary) map[string]string {
	icons := map[string]string{}
	if n := prizes.LeastPoints.EntryName; n != "" && n != "TBD" {
		icons[n] += " 👑"
	}
	if n := prizes.MostGoals.EntryName; n != "" && n != "TBD" {
		icons[n] += " ⚽"
	}
	if n := prizes.MostPoints.EntryName; n != "" && n != "TBD" {
		icons[n] += " 🥄"
	}
	return icons
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
		lives := s.liveCache.Get()
		data = applyLiveGoals(data, lives)
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

		// All upcoming fixtures for any of this entry's teams, merged and deduped.
		upcoming := worldcup.UpcomingMatches(data.Matches, len(data.Matches))
		entryTeams := make(map[string]bool)
		for _, t := range entry.Teams {
			entryTeams[t] = true
		}
		seen := make(map[string]bool)
		var combinedUpcoming []worldcup.Match
		for _, m := range upcoming {
			if !entryTeams[m.HomeTeam] && !entryTeams[m.AwayTeam] {
				continue
			}
			key := m.HomeTeam + "|||" + m.AwayTeam
			if seen[key] {
				continue
			}
			seen[key] = true
			combinedUpcoming = append(combinedUpcoming, m)
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
			Entry:            entry,
			Result:           result,
			Rank:             rank,
			TotalEntries:     len(results),
			AllResults:       results,
			Prizes:           prizes,
			CombinedUpcoming: combinedUpcoming,
			Teams:            data.Teams,
			LiveMatches:      lives,
			FetchedAt:        data.FetchedAt,
			NextRefreshUnix:  data.FetchedAt.Add(1 * time.Minute).Unix(),
			TotalPot:         worldcup.TotalPot,
			LeastPtsRank:     leastPtsRank,
			LeastPtsGap:      leastPtsGap,
			MostPtsRank:      mostPtsRank,
			MostPtsGap:       mostPtsGap,
			MostGoalsRank:    mostGoalsRank,
			MostGoalsGap:     mostGoalsGap,
		}
		if err := s.templates.ExecuteTemplate(w, "worldcup_entry", td); err != nil {
			slog.Error("worldcup entry: template error", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
