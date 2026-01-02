package sync

import (
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/conorfennell/knolhash/internal/domain"
	"github.com/conorfennell/knolhash/internal/gitsource"
	"github.com/conorfennell/knolhash/internal/knol"
	"github.com/conorfennell/knolhash/internal/parser"
	"github.com/conorfennell/knolhash/internal/storage"
)

// RunSync iterates over all sources and reconciles them.
func RunSync(db *storage.DB) {
	log.Println("Starting sync process for all sources...")
	sources, err := db.GetAllSources()
	if err != nil {
		log.Fatalf("Failed to get sources: %v", err)
	}

	if len(sources) == 0 {
		log.Println("No sources configured. Add one with --add-source <path/or/url.git>")
		return
	}

	reposDir := "repos"
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create repos directory: %v", err)
	}

	for _, source := range sources {
		log.Printf("Syncing source ID %d (Type: %s, Path: %s)", source.ID, source.Type, source.Path)
		
		sourceToReconcile := source
		
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

			sourceToReconcile.Path = localRepoPath
			reconcileLocalSource(db, &sourceToReconcile)
		}
	}
	log.Println("Sync process complete.")
}

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

func gitUrlToLocalPath(baseDir, repoURL string) (string, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
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
		return "", fmt.Errorf("could not parse git URL: %s", repoURL)
	}
	
	sanitizedPath := strings.TrimSuffix(parsedURL.Path, ".git")
	return filepath.Join(baseDir, parsedURL.Host, sanitizedPath), nil
}
