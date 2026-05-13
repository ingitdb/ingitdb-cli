package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// seedRecord writes a YAML file at <collection.DirPath>/$records/<key>.yaml.
func seedRecord(t *testing.T, dir, collectionID, key string, data map[string]any) error {
	t.Helper()
	def := testDef(dir)
	col, ok := def.Collections[collectionID]
	if !ok {
		return fmt.Errorf("collection %s not in test def", collectionID)
	}
	// testDef sets DirPath to dir directly; records live under $records/ subdir.
	colDir := filepath.Join(col.DirPath, "$records")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		return err
	}
	out, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(colDir, key+".yaml"), out, 0o644)
}

// runSelectCmd creates a Select command with the given deps, sets its
// output to a buffer (so tests are parallel-safe), runs it with args,
// and returns stdout and any error.
func runSelectCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (stdout string, err error) {
	t.Helper()
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err = runCobraCommand(cmd, args...)
	return buf.String(), err
}

// selectTestDeps returns a minimal DI set for the Select command.
func selectTestDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	return homeDir, getWd, readDef, newDB, logf
}

func TestSelect_RegistersAllSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "order-by", "fields", "limit", "min-affected", "format", "path", "remote"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestSelect_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--id=todo.items/x", "--from=todo.items")
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
	if !strings.Contains(err.Error(), "--id") && !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should name --id or --from, got: %v", err)
	}
}

func TestSelect_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}

func TestSelect_SingleRecord_DefaultYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)

	if err := seedRecord(t, dir, "test.items", "alpha", map[string]any{"title": "Alpha", "done": false}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--id=test.items/alpha")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "title: Alpha") {
		t.Errorf("expected YAML field title: Alpha, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "done: false") {
		t.Errorf("expected YAML field done: false, got:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_FormatJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "beta", map[string]any{"title": "Beta"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--id=test.items/beta", "--format=json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, `"title": "Beta"`) {
		t.Errorf("expected JSON title:Beta, got:\n%s", stdout)
	}
	// Single-record JSON must be a bare object, NOT an array.
	if strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Errorf("single-record JSON must be an object, got array:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_FormatINGR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "gamma", map[string]any{"title": "Gamma", "done": true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--id=test.items/gamma", "--fields=$id,title,done", "--format=ingr")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 1 record") {
		t.Errorf("single-record INGR must have '# 1 record' footer:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"Gamma"`) {
		t.Errorf("missing title cell:\n%s", stdout)
	}
}

func TestSelect_SetMode_NoFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for _, key := range []string{"a", "b", "c"} {
		if err := seedRecord(t, dir, "test.items", key, map[string]any{"title": "T-" + key}); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--format=yaml")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if c := strings.Count(stdout, "title: T-"); c != 3 {
		t.Errorf("want 3 records, got %d:\n%s", c, stdout)
	}
}

func TestSelect_SetMode_WhereFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "test.items", "b", map[string]any{"priority": float64(5)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--where=priority>2", "--format=yaml")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "priority: 5") {
		t.Errorf("expected priority:5 in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "priority: 1") {
		t.Errorf("did NOT expect priority:1 in output:\n%s", stdout)
	}
}

func TestSelect_SetMode_EmptyResult_CSV(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--where=priority>1000", "--fields=$id,priority")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Errorf("want header-only CSV (1 line), got %d lines:\n%s", len(lines), stdout)
	}
	if !strings.Contains(lines[0], "$id") || !strings.Contains(lines[0], "priority") {
		t.Errorf("header row missing expected columns:\n%s", stdout)
	}
}

func TestSelect_SetMode_EmptyResult_JSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--format=json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if got != "[]" {
		t.Errorf("empty set JSON must be `[]`, got: %s", got)
	}
}

func TestSelect_SetMode_EmptyResult_MD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--fields=$id,title", "--format=md")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Empty MD must still have the header + separator rows but no data rows.
	if !strings.Contains(stdout, "$id") || !strings.Contains(stdout, "title") {
		t.Errorf("empty MD output must include header columns, got:\n%s", stdout)
	}
	// Data row count: lines starting with "|" minus header (1) minus separator (1).
	dataLines := 0
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.HasPrefix(line, "|") {
			dataLines++
		}
	}
	if dataLines > 2 {
		t.Errorf("empty MD should have only header+separator (2 lines), got %d pipe lines:\n%s", dataLines, stdout)
	}
}

func TestSelect_SetMode_INGR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "a", map[string]any{"title": "Alpha", "priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "test.items", "b", map[string]any{"title": "Beta", "priority": float64(2)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--fields=$id,title,priority", "--format=ingr")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 2 records") {
		t.Errorf("missing record-count footer:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"Alpha"`) || !strings.Contains(stdout, `"Beta"`) {
		t.Errorf("missing JSON-encoded titles:\n%s", stdout)
	}
}

