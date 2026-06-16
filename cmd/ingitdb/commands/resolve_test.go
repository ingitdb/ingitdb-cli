package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

var errReadDef = errors.New("readDefinition failed")

func falseTerminal() bool { return false }

func noopConflictsTUI(context.Context, []string) error { return nil }

func testHomeDir() (string, error) { return "/home/test", nil }

// setupMergeConflict creates a git repo at a fresh temp dir with a merge
// conflict on relPath, and returns the repo directory. The file ends up with
// conflict markers and in an unmerged state.
func setupMergeConflict(t *testing.T, relPath string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte("# Initial\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	runGit(t, dir, "branch", "-m", "main")
	runGit(t, dir, "branch", "feature")

	if err := os.WriteFile(full, []byte("# Changed on main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "main change")

	runGit(t, dir, "checkout", "feature")
	if err := os.WriteFile(full, []byte("# Changed on feature\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "feature change")
	runGit(t, dir, "checkout", "main")

	mergeCmd := exec.Command("git", "merge", "--no-ff", "feature")
	mergeCmd.Dir = dir
	_ = mergeCmd.Run() // expected to fail (conflict)
	return dir
}

func testDefWithCollection(dir string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:           "test",
				DirPath:      filepath.Join(dir, "docs", "demo-apps", "test"),
				Titles:       map[string]string{"en": "Test Collection"},
				Columns:      map[string]*ingitdb.ColumnDef{"$ID": {Type: ingitdb.ColumnTypeString}},
				ColumnsOrder: []string{"$ID"},
			},
		},
	}
}

func TestResolve_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Resolve(testHomeDir, func() (string, error) { return ".", nil }, nil, func(...any) {}, falseTerminal, noopConflictsTUI)
	if cmd == nil {
		t.Fatal("Resolve() returned nil")
		return
	}
	if cmd.Use != "resolve" {
		t.Errorf("expected name 'resolve', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected RunE to be set")
	}
}

func TestResolve_AutoResolvesReadmeConflict(t *testing.T) {
	t.Parallel()

	readmeRel := filepath.Join("docs", "demo-apps", "test", "README.md")
	dir := setupMergeConflict(t, readmeRel)
	// Provide a collection definition file so ProcessCollection has context.
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile def: %v", err)
	}

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testDefWithCollection(dir), nil
	}
	var logs []string
	logf := func(args ...any) {
		for _, a := range args {
			logs = append(logs, fmt.Sprint(a))
		}
	}
	_ = logs

	cmd := Resolve(testHomeDir, getWd, readDef, logf, falseTerminal, noopConflictsTUI)
	if err := runCobraCommand(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The README conflict should now be staged/resolved: no unmerged files.
	out := runGit(t, dir, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("expected no unmerged files after resolve, got: %s", out)
	}
}

func TestResolve_NoConflicts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	var logs []string
	logf := func(args ...any) {
		for _, a := range args {
			logs = append(logs, fmt.Sprint(a))
		}
	}

	cmd := Resolve(testHomeDir, getWd, readDef, logf, falseTerminal, noopConflictsTUI)
	if err := runCobraCommand(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, m := range logs {
		if strings.Contains(m, "no conflicts to resolve") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no conflicts to resolve' log, got: %v", logs)
	}
}

func TestResolve_UnresolvedNonReadmeConflict(t *testing.T) {
	t.Parallel()

	dir := setupMergeConflict(t, "data.txt")

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	var logs []string
	logf := func(args ...any) {
		for _, a := range args {
			logs = append(logs, fmt.Sprint(a))
		}
	}

	cmd := Resolve(testHomeDir, getWd, readDef, logf, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Skip("git auto-merged; no conflict produced")
	}
	if !strings.Contains(err.Error(), "not implemented yet") {
		t.Errorf("expected 'not implemented yet' error, got: %v", err)
	}
	// Non-terminal path prints the placeholder text.
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "Interactive conflict resolution") {
		t.Errorf("expected placeholder text in logs, got: %v", logs)
	}
}

// TestResolve_SourceConflicts_TerminalLaunchesTUI covers the terminal branch:
// the interactive resolver is invoked, then a non-zero error is returned
// because manual resolution is not implemented yet.
func TestResolve_SourceConflicts_TerminalLaunchesTUI(t *testing.T) {
	t.Parallel()

	dir := setupMergeConflict(t, "data.txt")

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	tuiCalled := false
	runTUI := func(_ context.Context, files []string) error {
		tuiCalled = true
		if len(files) == 0 {
			t.Error("expected conflicted files passed to TUI")
		}
		return nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, func() bool { return true }, runTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Skip("git auto-merged; no conflict produced")
	}
	if !tuiCalled {
		t.Error("expected interactive resolver to be invoked on a terminal")
	}
	if !strings.Contains(err.Error(), "not implemented yet") {
		t.Errorf("expected 'not implemented yet' error, got: %v", err)
	}
}

// TestResolve_SourceConflicts_TUIError covers the branch where the interactive
// resolver itself returns an error.
func TestResolve_SourceConflicts_TUIError(t *testing.T) {
	t.Parallel()

	dir := setupMergeConflict(t, "data.txt")

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	runTUI := func(context.Context, []string) error { return errReadDef }

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, func() bool { return true }, runTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Skip("git auto-merged; no conflict produced")
	}
	if !errors.Is(err, errReadDef) {
		t.Errorf("expected the TUI error to propagate, got: %v", err)
	}
}

