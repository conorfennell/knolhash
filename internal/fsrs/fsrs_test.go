package fsrs

import (
	"math"
	"testing"
	"time"
)

func TestCalculateNewStability(t *testing.T) {
	params := DefaultParams()
	stability := 10.0
	difficulty := 5.0
	
	// S' = 10 * (1 + 0.2 * 5^(-0.5) * 10^0.1 * (e^(4 * (1-0.9)) - 1))
	// S' = 10 * (1 + 0.2 * 0.447 * 1.259 * (e^0.4 - 1))
	// S' = 10 * (1 + 0.112 * (1.4918 - 1))
	// S' = 10 * (1 + 0.112 * 0.4918)
	// S' = 10 * (1 + 0.055)
	// S' = 10 * 1.055 = 10.55
	expected := 10.55
	
	newStability := params.calculateNewStability(stability, difficulty)
	
	if math.Abs(newStability-expected) > 0.01 {
		t.Errorf("Expected new stability to be around %.2f, but got %.2f", expected, newStability)
	}
}

func TestNextState(t *testing.T) {
	params := DefaultParams()
	initialState := CardState{
		Stability:  10,
		Difficulty: 5,
		LastReview: time.Now(),
	}

	t.Run("Review with Again", func(t *testing.T) {
		newState := params.NextState(initialState, Again)
		if newState.Stability != 1 {
			t.Errorf("Expected stability to be reset to 1, but got %.2f", newState.Stability)
		}
		if newState.Difficulty <= initialState.Difficulty {
			t.Errorf("Expected difficulty to increase, but it did not. Got %.2f", newState.Difficulty)
		}
	})

	t.Run("Review with Good", func(t *testing.T) {
		newState := params.NextState(initialState, Good)
		if newState.Stability <= initialState.Stability {
			t.Errorf("Expected stability to increase, but it did not. Got %.2f", newState.Stability)
		}
		if newState.Difficulty != initialState.Difficulty {
			t.Errorf("Expected difficulty to remain the same for 'Good', but it changed to %.2f", newState.Difficulty)
		}
	})

	t.Run("Review with Hard", func(t *testing.T) {
		newState := params.NextState(initialState, Hard)
		if newState.Stability <= initialState.Stability {
			t.Errorf("Expected stability to increase, but it did not. Got %.2f", newState.Stability)
		}
		if newState.Difficulty <= initialState.Difficulty {
			t.Errorf("Expected difficulty to increase for 'Hard', but it did not. Got %.2f", newState.Difficulty)
		}
	})
}

func TestNextDueDate(t *testing.T) {
	now := time.Now()
	stability := 15.5 // Should round to 16 days
	
	expectedDate := now.Add(16 * 24 * time.Hour)
	actualDate := NextDueDate(stability)

	// Check if the dates are on the same day (ignoring time-of-day differences)
	if actualDate.Year() != expectedDate.Year() || actualDate.YearDay() != expectedDate.YearDay() {
		t.Errorf("Expected due date to be around %v, but got %v", expectedDate, actualDate)
	}
}
