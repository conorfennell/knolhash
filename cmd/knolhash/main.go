package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/sync"
	"github.com/conorfennell/knolhash/internal/web"
)

func main() {
	// 1. Configure Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Define flags
	dbPath := flag.String("db", "knolhash.db", "Path to the SQLite database file")
	addSource := flag.String("add-source", "", "The path or Git URL of a source to add")
	showDue := flag.Bool("show-due", false, "Show cards that are due for review and exit")
	serve := flag.Bool("serve", false, "Start the web server")
	listenAddr := flag.String("listen-addr", ":8080", "The address for the web server to listen on")
	syncInterval := flag.Duration("sync-interval", 30*time.Minute, "Interval for background sync when in server mode")
	flag.Parse()

	// 3. Open DB
	db, err := storage.Open(*dbPath)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Dispatch based on flags
	if *addSource != "" {
		if err := addNewSource(db, *addSource); err != nil {
			slog.Error("Failed to add source", "error", err)
			os.Exit(1)
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
		slog.Info("Source with path already exists", "path", path)
		return nil
	}

	_, err = db.InsertSource(path, sourceType)
	if err != nil {
		return fmt.Errorf("could not insert new source: %w", err)
	}
	slog.Info("Successfully added new source", "path", path, "type", sourceType)
	return nil
}

// runWebServer starts the HTTP server and a background sync ticker.
func runWebServer(db *storage.DB, addr string, syncInterval time.Duration) {
	startBackgroundSync(db, syncInterval)

	server := web.NewServer(db)
	slog.Info("Starting web server", "addr", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		slog.Error("Failed to start web server", "error", err)
		os.Exit(1)
	}
}

// startBackgroundSync starts a goroutine that periodically calls sync.RunSync.
func startBackgroundSync(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			slog.Info("Background sync triggered", "interval", interval)
			sync.RunSync(db)
		}
	}()
	slog.Info("Background sync started", "interval", interval)
}

// showDueCards fetches and prints cards that are due for review.
func showDueCards(db *storage.DB) {
	dueCards, err := db.GetDueCards()
	if err != nil {
		slog.Error("Failed to get due cards", "error", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d cards due for review:\n", len(dueCards))
	for _, card := range dueCards {
		fmt.Printf("- Hash: %s, Due: %s\n", card.Hash, card.DueDate.Format(time.RFC822))
	}
}


