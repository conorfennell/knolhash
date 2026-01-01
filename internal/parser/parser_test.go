package parser

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	testCases := []struct {
		name          string
		input         string
		expectedCards int
		expectedQ     string
		expectedA     string
		expectedC     string
	}{
		{
			name:          "Simple Q&A",
			input:         "Q: What is the capital of France?\nA: Paris",
			expectedCards: 1,
			expectedQ:     "What is the capital of France?",
			expectedA:     "Paris",
			expectedC:     "",
		},
		{
			name:          "Simple Q, A, and C",
			input:         "Q: What is 1+1?\nA: 2\nC: Basic arithmetic",
			expectedCards: 1,
			expectedQ:     "What is 1+1?",
			expectedA:     "2",
			expectedC:     "Basic arithmetic",
		},
		{
			name: "Multiline Answer",
			input: `
Q: What are the primary colors?
A: Red
Blue
Yellow
`,
			expectedCards: 1,
			expectedQ:     "What are the primary colors?",
			expectedA:     "Red\nBlue\nYellow",
			expectedC:     "",
		},
		{
			name: "Two Cards",
			input: `
Q: First question
A: First answer

Q: Second question
A: Second answer
`,
			expectedCards: 2,
		},
		{
			name: "Card with all fields and multiline",
			input: `
Q: What is Go?
A: A statically typed, compiled programming language.
It was designed at Google.
C: Programming Languages
`,
			expectedCards: 1,
			expectedQ:     "What is Go?",
			expectedA:     "A statically typed, compiled programming language.\nIt was designed at Google.",
			expectedC:     "Programming Languages",
		},
		{
			name: "No cards, just text",
			input: "This is a file with no questions.",
			expectedCards: 0,
		},
        {
            name: "Prefixes with no space",
            input: "Q:Question\nA:Answer",
            expectedCards: 1,
            expectedQ: "Question",
            expectedA: "Answer",
        },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := strings.NewReader(tc.input)
			cards, err := Parse(r)
			if err != nil {
				t.Fatalf("Parse() returned an unexpected error: %v", err)
			}

			if len(cards) != tc.expectedCards {
				t.Fatalf("Expected %d cards, but got %d", tc.expectedCards, len(cards))
			}

			if tc.expectedCards == 1 {
				card := cards[0]
				if card.Question != tc.expectedQ {
					t.Errorf("Expected Question to be '%s', but got '%s'", tc.expectedQ, card.Question)
				}
				if card.Answer != tc.expectedA {
					t.Errorf("Expected Answer to be '%s', but got '%s'", tc.expectedA, card.Answer)
				}
				if card.Context != tc.expectedC {
					t.Errorf("Expected Context to be '%s', but got '%s'", tc.expectedC, card.Context)
				}
			}
		})
	}
}
