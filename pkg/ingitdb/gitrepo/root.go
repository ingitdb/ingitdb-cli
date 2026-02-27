package gitrepo

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRepoRoot walks up the directory tree from startPath looking for a .git entry
// (either a file or a directory) and returns the absolute path of the first directory
// that contains .git. It returns an error if the filesystem root is reached without
// finding .git.
func FindRepoRoot(startPath string) (string, error) {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %q: %w", startPath, err)
	}

	// Start from the given path and walk up the directory tree
	current := absPath

	for {
		gitPath := filepath.Join(current, ".git")
		_, err := os.Stat(gitPath)
		if err == nil {
			// .git exists, return this directory
			return current, nil
		}
		if !os.IsNotExist(err) {
			// Some other error occurred (permission denied, etc.)
			return "", fmt.Errorf("failed to check for .git in %q: %w", current, err)
		}

		// Move to parent directory
		parent := filepath.Dir(current)
		if parent == current {
			// We've reached the filesystem root
			return "", fmt.Errorf("no .git directory found in %q or any parent directory", absPath)
		}
		current = parent
	}
}
