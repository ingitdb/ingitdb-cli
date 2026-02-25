package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/urfave/cli/v3"
)

func TestRebaseCommand(t *testing.T) {
	// Setup a temporary directory acting as our test DB and git repo
	tempDir := t.TempDir()

	// Initialize a git repo
	runGit(t, tempDir, "init")
	runGit(t, tempDir, "config", "user.email", "test@example.com")
	runGit(t, tempDir, "config", "user.name", "Test User")

	// Create a stable initial commit on main
	runGit(t, tempDir, "commit", "--allow-empty", "-m", "initial commit")

	// Create a collection README.md
	readmePath := filepath.Join(tempDir, "docs", "demo-apps", "test", "README.md")
	if err := os.MkdirAll(filepath.Dir(readmePath), 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(readmePath, []byte("# Initial"), 0o644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}

	// Create ingitdb definition to avoid docs update failures
	defPath := filepath.Join(tempDir, ".ingitdb.yaml")
	if err := os.WriteFile(defPath, []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write def: %v", err)
	}
	colDefPath := filepath.Join(tempDir, "docs", "demo-apps", "test", "collection.yaml")
	if err := os.WriteFile(colDefPath, []byte("id: test"), 0o644); err != nil {
		t.Fatalf("failed to write col def: %v", err)
	}

	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "add readme and config")

	runGit(t, tempDir, "branch", "base_branch")

	// Make a change on the main branch
	if err := os.WriteFile(readmePath, []byte("# Changed on main"), 0o644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "change on main")

	// Switch to base_branch and make a conflicting change
	runGit(t, tempDir, "checkout", "base_branch")
	if err := os.WriteFile(readmePath, []byte("# Changed on base_branch"), 0o644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}
	runGit(t, tempDir, "add", ".")
	runGit(t, tempDir, "commit", "-m", "change on base_branch")

	// Check out main and attempt to rebase base_branch.
	// Wait, the test is: `main` is our branch, we rebase ON TOP OF `base_branch`.
	// So we should be on `main`, and rebase onto `base_branch`.
	runGit(t, tempDir, "checkout", "main")

	// Now run the rebase command.
	getWd := func() (string, error) { return tempDir, nil }

	fakeLogs := []string{}
	logf := func(args ...any) {
		var msgs []string
		for _, arg := range args {
			msgs = append(msgs, fmt.Sprint(arg))
		}
		fakeLogs = append(fakeLogs, msgs...)
	}

	readDefinition := func(path string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"test": {
					ID:      "test",
					DirPath: filepath.Join(tempDir, "docs", "demo-apps", "test"),
					Titles:  map[string]string{"en": "Test Collection"},
					Columns: map[string]*ingitdb.ColumnDef{
						"id": {Type: ingitdb.ColumnTypeString},
					},
					ColumnsOrder: []string{"id"},
				},
			},
		}, nil
	}

	cmd := Rebase(getWd, readDefinition, logf)

	app := &cli.Command{
		Commands:  []*cli.Command{cmd},
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	}

	err := app.Run(context.Background(), []string{"ingitdb", "rebase", "--base_ref", "base_branch", "--resolve", "readme"})
	if err != nil {
		t.Fatalf("unexpected error running rebase: %v", err)
	}

	// Verify the commit message was amended!
	out := runGit(t, tempDir, "log", "-1", "--pretty=%B")
	if !strings.Contains(string(out), "chore(ingitdb):") {
		t.Errorf("expected commit message to start with chore(ingitdb):, got %q", string(out))
	}
}

func runGit(t *testing.T, dir string, args ...string) []byte {
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
	return out
}
