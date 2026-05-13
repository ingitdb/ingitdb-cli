package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// fakeTreeWriter records the changes it would commit so tests can assert the
// caller assembled the right set of operations.
type fakeTreeWriter struct {
	files       []string // returned by ListFilesUnder
	listErr     error
	commitErr   error
	gotMessage  string
	gotChanges  []dalgo2ghingitdb.TreeChange
	commitCalls int
}

func (f *fakeTreeWriter) ListFilesUnder(_ context.Context, _ string) ([]string, error) {
	return f.files, f.listErr
}

func (f *fakeTreeWriter) CommitChanges(_ context.Context, msg string, changes []dalgo2ghingitdb.TreeChange) (string, error) {
	f.commitCalls++
	f.gotMessage = msg
	f.gotChanges = changes
	if f.commitErr != nil {
		return "", f.commitErr
	}
	return "fake-new-commit-sha", nil
}

type fakeTreeWriterFactory struct {
	w *fakeTreeWriter
}

func (f *fakeTreeWriterFactory) NewTreeWriter(_ dalgo2ghingitdb.Config) (treeWriter, error) {
	return f.w, nil
}

// withFakeRemote installs fake gitHubFileReaderFactory + treeWriterFactory
// for the duration of the test and returns the captured fakes plus a cleanup
// function. Tests that use this MUST NOT run in parallel (package-level
// variables).
func withFakeRemote(t *testing.T, files map[string][]byte, fakeFiles []string) (*fakeTreeWriter, func()) {
	t.Helper()

	prevReader := gitHubFileReaderFactory
	prevWriter := treeWriterFactory

	reader := &fakeFileReader{files: files}
	gitHubFileReaderFactory = &constantFileReaderFactory{reader: reader}

	fw := &fakeTreeWriter{files: fakeFiles}
	treeWriterFactory = &fakeTreeWriterFactory{w: fw}

	return fw, func() {
		gitHubFileReaderFactory = prevReader
		treeWriterFactory = prevWriter
	}
}

// constantFileReaderFactory always returns the same reader. Used to short-circuit
// the existing factory plumbing in remote-drop tests.
type constantFileReaderFactory struct {
	reader dalgo2ghingitdb.FileReader
}

func (f *constantFileReaderFactory) NewGitHubFileReader(_ dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
	return f.reader, nil
}

// emptyDropDeps returns dependencies suitable for the remote drop path,
// where local-source functions are never invoked.
func emptyDropDeps(t *testing.T) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return "/tmp/db", nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		t.Fatal("readDef should not be called on remote path")
		return nil, nil
	}
	newDB = func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		t.Fatal("newDB should not be called on remote path")
		return nil, nil
	}
	logf = func(...any) {}
	return
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestDropCollection_Remote_HappyPath verifies that drop collection --remote
// (a) reads .ingitdb/root-collections.yaml, (b) enumerates every file in the
// collection directory via ListFilesUnder, (c) commits one batch containing
// the file deletions plus the root-collections.yaml update.
//
// Modifies package-level variables — must not run in parallel.
func TestDropCollection_Remote_HappyPath(t *testing.T) {
	rootYAML := "countries: data/countries\ncities: data/cities\n"
	files := map[string][]byte{
		".ingitdb/root-collections.yaml": []byte(rootYAML),
	}
	enumFiles := []string{
		"data/cities/ie.yaml",
		"data/cities/gb.yaml",
		"data/cities/$views/by_country.yaml",
	}
	fw, cleanup := withFakeRemote(t, files, enumFiles)
	defer cleanup()

	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "cities", "--remote=github.com/owner/repo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fw.commitCalls != 1 {
		t.Fatalf("expected exactly 1 CommitChanges call, got %d", fw.commitCalls)
	}
	if !strings.Contains(fw.gotMessage, "drop collection cities") {
		t.Errorf("commit message %q should mention 'drop collection cities'", fw.gotMessage)
	}

	// Expect: 1 root-collections.yaml modification + 3 file deletions = 4 entries.
	if len(fw.gotChanges) != len(enumFiles)+1 {
		t.Fatalf("expected %d changes, got %d", len(enumFiles)+1, len(fw.gotChanges))
	}
	gotPaths := make(map[string]bool, len(fw.gotChanges))
	var rootChange *dalgo2ghingitdb.TreeChange
	for i := range fw.gotChanges {
		ch := fw.gotChanges[i]
		gotPaths[ch.Path] = true
		if ch.Path == ".ingitdb/root-collections.yaml" {
			rootChange = &fw.gotChanges[i]
		}
	}
	for _, f := range enumFiles {
		if !gotPaths[f] {
			t.Errorf("expected change for %q, not found", f)
		}
	}
	if rootChange == nil {
		t.Fatal("expected change for .ingitdb/root-collections.yaml")
	}
	// Modified root-collections.yaml must include the surviving collection.
	if !strings.Contains(string(rootChange.Content), "countries") {
		t.Errorf("modified root-collections.yaml should retain 'countries', got: %s", rootChange.Content)
	}
	// ... and MUST NOT include the dropped collection.
	if strings.Contains(string(rootChange.Content), "cities") {
		t.Errorf("modified root-collections.yaml should NOT retain dropped 'cities', got: %s", rootChange.Content)
	}
}

