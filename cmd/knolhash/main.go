package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/sync"
	"github.com/conorfennell/knolhash/internal/web"
)

func main() {
	// 1. Define flags
	dbPath := flag.String("db", "knolhash.db", "Path to the SQLite database file")
	addSource := flag.String("add-source", "", "The path or Git URL of a source to add")
	showDue := flag.Bool("show-due", false, "Show cards that are due for review and exit")
	serve := flag.Bool("serve", false, "Start the web server")
	listenAddr := flag.String("listen-addr", ":8080", "The address for the web server to listen on")
	syncInterval := flag.Duration("sync-interval", 30*time.Minute, "Interval for background sync when in server mode")
	flag.Parse()

	// 2. Open DB
	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 3. Dispatch based on flags
	if *addSource != "" {
		if err := addNewSource(db, *addSource); err != nil {
			log.Fatalf("Failed to add source: %v", err)
		}
		return
	}
	if *serve {
		runWebServer(db, *listenAddr, *syncInterval)
		return
	}
	if *showDue {
		showDueCards(db)
		return
	}
	
	// Default action is to sync
	sync.RunSync(db)
}

// addNewSource adds a new source to the database, determining its type.
func addNewSource(db *storage.DB, path string) error {
	// This logic could be moved to a shared package if it gets more complex
	sourceType := "local"
	if strings.HasSuffix(path, ".git") || strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "https://") {
		sourceType = "git"
	}
	
	existing, err := db.FindSourceByPath(path)
	if err != nil {
		return fmt.Errorf("error checking for existing source: %w", err)
	}
	if existing != nil {
		log.Printf("Source with path '%s' already exists.", path)
		return nil
	}

	_, err = db.InsertSource(path, sourceType)
	if err != nil {
		return fmt.Errorf("could not insert new source: %w", err)
	}
	log.Printf("Successfully added new source: %s (type: %s)", path, sourceType)
	return nil
}

// runWebServer starts the HTTP server and a background sync ticker.
func runWebServer(db *storage.DB, addr string, syncInterval time.Duration) {
	startBackgroundSync(db, syncInterval)

	server := web.NewServer(db)
	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}

// startBackgroundSync starts a goroutine that periodically calls sync.RunSync.
func startBackgroundSync(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Printf("Background sync triggered (interval: %v)...", interval)
			sync.RunSync(db)
		}
	}()
	log.Printf("Background sync started, running every %v", interval)
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


