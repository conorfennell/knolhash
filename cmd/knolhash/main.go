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
	"github.com/conorfennell/knolhash/internal/storage"
)

func main() {
	// 1. Define and parse command-line flags
	dir := flag.String("dir", ".", "The directory to scan for markdown files")
	dbPath := flag.String("db", "knolhash.db", "Path to the SQLite database file")
	flag.Parse()

	// 2. Open the database
	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	log.Printf("Database opened successfully: %s", *dbPath)

	var cards []domain.Card
	var errors []error

	// 3. Walk the directory
	err = filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // Propagate errors from WalkDir
		}
		// For each .md file, parse it
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
	
	// Optional: Print details of errors if there are any
	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("- %s\n", e)
		}
	}
}
