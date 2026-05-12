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
	for _, name := range []string{"id", "from", "where", "order-by", "fields", "limit", "min-affected", "format", "path", "github"} {
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