// TestDropCollection_Remote_IfExistsMissing verifies that --if-exists turns
// "collection not found" into a silent success on the remote path, with no
// commit attempted.
func TestDropCollection_Remote_IfExistsMissing(t *testing.T) {
	files := map[string][]byte{
		".ingitdb/root-collections.yaml": []byte("countries: data/countries\n"),
	}
	fw, cleanup := withFakeRemote(t, files, nil)
	defer cleanup()

	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "missing", "--remote=github.com/owner/repo", "--if-exists"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if fw.commitCalls != 0 {
		t.Errorf("expected 0 commits when target missing + --if-exists, got %d", fw.commitCalls)
	}
}

// TestDropCollection_Remote_MissingErrors verifies that without --if-exists,
// dropping a non-existent collection fails before any commit.
func TestDropCollection_Remote_MissingErrors(t *testing.T) {
	files := map[string][]byte{
		".ingitdb/root-collections.yaml": []byte("countries: data/countries\n"),
	}
	fw, cleanup := withFakeRemote(t, files, nil)
	defer cleanup()

	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "missing", "--remote=github.com/owner/repo"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing collection without --if-exists")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should name the missing collection, got: %v", err)
	}
	if fw.commitCalls != 0 {
		t.Errorf("expected 0 commits on error path, got %d", fw.commitCalls)
	}
}

// TestDrop_Remote_PathMutex verifies that --path and --remote together are
// rejected with a clear error, before any I/O.
func TestDrop_Remote_PathMutex(t *testing.T) {
	t.Parallel()
	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "x", "--path=.", "--remote=github.com/owner/repo"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --path with --remote")
	}
	if !strings.Contains(err.Error(), "--path") || !strings.Contains(err.Error(), "--remote") {
		t.Errorf("error should mention both --path and --remote, got: %v", err)
	}
}

// TestDrop_Remote_InvalidRemoteValue verifies that an invalid --remote value
// is rejected by the parser before any I/O.
func TestDrop_Remote_InvalidRemoteValue(t *testing.T) {
	t.Parallel()
	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "x", "--remote=invalid-no-slash"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed --remote")
	}
}

// TestDrop_Remote_ProviderNotImplemented verifies that --provider=gitlab
// fails fast with a clear error since only the github provider has an adapter.
func TestDrop_Remote_ProviderNotImplemented(t *testing.T) {
	t.Parallel()
	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "x", "--remote=gitlab.com/owner/repo"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "not yet supported") {
		t.Errorf("error should mention 'not yet supported', got: %v", err)
	}
}

// TestDropView_Remote_HappyPath verifies that drop view --remote reads the
// view file, finds its `file_name`, and atomically removes both the view
// definition and the materialized output in one commit.
func TestDropView_Remote_HappyPath(t *testing.T) {
	viewYAML := "file_name: by_country.csv\nselect: '*'\nfrom: cities\n"
	files := map[string][]byte{
		".ingitdb/root-collections.yaml":     []byte("cities: data/cities\n"),
		"data/cities/$views/by_country.yaml": []byte(viewYAML),
	}
	fw, cleanup := withFakeRemote(t, files, nil)
	defer cleanup()

	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"view", "by_country", "--remote=github.com/owner/repo"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fw.commitCalls != 1 {
		t.Fatalf("expected 1 commit, got %d", fw.commitCalls)
	}
	if !strings.Contains(fw.gotMessage, "drop view by_country") {
		t.Errorf("commit message %q should mention 'drop view by_country'", fw.gotMessage)
	}
	wantPaths := map[string]bool{
		"data/cities/$views/by_country.yaml": true,
		"data/cities/by_country.csv":         true,
	}
	if len(fw.gotChanges) != len(wantPaths) {
		t.Fatalf("expected %d changes, got %d", len(wantPaths), len(fw.gotChanges))
	}
	for _, ch := range fw.gotChanges {
		if !wantPaths[ch.Path] {
			t.Errorf("unexpected change path %q", ch.Path)
		}
		if ch.Content != nil {
			t.Errorf("change for %q should be a deletion (nil Content), got content %q", ch.Path, ch.Content)
		}
	}
}
