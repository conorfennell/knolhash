package worldcup

import (
	"testing"
	"time"
)

func TestPlacementPoints(t *testing.T) {
	tests := []struct {
		place int
		want  int
	}{
		{0, 0},   // still active
		{1, 1},   // winner
		{2, 2},   // runner-up
		{3, 3},   // 3rd place
		{4, 4},   // 4th place
		{5, 6},   // QF loser boundary
		{8, 6},   // QF loser boundary
		{9, 12},  // R16 loser boundary
		{16, 12}, // R16 loser boundary
		{17, 25}, // R32 loser boundary
		{32, 25}, // R32 loser boundary
		{33, 33},
		{34, 34},
		{35, 35},
		{36, 36},
		{37, 42}, // group-eliminated boundary
		{40, 42},
		{48, 42}, // group-eliminated boundary
		{99, 0},  // out of range
	}
	for _, tt := range tests {
		if got := PlacementPoints(tt.place); got != tt.want {
			t.Errorf("PlacementPoints(%d) = %d, want %d", tt.place, got, tt.want)
		}
	}
}

func TestThirdPlaceGroupBonus(t *testing.T) {
	tests := []struct {
		rank int
		want int
	}{
		{0, 0},
		{1, 1},
		{5, 5},
		{8, 8},
		{9, 0},
		{-1, 0},
	}
	for _, tt := range tests {
		if got := ThirdPlaceGroupBonus(tt.rank); got != tt.want {
			t.Errorf("ThirdPlaceGroupBonus(%d) = %d, want %d", tt.rank, got, tt.want)
		}
	}
}

// TestComputePrizes verifies the 9-person example from the sweepstake rules:
// 9 people × €5 = €45 total.
// Variable pool = €45 − €15 = €30.
// Least points: €5 + 50%×€30 = €20.
// Overall winner: 50%×€30 = €15.
func TestComputePrizes(t *testing.T) {
	results := []EntryResult{
		{Entry: Entry{Name: "Best", Teams: [4]string{"A", "B", "C", "D"}}, TotalPoints: 10, TotalGoals: 5},
		{Entry: Entry{Name: "Middle", Teams: [4]string{"E", "F", "G", "H"}}, TotalPoints: 50, TotalGoals: 8},
		{Entry: Entry{Name: "Worst", Teams: [4]string{"I", "J", "K", "L"}}, TotalPoints: 100, TotalGoals: 3},
	}
	// Ensure sorted ascending (ScoreAll does this; here we do it manually).
	data := TournamentData{
		Teams:     map[string]TeamState{"A": {FinalPlace: 1}},
		FetchedAt: time.Now(),
	}

	prizes := ComputePrizes(results, data, 45)

	if prizes.MostPoints.EntryName != "Worst" {
		t.Errorf("MostPoints = %q, want %q", prizes.MostPoints.EntryName, "Worst")
	}
	if prizes.MostPoints.Amount != 5.0 {
		t.Errorf("MostPoints.Amount = %.2f, want 5.00", prizes.MostPoints.Amount)
	}
	if prizes.MostGoals.EntryName != "Middle" {
		t.Errorf("MostGoals = %q, want %q", prizes.MostGoals.EntryName, "Middle")
	}
	if prizes.LeastPoints.EntryName != "Best" {
		t.Errorf("LeastPoints = %q, want %q", prizes.LeastPoints.EntryName, "Best")
	}
	if prizes.LeastPoints.Amount != 20.0 {
		t.Errorf("LeastPoints.Amount = %.2f, want 20.00", prizes.LeastPoints.Amount)
	}
	if prizes.OverallWinner.Amount != 15.0 {
		t.Errorf("OverallWinner.Amount = %.2f, want 15.00", prizes.OverallWinner.Amount)
	}
	// "Best" entry has team "A" which has FinalPlace==1 → it's the champion's entry.
	if prizes.OverallWinner.EntryName != "Best" {
		t.Errorf("OverallWinner = %q, want %q", prizes.OverallWinner.EntryName, "Best")
	}
}

func TestScoreEntry_ThirdPlaceBonus(t *testing.T) {
	entry := Entry{
		Name:  "Test",
		Teams: [4]string{"TeamA", "TeamB", "TeamC", "TeamD"},
	}
	data := TournamentData{
		Teams: map[string]TeamState{
			"TeamA": {Name: "TeamA", FinalPlace: 1, ThirdPlaceGroupRank: 5, GoalsFor: 10},
			"TeamB": {Name: "TeamB", FinalPlace: 0, GoalsFor: 3},
			"TeamC": {Name: "TeamC", FinalPlace: 40, GoalsFor: 2},
			"TeamD": {Name: "TeamD", FinalPlace: 0, GoalsFor: 1},
		},
		FetchedAt: time.Now(),
	}
	// TeamA: place=1 → 1pt, bonus rank=5 → 5pt; total = 6 (matches example in rules)
	// TeamB: place=0 → 0pt
	// TeamC: place=40 → 42pt (group eliminated)
	// TeamD: place=0 → 0pt
	// Total points: 6 + 0 + 42 + 0 = 48
	// Total goals: 10 + 3 + 2 + 1 = 16
	result := ScoreEntry(entry, data)
	if result.TotalPoints != 48 {
		t.Errorf("TotalPoints = %d, want 48", result.TotalPoints)
	}
	if result.TotalGoals != 16 {
		t.Errorf("TotalGoals = %d, want 16", result.TotalGoals)
	}
}
