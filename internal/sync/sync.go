package sync

import (
	"fmt"
	"io/fs"
	"log/slog"
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
	slog.Info("Starting sync process for all sources...")
	sources, err := db.GetAllSources()
	if err != nil {
		slog.Error("Failed to get sources", "error", err)
		os.Exit(1)
	}

	if len(sources) == 0 {
		slog.Info("No sources configured. Add one with --add-source <path/or/url.git>")
		return
	}

	reposDir := "repos"
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		slog.Error("Failed to create repos directory", "error", err)
		os.Exit(1)
	}

	for _, source := range sources {
		slog.Info("Syncing source", "id", source.ID, "type", source.Type, "path", source.Path)

		sourceToReconcile := source

		if source.Type == "local" {
			reconcileLocalSource(db, &sourceToReconcile)
		} else if source.Type == "git" {
			localRepoPath, err := gitUrlToLocalPath(reposDir, source.Path)
			if err != nil {
				slog.Error("Error determining local path for git repo", "url", source.Path, "error", err)
				continue
			}

			if err := gitsource.Sync(source.Path, localRepoPath); err != nil {
				slog.Error("Error syncing git repo", "url", source.Path, "error", err)
				continue
			}

			sourceToReconcile.Path = localRepoPath
			reconcileLocalSource(db, &sourceToReconcile)
		}
	}
	slog.Info("Sync process complete.")
}

func reconcileLocalSource(db *storage.DB, source *storage.Source) {
	var parsedCards []domain.Card
	var parseErrors []error
	foundCardHashes := make(map[string]bool)

	walkErr := filepath.WalkDir(source.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			fileCards, parseErr := parser.ParseFile(path)
			if parseErr != nil {
				parseErrors = append(parseErrors, fmt.Errorf("parsing %s: %w", path, parseErr))
			}
			for _, card := range fileCards {
				card.Hash = knol.Hash(card)
				parsedCards = append(parsedCards, card)
				foundCardHashes[card.Hash] = true

				existingCard, findErr := db.FindCardByHash(card.Hash)
				if findErr != nil {
					parseErrors = append(parseErrors, fmt.Errorf("db check for %s: %w", card.Hash, findErr))
					continue
				}
				if existingCard == nil {
					slog.Info("New card found, inserting...", "hash", card.Hash)
					if insertErr := db.InsertCard(card, source.ID); insertErr != nil {
						parseErrors = append(parseErrors, fmt.Errorf("db insert for %s: %w", card.Hash, insertErr))
					}
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		slog.Error("Error walking directory", "path", source.Path, "error", walkErr)
		return
	}

	dbCards, err := db.GetCardsBySourceID(source.ID)
	if err != nil {
		slog.Error("Error getting cards for source", "source_id", source.ID, "error", err)
		return
	}

	var orphanedCards int
	for _, dbCard := range dbCards {
		if _, found := foundCardHashes[dbCard.Hash]; !found {
			slog.Info("Orphaned card, deleting", "hash", dbCard.Hash)
			orphanedCards++
			if err := db.DeleteCardByHash(dbCard.Hash); err != nil {
				slog.Warn("Failed to delete orphaned card", "hash", dbCard.Hash, "error", err)
			}
		}
	}

	if err := db.UpdateSourceLastScanned(source.ID); err != nil {
		slog.Warn("Failed to update last scanned for source", "source_id", source.ID, "error", err)
	}

	slog.Info("reconciliation complete",
		"path", source.Path,
		"parsed_cards", len(parsedCards),
		"orphaned_deleted", orphanedCards,
		"errors", len(parseErrors),
	)
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
