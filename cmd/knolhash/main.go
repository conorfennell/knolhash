package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/domain"
	"github.com/conorfennell/knolhash/internal/gitsource"
	"github.com/conorfennell/knolhash/internal/knol"
	"github.com/conorfennell/knolhash/internal/parser"
	"github.com/conorfennell/knolhash/internal/storage"
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
	runSync(db)
}

// addNewSource adds a new source to the database, determining its type.
func addNewSource(db *storage.DB, path string) error {
	sourceType := "local"
	if strings.HasSuffix(path, ".git") || strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "https://") {
		sourceType = "git"
	}
	if sourceType == "local" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("local path does not exist: %s", path)
		}
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

// runWebServer starts the HTTP server.
func runWebServer(db *storage.DB, addr string, syncInterval time.Duration) {
	// Start background sync
	startBackgroundSync(db, syncInterval)

	server := web.NewServer(db)
	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}
}

// startBackgroundSync starts a goroutine that periodically calls runSync.
func startBackgroundSync(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Printf("Background sync triggered (interval: %v)...", interval)
			runSync(db) // Call the main sync logic
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

// runSync iterates over all sources and reconciles them.
func runSync(db *storage.DB) {
	log.Println("Starting sync process for all sources...")
	sources, err := db.GetAllSources()
	if err != nil {
		log.Fatalf("Failed to get sources: %v", err)
	}

	if len(sources) == 0 {
		log.Println("No sources configured. Add one with --add-source <path/or/url.git>")
		return
	}

	// Create a directory to store cloned repos
	reposDir := "repos"
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create repos directory: %v", err)
	}

	for _, source := range sources {
		log.Printf("Syncing source ID %d (Type: %s, Path: %s)", source.ID, source.Type, source.Path)
		
		sourceToReconcile := source // Make a copy to avoid modifying the loop variable
		
		if source.Type == "local" {
			reconcileLocalSource(db, &sourceToReconcile)
		} else if source.Type == "git" {
			localRepoPath, err := gitUrlToLocalPath(reposDir, source.Path)
			if err != nil {
				log.Printf("Error determining local path for git repo %s: %v", source.Path, err)
				continue
			}
			
			if err := gitsource.Sync(source.Path, localRepoPath); err != nil {
				log.Printf("Error syncing git repo %s: %v", source.Path, err)
				continue
			}

			// Reconcile the cloned repo's local path
			sourceToReconcile.Path = localRepoPath
			reconcileLocalSource(db, &sourceToReconcile)
		}
	}
	log.Println("Sync process complete.")
}

// gitUrlToLocalPath creates a safe local directory path from a git URL.
func gitUrlToLocalPath(baseDir, repoURL string) (string, error) {
	// A simple way to generate a path: join baseDir with the sanitized URL path.
	// e.g., https://github.com/user/repo.git -> repos/github.com/user/repo.git
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		// Handle git@github.com:user/repo.git style URLs
		if strings.Contains(repoURL, "@") {
			parts := strings.Split(repoURL, ":")
			if len(parts) == 2 {
				hostAndUser := strings.Split(parts[0], "@")
				if len(hostAndUser) == 2 {
					host := hostAndUser[1]
					repoPath := strings.TrimSuffix(parts[1], ".git")
					return filepath.Join(baseDir, host, repoPath), nil
				}
			}
		}
		return "", fmt.Errorf("could not parse git URL: %w", err)
	}
	
	sanitizedPath := strings.TrimSuffix(parsedURL.Path, ".git")
	return filepath.Join(baseDir, parsedURL.Host, sanitizedPath), nil
}

// reconcileLocalSource performs the file scan and DB sync for a local directory.
func reconcileLocalSource(db *storage.DB, source *storage.Source) {
	var parsedCards []domain.Card
	var parseErrors []error
	foundCardHashes := make(map[string]bool)

	walkErr := filepath.WalkDir(source.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil { return err }
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			fileCards, parseErr := parser.ParseFile(path)
			if parseErr != nil {
				parseErrors = append(parseErrors, fmt.Errorf("parsing %s: %w", path, parseErr))
			}
			for _, card := range fileCards {
				card.Hash = knol.Hash(card)
				parsedCards = append(parsedCards, card)
				foundCardHashes[card.Hash] = true
				
				cardState, findErr := db.FindCardStateByHash(card.Hash)
				if findErr != nil {
					parseErrors = append(parseErrors, fmt.Errorf("db check for %s: %w", card.Hash, findErr))
					continue
				}
				if cardState == nil {
					log.Printf("New card found: %s, inserting...", card.Hash)
					if insertErr := db.InsertCard(card, source.ID); insertErr != nil {
						parseErrors = append(parseErrors, fmt.Errorf("db insert for %s: %w", card.Hash, insertErr))
					}
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Printf("Error walking directory %s: %v", source.Path, walkErr)
		return
	}

	dbCards, err := db.GetCardsBySourceID(source.ID)
	if err != nil {
		log.Printf("Error getting cards for source %d: %v", source.ID, err)
		return
	}

	var orphanedCards int
	for _, dbCard := range dbCards {
		if _, found := foundCardHashes[dbCard.Hash]; !found {
			log.Printf("Orphaned card, deleting: %s", dbCard.Hash)
			orphanedCards++
			if err := db.DeleteCardByHash(dbCard.Hash); err != nil {
				log.Printf("Warning: Failed to delete orphaned card %s: %v", dbCard.Hash, err)
			}
		}
	}

	if err := db.UpdateSourceLastScanned(source.ID); err != nil {
		log.Printf("Warning: Failed to update last scanned for source %d: %v", source.ID, err)
	}

	fmt.Printf("Reconciliation for '%s' complete. Found %d cards. %d orphaned deleted. %d errors.\n",
		source.Path, len(parsedCards), orphanedCards, len(parseErrors))
}

