package knol

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/conorfennell/knolhash/internal/domain"
)

// Normalize concatenates the card's content after cleaning each part.
// It trims whitespace, lowercases, and normalizes line endings for each field
// before joining them.
func Normalize(card domain.Card) string {
	normalizePart := func(part string) string {
		p := strings.ToLower(part)
		p = strings.TrimSpace(p)
		p = strings.ReplaceAll(p, "\r\n", "\n")
		return p
	}

	q := normalizePart(card.Question)
	a := normalizePart(card.Answer)
	c := normalizePart(card.Context)

	// We join with a newline to ensure separation between fields,
	// preventing accidental joining of words. e.g. "question" and "answer"
	// becoming "questionanswer".
	return strings.Join([]string{q, a, c}, "\n")
}

// Hash takes a card, normalizes it, and returns its SHA-256 hash as a hex string.
func Hash(card domain.Card) string {
	normalized := Normalize(card)
	hashBytes := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", hashBytes)
}
