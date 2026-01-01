package domain

import "time"

// Card represents a single question-answer-context entry.
type Card struct {
	Question string
	Answer   string
	Context  string
	Hash     string
}

// ReviewLog records a single review event for a card.
// The Grade corresponds to FSRS-4.5 ratings:
// 1: Again (Incorrect)
// 2: Hard
// 3: Good
// 4: Easy
type ReviewLog struct {
	CardHash  string
	Timestamp time.Time
	Grade     int
}