func TestSelect_SetMode_INGR_EmptyResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--fields=$id,title", "--format=ingr")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.HasPrefix(stdout, "# INGR.io | select: ") {
		t.Errorf("missing INGR header in empty-result output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "# 0 records") {
		t.Errorf("empty INGR must have '# 0 records' footer:\n%s", stdout)
	}
}

func TestSelect_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--id=test.items/missing")
	if err == nil {
		t.Fatal("expected error when record not found")
	}
	if !strings.Contains(err.Error(), "test.items/missing") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should name the missing id, got: %v", err)
	}
}

func TestSelect_SetMode_OrderByThenLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "a", map[string]any{"priority": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "test.items", "b", map[string]any{"priority": float64(5)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := seedRecord(t, dir, "test.items", "c", map[string]any{"priority": float64(3)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items",
		"--order-by=-priority", "--limit=1",
		"--fields=$id,priority", "--format=csv",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("want header + 1 data row, got %d lines:\n%s", len(lines), stdout)
	}
	// Highest priority first; --limit=1 yields only "b".
	if !strings.Contains(lines[1], "b") {
		t.Errorf("expected 'b' (priority=5) as the single row, got: %s", lines[1])
	}
}

func TestSelect_SetMode_LimitValidation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--limit=-1")
	if err == nil {
		t.Fatal("expected error for --limit=-1")
	}
}

func TestSelect_SetMode_MinAffected_Met(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for _, k := range []string{"a", "b", "c"} {
		if err := seedRecord(t, dir, "test.items", k, map[string]any{"x": float64(1)}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--min-affected=2", "--format=yaml")
	if err != nil {
		t.Fatalf("expected success (3 >= 2), got: %v", err)
	}
	if !strings.Contains(stdout, "x: 1") {
		t.Errorf("expected output, got:\n%s", stdout)
	}
}

func TestSelect_SetMode_MinAffected_Unmet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	if err := seedRecord(t, dir, "test.items", "a", map[string]any{"x": float64(1)}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=test.items", "--min-affected=5")
	if err == nil {
		t.Fatalf("expected error (1 < 5), got nil. stdout: %s", stdout)
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "5") {
		t.Errorf("error should name actual=1 and required=5, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout MUST be empty when threshold unmet, got: %s", stdout)
	}
}

func TestSelect_PathAndGitHubMutuallyExclusive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--remote=github.com/foo/bar", "--from=test.items")
	if err == nil {
		t.Fatal("expected error when both --path and --remote supplied")
	}
	if !strings.Contains(err.Error(), "--path") || !strings.Contains(err.Error(), "--remote") {
		t.Errorf("error should mention --path and --remote, got: %v", err)
	}
}

func TestSelect_SetMode_GitHubFlagAccepted(t *testing.T) {
	// Not parallel: replaces package-level gitHubFileReaderFactory.
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	// Smoke test: --from + --remote parses and branches correctly before any
	// network call. We inject a reader that returns a network-style error so
	// the test is hermetic and confirms the GitHub branch is wired — not just
	// that flag parsing succeeds.
	origFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = &stubFileReaderFactory{err: fmt.Errorf("github: network error")}
	defer func() { gitHubFileReaderFactory = origFactory }()

	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--remote=github.com/owner/repo", "--from=test.items")
	if err == nil {
		t.Fatal("expected error from injected GitHub factory failure")
	}
	if strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("GitHub set-mode branch not wired: %v", err)
	}
	// Error must mention the injected message, confirming the GitHub path was taken.
	if !strings.Contains(err.Error(), "github") && !strings.Contains(err.Error(), "network") {
		t.Errorf("error should come from GitHub path, got: %v", err)
	}
}

// stubFileReaderFactory returns an error from NewGitHubFileReader without any mock framework.
type stubFileReaderFactory struct {
	err error
}

func (s *stubFileReaderFactory) NewGitHubFileReader(_ dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
	return nil, s.err
}

func TestSelect_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	for i, key := range []string{"low", "mid", "high"} {
		pri := float64((i + 1) * 10)
		if err := seedRecord(t, dir, "test.items", key, map[string]any{
			"priority": pri,
			"title":    "T-" + key,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items",
		"--where=priority>=20",
		"--order-by=-priority",
		"--fields=$id,priority,title",
		"--format=json",
		"--min-affected=1",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Result should be JSON array with two rows ordered high → mid.
	if !strings.Contains(stdout, `"$id": "high"`) {
		t.Errorf("missing $id:high in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"$id": "mid"`) {
		t.Errorf("missing $id:mid in output:\n%s", stdout)
	}
	if strings.Contains(stdout, `"$id": "low"`) {
		t.Errorf("unexpected $id:low (priority=10 should be filtered out):\n%s", stdout)
	}
	idxHigh := strings.Index(stdout, `"$id": "high"`)
	idxMid := strings.Index(stdout, `"$id": "mid"`)
	if idxHigh > idxMid {
		t.Errorf("expected high before mid (descending priority), got high@%d, mid@%d", idxHigh, idxMid)
	}
}
