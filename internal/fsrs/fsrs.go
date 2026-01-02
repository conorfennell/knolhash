package fsrs

import (
	"math"
	"time"
)

type Rating int

const (
	Again Rating = 1
	Hard  Rating = 2
	Good  Rating = 3
	Easy  Rating = 4
)

type Params struct {
	// W is the array of weights (simplified here for clarity)
	// In the real FSRS, there are 17-19 weights.
	W                []float64
	DesiredRetention float64
}

func DefaultParams() *Params {
	return &Params{
		// These weights are closer to the FSRS v4.5 defaults
		W:                []float64{0.4, 0.6, 2.4, 5.8, 4.93, 0.94, 0.86, 0.01, 1.49, 0.14, 0.94},
		DesiredRetention: 0.9,
	}
}

type CardState struct {
	Stability  float64
	Difficulty float64
	LastReview time.Time
}

func (p *Params) NextState(currentState CardState, rating Rating) CardState {
	if currentState.Stability == 0 {
		// First review for a new card
		// Initial stability is one of the first 4 weights (w0-w3)
		newStability := p.W[rating-1]

		// Initial difficulty is calculated based on w4
		newDifficulty := p.W[4] - p.W[6]*(float64(rating)-3)
		newDifficulty = math.Max(1, math.Min(10, newDifficulty))

		return CardState{
			Stability:  newStability,
			Difficulty: newDifficulty,
			LastReview: time.Now(),
		}
	}

	// Subsequent reviews
	var newStability float64
	var newDifficulty float64

	// 1. Calculate New Difficulty
	// Formula: D' = D - w6 * (R - 3)
	adjustment := float64(rating) - 3
	newDifficulty = currentState.Difficulty - p.W[6]*adjustment
	newDifficulty = math.Max(1, math.Min(10, newDifficulty)) // Keep between 1-10

	// 2. Calculate New Stability
	if rating == Again {
		// Stability drops significantly on failure
		newStability = currentState.Stability * p.W[7]
		newStability = math.Max(0.1, newStability)
	} else {
		newStability = p.calculateNewStability(currentState.Stability, newDifficulty, rating)
	}

	return CardState{
		Stability:  newStability,
		Difficulty: newDifficulty,
		LastReview: time.Now(),
	}
}

func (p *Params) calculateNewStability(s, d float64, r Rating) float64 {
	// The FSRS formula for success: S' = S * (1 + exp(w8) * (11 - D) * S^-w9 * (exp(w10 * (1 - R)) - 1))
	// We simplify the 'retention' part for this implementation

	hardPenalty := 1.0
	if r == Hard {
		hardPenalty = p.W[10] // Usually around 0.15 to slow growth for Hard
	} else if r == Easy {
		hardPenalty = 1.3 // Bonus for Easy
	}

	// This multiplier ensures that as Difficulty (D) goes up, the interval growth slows down
	growthFactor := math.Exp(p.W[8]) * (11 - d) * math.Pow(s, -p.W[9])

	// Apply the desired retention scaling
	// (9/DesiredRetention - 1) is a common way to scale the interval
	retentionFactor := math.Exp(p.W[10]*(1-p.DesiredRetention)) - 1

	return s * (1 + growthFactor*retentionFactor*hardPenalty)
}

func NextDueDate(newStability float64) time.Time {
	// Instead of math.Round, we use the stability as the raw day count.
	// We add a tiny bit of "fuzz" to prevent cards from grouping together perfectly.
	hours := newStability * 24
	return time.Now().Add(time.Duration(hours) * time.Hour)
}
