package knol

import (
	"testing"

	"github.com/conorfennell/knolhash/internal/domain"
)

func TestNormalize(t *testing.T) {
	card := domain.Card{
		Question: "  What is HTMX? \r\n",
		Answer:   "A library for AJAX.",
		Context:  "Web Development",
	}
	expected := "what is htmx?\na library for ajax.\nweb development"
	normalized := Normalize(card)

	if normalized != expected {
		t.Errorf("Expected normalized string to be '%s', but got '%s'", expected, normalized)
	}
}

func TestHash(t *testing.T) {
	t.Run("generates correct hash", func(t *testing.T) {
		card := domain.Card{
			Question: "Q",
			Answer:   "A",
			Context:  "C",
		}
		// Hash for "q\na\nc"
		expectedHash := "eb2456c1ee4f36305069dd0f63a30e92d5443129f5e8fd9a5ec490fbc4d4d8a2"
		hash := Hash(card)

		if hash != expectedHash {
			t.Errorf("Expected hash '%s', but got '%s'", expectedHash, hash)
		}
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		card1 := domain.Card{Question: "Test"}
		card2 := domain.Card{Question: "Test"}
		if Hash(card1) != Hash(card2) {
			t.Error("Expected hashes for identical cards to be the same")
		}
	})

	t.Run("normalization produces same hash", func(t *testing.T) {
		card1 := domain.Card{
			Question: "  what is go? ",
			Answer:   "A programming language.",
		}
		card2 := domain.Card{
			Question: "What Is Go?",
			Answer:   "A programming language.",
		}
		if Hash(card1) != Hash(card2) {
			t.Error("Expected hashes to be the same after normalization, but they were different.")
		}
	})

	t.Run("different cards have different hashes", func(t *testing.T) {
		card1 := domain.Card{Question: "Card 1"}
		card2 := domain.Card{Question: "Card 2"}
		if Hash(card1) == Hash(card2) {
			t.Error("Expected hashes for different cards to be different")
		}
	})
}
