package gitrepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRoot_Found(t *testing.T) {
	t.Run("finds .git directory in current path", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.Mkdir(gitDir, 0o755); err != nil {
			t.Fatalf("failed to create .git directory: %v", err)
		}

		root, err := FindRepoRoot(tmpDir)
		if err != nil {
			t.Fatalf("FindRepoRoot failed: %v", err)
		}

		if root != tmpDir {
			t.Errorf("expected root %q, got %q", tmpDir, root)
		}
	})

	t.Run("finds .git directory in parent path", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitDir := filepath.Join(tmpDir, ".git")
		if err := os.Mkdir(gitDir, 0o755); err != nil {
			t.Fatalf("failed to create .git directory: %v", err)
		}

		// Create a subdirectory
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.Mkdir(subDir, 0o755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		// Search from the subdirectory
		root, err := FindRepoRoot(subDir)
		if err != nil {
			t.Fatalf("FindRepoRoot failed: %v", err)
		}

		if root != tmpDir {
			t.Errorf("expected root %q, got %q", tmpDir, root)
		}
	})

	t.Run("finds .git file in parent path", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitFile := filepath.Join(tmpDir, ".git")
		if err := os.WriteFile(gitFile, []byte("gitdir: .git"), 0o644); err != nil {
			t.Fatalf("failed to create .git file: %v", err)
		}

		// Create a nested subdirectory
		subDir := filepath.Join(tmpDir, "sub", "dir")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("failed to create subdirectories: %v", err)
		}

		// Search from the nested subdirectory
		root, err := FindRepoRoot(subDir)
		if err != nil {
			t.Fatalf("FindRepoRoot failed: %v", err)
		}

		if root != tmpDir {
			t.Errorf("expected root %q, got %q", tmpDir, root)
		}
	})
}

func TestFindRepoRoot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create a .git directory

	root, err := FindRepoRoot(tmpDir)
	if err == nil {
		t.Errorf("expected error, but got root: %q", root)
	}
}
