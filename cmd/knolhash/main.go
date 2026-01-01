package main

import (
	"database/sql" // Added for sql.NullTime
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/domain"
	"github.com/conorfennell/knolhash/internal/knol" // Import knol for hashing
	"github.com/conorfennell/knolhash/internal/parser"
	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/web"
)


func main() {
	// 1. Define and parse command-line flags
	dir := flag.String("dir", ".", "The directory to scan for markdown files")
	dbPath := flag.String("db", "knolhash.db", "Path to the SQLite database file")
	showDue := flag.Bool("show-due", false, "If set, show cards that are due for review and exit")
	serve := flag.Bool("serve", false, "If set, start the web server")
	listenAddr := flag.String("listen-addr", ":8080", "The address for the web server to listen on")
	flag.Parse()

	// 2. Open the database
	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 3. Decide execution mode based on flags
	if *serve {
		runWebServer(db, *listenAddr)
	} else if *showDue {
		showDueCards(db)
	} else {
		runReconciliation(db, *dir)
	}
}

// runWebServer starts the HTTP server.
func runWebServer(db *storage.DB, addr string) {
	server := web.NewServer(db)
	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}

// showDueCards fetches and prints cards that are due for review.
func showDueCards(db *storage.DB) {
	dueCards, err := db.GetDueCards()
	if err != nil {
		log.Fatalf("Failed to get due cards: %v", err)
	}
	fmt.Printf("Found %d cards due for review:\n", len(dueCards))
	for _, card := range dueCards {
		fmt.Printf("- Hash: %s, Due: %s\n", card.Hash, card.DueDate.Format(time.RFC822))
	}
}

// runReconciliation performs the file system scan and database synchronization.
func runReconciliation(db *storage.DB, dir string) {
	log.Printf("Database opened successfully. Starting reconciliation for directory: %s", dir)

	// Get or create the source entry for the scanned directory
	source, err := db.FindSourceByPath(dir)
	if err != nil {
		log.Fatalf("Failed to find source by path %s: %v", dir, err)
	}
	if source == nil {
		log.Printf("Source path %s not found, inserting new source.", dir)
		sourceID, insertErr := db.InsertSource(dir)
		if insertErr != nil {
			log.Fatalf("Failed to insert new source %s: %v", dir, insertErr)
		}
		source = &storage.Source{ID: sourceID, Path: dir, LastScanned: sql.NullTime{Time: time.Now(), Valid: true}}
	} else {
		log.Printf("Found existing source for path %s (ID: %d).", dir, source.ID)
	}

	var parsedCards []domain.Card
	var parseErrors []error
	foundCardHashes := make(map[string]bool) // To track cards found in current scan

	// Walk the directory, parse files, and reconcile cards
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // Propagate errors from WalkDir
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			fileCards, parseErr := parser.ParseFile(path)
			if parseErr != nil {
				parseErrors = append(parseErrors, fmt.Errorf("error parsing %s: %w", path, parseErr))
			}

			for _, card := range fileCards {
				card.Hash = knol.Hash(card) // Calculate hash for the card
				parsedCards = append(parsedCards, card)
				foundCardHashes[card.Hash] = true

				// Check if card exists in DB
				cardState, findErr := db.FindCardStateByHash(card.Hash)
				if findErr != nil {
					parseErrors = append(parseErrors, fmt.Errorf("error checking card %s in DB: %w", card.Hash, findErr))
					continue
				}

				if cardState == nil {
					// Card not in DB, insert it
					log.Printf("New card found: %s, inserting into DB.", card.Hash)
					insertErr := db.InsertCard(card, source.ID)
					if insertErr != nil {
						parseErrors = append(parseErrors, fmt.Errorf("error inserting card %s: %w", card.Hash, insertErr))
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory %s: %v", dir, err)
	}

	// Identify orphaned cards
	dbCards, err := db.GetCardsBySourceID(source.ID)
	if err != nil {
		log.Fatalf("Failed to get cards for source ID %d from DB: %v", source.ID, err)
	}

	var orphanedCards int
	for _, dbCard := range dbCards {
		if _, found := foundCardHashes[dbCard.Hash]; !found {
			log.Printf("Orphaned card detected, deleting: Hash %s (Question: %s)", dbCard.Hash, dbCard.Question)
			orphanedCards++
			if err := db.DeleteCardByHash(dbCard.Hash); err != nil {
				log.Printf("Warning: Failed to delete orphaned card %s: %v", dbCard.Hash, err)
			}
		}
	}

	// Update source's last scanned timestamp
	if err := db.UpdateSourceLastScanned(source.ID); err != nil {
		log.Printf("Warning: Failed to update last scanned timestamp for source %d: %v", source.ID, err)
	}

	// Print the final report
	fmt.Printf("Reconciliation complete. Found %d cards in files. %d orphaned cards deleted. %d errors.\n",
		len(parsedCards), orphanedCards, len(parseErrors))

	if len(parseErrors) > 0 {
		fmt.Println("\nErrors during parsing or reconciliation:")
		for _, e := range parseErrors {
			fmt.Printf("- %s\n", e)
		}
	}
}

