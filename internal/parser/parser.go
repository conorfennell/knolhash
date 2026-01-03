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
		if len(currentBlock) > 0 {
			content := strings.Join(currentBlock, "\n")
			switch currentState {
			case readingQuestion:
				currentCard.Question = content
			case readingAnswer:
				currentCard.Answer = content
			case readingContext:
				currentCard.Context = content
			}
			currentBlock = nil
		}

		if currentCard.Question != "" {
			cards = append(cards, currentCard)
		}
		currentCard = domain.Card{}
		currentState = seeking
	}

	for scanner.Scan() {
		line := scanner.Text()

		isQ := strings.HasPrefix(line, questionPrefix)
		isA := strings.HasPrefix(line, answerPrefix)
		isC := strings.HasPrefix(line, contextPrefix)
		isSeparator := line == "---"

		if isSeparator {
			finishCard()
			continue
		}

		if isQ || isA || isC {
			if len(currentBlock) > 0 {
				content := strings.Join(currentBlock, "\n")
				switch currentState {
				case readingQuestion:
					currentCard.Question = content
				case readingAnswer:
					currentCard.Answer = content
				case readingContext:
					currentCard.Context = content
				}
				currentBlock = nil
			}

			if isQ {
				if currentState != seeking { // A new question always starts a new card
					finishCard()
				}
				currentState = readingQuestion
				lineContent := line[len(questionPrefix):]
				if strings.HasPrefix(lineContent, " ") {
					lineContent = lineContent[1:]
				}
				currentBlock = append(currentBlock, lineContent)
			} else if isA {
				currentState = readingAnswer
				lineContent := line[len(answerPrefix):]
				if strings.HasPrefix(lineContent, " ") {
					lineContent = lineContent[1:]
				}
				currentBlock = append(currentBlock, lineContent)
			} else if isC {
				currentState = readingContext
				lineContent := line[len(contextPrefix):]
				if strings.HasPrefix(lineContent, " ") {
					lineContent = lineContent[1:]
				}
				currentBlock = append(currentBlock, lineContent)
			}
		} else if currentState != seeking {
			currentBlock = append(currentBlock, line)
		}
	}

	finishCard() // Finish the very last card in the file

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cards, nil
}
