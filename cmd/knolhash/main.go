package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/conorfennell/knolhash/internal/domain"
	"github.com/conorfennell/knolhash/internal/parser"
)

func main() {
	// 1. Define and parse the command-line flag for the directory
	dir := flag.String("dir", ".", "The directory to scan for markdown files")
	flag.Parse()

	var cards []domain.Card
	var errors []error

	// 2. Walk the directory
	err := filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // Propagate errors from WalkDir
		}
		// 3. For each .md file, parse it
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			fileCards, parseErr := parser.ParseFile(path)
			if parseErr != nil {
				errors = append(errors, fmt.Errorf("error parsing %s: %w", path, parseErr))
			}
			if len(fileCards) > 0 {
				cards = append(cards, fileCards...)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory %s: %v", *dir, err)
	}

	// 4. Print the final report
	fmt.Printf("Found %d cards, %d errors.\n", len(cards), len(errors))
	for _, card := range cards {
		fmt.Printf("Q %s\n", card.Question)
		fmt.Printf("A %s\n", card.Answer)
		fmt.Printf("C %s\n", card.Context)
		fmt.Printf("H %s\n", card.Hash)
	}

	// Optional: Print details of errors if there are any
	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("- %s\n", e)
		}
	}
}
