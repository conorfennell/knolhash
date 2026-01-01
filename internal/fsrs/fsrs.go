package fsrs

import (
	"math"
	"time"
)

// Rating is the user's response to a card review.
type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

// Params holds the parameters for the FSRS algorithm.
// These are placeholder values and should be optimized later.
type Params struct {
	A                float64 // scales the overall memory increase
	B                float64 // difficulty exponent
	C                float64 // stability exponent
	D                float64 // retention effect scaler
	DesiredRetention float64 // desired retention rate (e.g., 0.9 for 90%)
}

// DefaultParams provides a set of sensible default parameters to start with.
func DefaultParams() *Params {
	return &Params{
		A:                0.2,
		B:                0.5,
		C:                0.1,
		D:                4.0,
		DesiredRetention: 0.9,
	}
}

// CardState holds the memory state of a card.
type CardState struct {
	Stability  float64
	Difficulty float64
	LastReview time.Time
}

// NextState calculates the next stability and difficulty based on a review.
func (p *Params) NextState(currentState CardState, rating Rating) CardState {
	if rating == Again {
		// If the user forgot, reset stability. Difficulty might increase.
		// This is a simplified handling. A full FSRS model has a more nuanced approach.
		return CardState{
			Stability:  1, // Reset to a low stability (e.g., 1 day)
			Difficulty: math.Min(10, currentState.Difficulty+0.5), // Increase difficulty, capped
			LastReview: time.Now(),
		}
	}

	// For successful reviews (Hard, Good, Easy)
	newStability := p.calculateNewStability(currentState.Stability, currentState.Difficulty)
	// Difficulty can be adjusted based on the rating, e.g., 'Hard' increases it slightly.
	newDifficulty := currentState.Difficulty
	if rating == Hard {
		newDifficulty = math.Min(10, newDifficulty+0.1)
	}

	return CardState{
		Stability:  newStability,
		Difficulty: newDifficulty,
		LastReview: time.Now(),
	}
}

// calculateNewStability applies the core FSRS formula for a successful review.
func (p *Params) calculateNewStability(stability, difficulty float64) float64 {
	// Formula: S' = S * (1 + a * D^(-b) * S^c * (e^(d * (1-R)) - 1))
	if stability < 1 {
		stability = 1 // Ensure stability is at least 1 to avoid issues with pow
	}
	if difficulty < 1 {
		difficulty = 1 // Ensure difficulty is at least 1
	}

	factor := p.A * math.Pow(difficulty, -p.B) * math.Pow(stability, p.C)
	exponent := p.D * (1 - p.DesiredRetention)
	multiplier := math.Exp(exponent) - 1

	return stability * (1 + factor*multiplier)
}

// NextDueDate calculates the next review date based on the new stability.
func NextDueDate(newStability float64) time.Time {
	// The next review is scheduled 'newStability' days from now.
	daysToAdd := time.Duration(math.Round(newStability))
	return time.Now().Add(daysToAdd * 24 * time.Hour)
}
