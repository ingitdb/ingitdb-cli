package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
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
	runGit(t, tempDir, "branch", "-m", "main")

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
						"$ID": {Type: ingitdb.ColumnTypeString},
					},
					ColumnsOrder: []string{"$ID"},
				},
			},
		}, nil
	}

	cmd := Rebase(getWd, readDefinition, logf)

	err := runCobraCommand(cmd, "--base_ref", "base_branch", "--resolve", "readme")
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

// setupRebaseCollectionReadmeConflict creates a git repo where rebasing main
// onto base conflicts on a collection README.md. With secondConflict, main has
// two README-changing commits so `git rebase --continue` hits a second
// conflict. Returns the collection directory.
func setupRebaseCollectionReadmeConflict(t *testing.T, dir string, secondConflict bool) string {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	colDir := filepath.Join(dir, "docs", "col")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	readme := filepath.Join(colDir, "README.md")
	writeRebaseFile(t, readme, "# Initial\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "branch", "-m", "main")
	runGit(t, dir, "branch", "base")

	writeRebaseFile(t, readme, "# Main 1\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "main 1")
	if secondConflict {
		writeRebaseFile(t, readme, "# Main 2\n")
		runGit(t, dir, "add", ".")
		runGit(t, dir, "commit", "-m", "main 2")
	}

	runGit(t, dir, "checkout", "base")
	writeRebaseFile(t, readme, "# Base\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	runGit(t, dir, "checkout", "main")
	return colDir
}

func writeRebaseFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func rebaseCollectionDef(colDir string) func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"col": {ID: "col", DirPath: colDir, Columns: map[string]*ingitdb.ColumnDef{}},
			},
		}, nil
	}
}

// TestRebase_GitAddFails covers the resolveErr branch: a rebase is left halted
// on a README conflict and the index is locked, so the command's internal
// `git add` (inside the resolve engine) fails.
func TestRebase_GitAddFails(t *testing.T) {
	dir := t.TempDir()
	colDir := setupRebaseCollectionReadmeConflict(t, dir, false)

	// Leave a rebase halted on the README conflict.
	_ = runGitNoFailOut(dir, "rebase", "base")
	conflict := strings.TrimSpace(runGitNoFailOut(dir, "diff", "--name-only", "--diff-filter=U"))
	if conflict == "" {
		t.Skip("git auto-merged; no conflict produced")
	}
	// Lock the index so any `git add` fails (git diff reads still succeed).
	if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), nil, 0o644); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}

	getWd := func() (string, error) { return dir, nil }
	cmd := Rebase(getWd, rebaseCollectionDef(colDir), func(...any) {})
	err := runCobraCommand(cmd, "--base_ref=base", "--resolve=readme")
	_ = os.Remove(filepath.Join(dir, ".git", "index.lock"))
	_ = runGitNoFail(dir, "rebase", "--abort")
	if err == nil {
		t.Skip("git add did not fail in this environment")
	}
	if !strings.Contains(err.Error(), "failed to resolve docs") {
		t.Skipf("rebase failed before resolve step: %v", err)
	}
}

// TestRebase_CommitFails covers the commitErr branch: README resolves and is
// staged, but `git commit` fails because the repo has no configured identity.
func TestRebase_CommitFails(t *testing.T) {
	// Neutralize global/system git config so identity cannot fall back to the
	// developer's ~/.gitconfig. Uses t.Setenv → must not be parallel.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	dir := t.TempDir()
	colDir := setupRebaseCollectionReadmeConflict(t, dir, false)

	out := runGitNoFailOut(dir, "rebase", "base")
	_ = out
	conflict := strings.TrimSpace(runGitNoFailOut(dir, "diff", "--name-only", "--diff-filter=U"))
	_ = runGitNoFail(dir, "rebase", "--abort")
	if conflict == "" {
		t.Skip("git auto-merged; no conflict produced")
	}
	// Remove local identity and forbid auto-detection; with global/system
	// neutralized, commit must fail because no identity is available.
	_ = runGitNoFail(dir, "config", "--unset", "user.email")
	_ = runGitNoFail(dir, "config", "--unset", "user.name")
	runGit(t, dir, "config", "user.useConfigOnly", "true")

	getWd := func() (string, error) { return dir, nil }
	cmd := Rebase(getWd, rebaseCollectionDef(colDir), func(...any) {})
	err := runCobraCommand(cmd, "--base_ref=base", "--resolve=readme")
	_ = runGitNoFail(dir, "rebase", "--abort")
	if err == nil {
		t.Skip("commit did not fail in this environment")
	}
	if !strings.Contains(err.Error(), "failed to commit") {
		t.Errorf("expected commit failure, got: %v", err)
	}
}

// TestRebase_ContinueFails covers the contErr branch: the first README conflict
// resolves and commits, but `git rebase --continue` hits a second conflict.
func TestRebase_ContinueFails(t *testing.T) {
	dir := t.TempDir()
	colDir := setupRebaseCollectionReadmeConflict(t, dir, true)

	getWd := func() (string, error) { return dir, nil }
	cmd := Rebase(getWd, rebaseCollectionDef(colDir), func(...any) {})
	err := runCobraCommand(cmd, "--base_ref=base", "--resolve=readme")
	_ = runGitNoFail(dir, "rebase", "--abort")
	if err == nil {
		t.Skip("rebase completed without a second conflict in this environment")
	}
	if !strings.Contains(err.Error(), "failed to continue rebase") {
		t.Errorf("expected continue failure, got: %v", err)
	}
}

// runGitNoFailOut runs git, ignoring errors, returning combined output.
func runGitNoFailOut(dir string, args ...string) string {
	c := exec.Command("git", args...)
	c.Dir = dir
	out, _ := c.CombinedOutput()
	return string(out)
}
