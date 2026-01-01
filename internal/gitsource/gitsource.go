package gitsource

import (
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
)

// Sync clones a git repository if it doesn't exist at the given path,
// or pulls the latest changes if it does.
func Sync(url, localPath string) error {
	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		// Path does not exist, clone the repository
		log.Printf("Cloning repository %s into %s...", url, localPath)
		_, err := git.PlainClone(localPath, false, &git.CloneOptions{
			URL:      url,
			Progress: os.Stdout, // You can make this more sophisticated later
		})
		if err != nil {
			return fmt.Errorf("failed to clone repo %s: %w", url, err)
		}
		log.Printf("Clone successful.")
	} else if err == nil {
		// Path exists, pull the latest changes
		log.Printf("Pulling latest changes for repository at %s...", localPath)
		repo, err := git.PlainOpen(localPath)
		if err != nil {
			return fmt.Errorf("failed to open existing repo at %s: %w", localPath, err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree for repo at %s: %w", localPath, err)
		}

		err = worktree.Pull(&git.PullOptions{
			RemoteName: "origin",
			Progress:   os.Stdout,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to pull changes for repo at %s: %w", localPath, err)
		}
		log.Printf("Pull successful (or already up-to-date).")
	} else {
		// Some other error occurred
		return fmt.Errorf("error checking path %s: %w", localPath, err)
	}

	return nil
}