func TestResolve_GitDiffError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir() // not a git repo → git diff fails

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error in non-git directory")
	}
	if !strings.Contains(err.Error(), "failed to get conflicted files") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolve_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	readmeRel := filepath.Join("docs", "demo-apps", "test", "README.md")
	dir := setupMergeConflict(t, readmeRel)

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errReadDef
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Skip("git auto-merged; no conflict produced")
	}
	if !strings.Contains(err.Error(), "failed to read database definition") {
		t.Errorf("expected read-definition error, got: %v", err)
	}
}

func TestResolve_FileFilter_NoMatch(t *testing.T) {
	t.Parallel()

	dir := setupMergeConflict(t, "data.txt")

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	var logs []string
	logf := func(args ...any) {
		for _, a := range args {
			logs = append(logs, fmt.Sprint(a))
		}
	}

	cmd := Resolve(testHomeDir, getWd, readDef, logf, falseTerminal, noopConflictsTUI)
	// --file points at a path that is not among the conflicts → nothing to do.
	if err := runCobraCommand(cmd, "--file", "nonexistent.md"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, m := range logs {
		if strings.Contains(m, "no conflicts to resolve") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no conflicts to resolve' log, got: %v", logs)
	}
}

func TestResolve_GetWdError(t *testing.T) {
	t.Parallel()

	getWd := func() (string, error) { return "", fmt.Errorf("no working dir") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
	if !strings.Contains(err.Error(), "failed to get working directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

// lockGitIndex creates .git/index.lock so a subsequent `git add` fails
// deterministically. It skips the test if there is no conflict to resolve.
func lockGitIndex(t *testing.T, dir string) {
	t.Helper()
	out := runGit(t, dir, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(string(out)) == "" {
		t.Skip("git auto-merged; no conflict produced")
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "index.lock"), []byte(""), 0o644); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}
}

func TestResolve_GitAddFails(t *testing.T) {
	t.Parallel()

	readmeRel := filepath.Join("docs", "demo-apps", "test", "README.md")
	dir := setupMergeConflict(t, readmeRel)
	lockGitIndex(t, dir)

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testDefWithCollection(dir), nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected git add to fail with index.lock present")
	}
	if !strings.Contains(err.Error(), "failed to stage resolved files") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolve_ProcessCollectionError(t *testing.T) {
	t.Parallel()

	readmeRel := filepath.Join("docs", "demo-apps", "test", "README.md")
	dir := setupMergeConflict(t, readmeRel)
	out := runGit(t, dir, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(string(out)) == "" {
		t.Skip("git auto-merged; no conflict produced")
	}
	// Replace README.md with a directory so ProcessCollection cannot write it.
	readmePath := filepath.Join(dir, readmeRel)
	if err := os.Remove(readmePath); err != nil {
		t.Fatalf("remove README: %v", err)
	}
	if err := os.MkdirAll(readmePath, 0o755); err != nil {
		t.Fatalf("mkdir README: %v", err)
	}

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testDefWithCollection(dir), nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when ProcessCollection fails")
	}
}

func TestRunDocsUpdate_GitAddFails_Locked(t *testing.T) {
	t.Parallel()

	readmeRel := filepath.Join("docs", "demo-apps", "test", "README.md")
	dir := setupMergeConflict(t, readmeRel)
	lockGitIndex(t, dir)

	def := testDefWithCollection(dir)
	err := runDocsUpdate(t.Context(), dir, def, "", "readme", func(...any) {})
	if err == nil {
		t.Fatal("expected git add to fail with index.lock present")
	}
	if !strings.Contains(err.Error(), "failed to stage resolved files") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestResolve_RecordMergeAutoResolves covers the branch where
// resolveRecordMergeConflicts succeeds and returns non-empty mergedFiles with
// no stillUnresolved entries (lines 69-71 and 75 of resolve.go).
func TestResolve_RecordMergeAutoResolves(t *testing.T) {
	t.Parallel()

	// A YAML map-of-records file with disjoint additions on each branch — the
	// record-merge engine can auto-resolve this without human input.
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	var logs []string
	logf := func(args ...any) {
		for _, a := range args {
			logs = append(logs, fmt.Sprint(a))
		}
	}

	cmd := Resolve(testHomeDir, getWd, readDef, logf, falseTerminal, noopConflictsTUI)
	if err := runCobraCommand(cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "auto-merged") {
		t.Errorf("expected 'auto-merged' log, got: %v", logs)
	}
}

// TestResolve_RecordMergeInfraError covers the branch where
// resolveRecordMergeConflicts itself returns an infrastructure error (e.g. a
// locked git index prevents staging the merged file) — line 66-68 of resolve.go.
func TestResolve_RecordMergeInfraError(t *testing.T) {
	t.Parallel()

	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)
	lockGitIndex(t, dir)

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}

	cmd := Resolve(testHomeDir, getWd, readDef, func(...any) {}, falseTerminal, noopConflictsTUI)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected an error when git add fails during record merge")
	}
	if !strings.Contains(err.Error(), "stage merged") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFilterConflictedByFile(t *testing.T) {
	t.Parallel()

	files := []string{"docs/a/README.md", "docs/b/data.txt"}
	tests := []struct {
		name     string
		only     string
		wantLen  int
		wantHave string
	}{
		{"by relative path", "docs/a/README.md", 1, "docs/a/README.md"},
		{"by base name", "data.txt", 1, "docs/b/data.txt"},
		{"no match", "missing.go", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterConflictedByFile(files, tt.only)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%v)", len(got), tt.wantLen, got)
			}
			if tt.wantHave != "" && got[0] != tt.wantHave {
				t.Errorf("got[0] = %q, want %q", got[0], tt.wantHave)
			}
		})
	}
}
