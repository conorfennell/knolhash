package parser

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/conorfennell/knolhash/internal/domain"
)

const (
	questionPrefix = "Q:"
	answerPrefix   = "A:"
	contextPrefix  = "C:"
)

type state int

const (
	seeking state = iota
	readingQuestion
	readingAnswer
	readingContext
)

// ParseFile reads a file from the given path and extracts all cards.
func ParseFile(path string) ([]domain.Card, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads from an io.Reader and extracts all cards.
func Parse(r io.Reader) ([]domain.Card, error) {
	scanner := bufio.NewScanner(r)
	var cards []domain.Card
	var currentCard domain.Card
	var currentBlock []string
	currentState := seeking

	finishCard := func() {
		if currentCard.Question != "" {
			cards = append(cards, currentCard)
		}
		currentCard = domain.Card{}
	}

	for scanner.Scan() {
		line := scanner.Text()

		isQ := strings.HasPrefix(line, questionPrefix)
		isA := strings.HasPrefix(line, answerPrefix)
		isC := strings.HasPrefix(line, contextPrefix)

		if isQ || isA || isC {
			// When a new prefix is found, save the previous block's content
			switch currentState {
			case readingQuestion:
				currentCard.Question = strings.TrimSpace(strings.Join(currentBlock, "\n"))
			case readingAnswer:
				currentCard.Answer = strings.TrimSpace(strings.Join(currentBlock, "\n"))
			case readingContext:
				currentCard.Context = strings.TrimSpace(strings.Join(currentBlock, "\n"))
			}
			currentBlock = nil // Reset for the new block

			if isQ {
				finishCard() // A new question starts a new card
				currentState = readingQuestion
				currentBlock = append(currentBlock, strings.TrimSpace(strings.TrimPrefix(line, questionPrefix)))
			} else if isA {
				currentState = readingAnswer
				currentBlock = append(currentBlock, strings.TrimSpace(strings.TrimPrefix(line, answerPrefix)))
			} else if isC {
				currentState = readingContext
				currentBlock = append(currentBlock, strings.TrimSpace(strings.TrimPrefix(line, contextPrefix)))
			}
		} else if currentState != seeking {
			// Continue reading multi-line content
			currentBlock = append(currentBlock, line)
		}
	}

	// Save the last block of the last card
	switch currentState {
	case readingQuestion:
		currentCard.Question = strings.TrimSpace(strings.Join(currentBlock, "\n"))
	case readingAnswer:
		currentCard.Answer = strings.TrimSpace(strings.Join(currentBlock, "\n"))
	case readingContext:
		currentCard.Context = strings.TrimSpace(strings.Join(currentBlock, "\n"))
	}
	finishCard() // Finish the very last card in the file

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cards, nil
}
