package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func yamlUnmarshalForTest(data []byte, out any) error {
	return yaml.Unmarshal(data, out)
}

func yamlMarshalForTest(in any) ([]byte, error) {
	return yaml.Marshal(in)
}

// dropTestDeps returns a minimal DI set for the Drop command.
func dropTestDeps(t *testing.T, dir string) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return dir, nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB = func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf = func(...any) {}
	return
}

// runDropCmd invokes the Drop command with the given args and returns
// captured stdout + any error.
func runDropCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestDrop_RequiresKindAndName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "no args at all", args: []string{"--path=" + dir}},
		{name: "only kind, no name", args: []string{"--path=" + dir, "collection"}},
		{name: "unknown kind", args: []string{"--path=" + dir, "unknownkind", "x"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf, tc.args...)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}

func TestDrop_RegistersIfExistsAndCascade(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)

	// PersistentFlags must include --if-exists and --cascade so that
	// both subcommands inherit them.
	for _, name := range []string{"if-exists", "cascade", "path"} {
		if cmd.PersistentFlags().Lookup(name) == nil && cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered on drop", name)
		}
	}
}

// seedCollection creates a minimal collection directory at
// <dbDir>/.collections/<name>/ with a definition.yaml and a data
// file, plus a root-collections.yaml entry. Returns the absolute
// collection directory path.
func seedCollection(t *testing.T, dbDir, name string) string {
	t.Helper()
	colDir := filepath.Join(dbDir, ".collections", name)
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir colDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "definition.yaml"), []byte("columns:\n  id: {}\n"), 0o644); err != nil {
		t.Fatalf("write definition.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "sample.yaml"), []byte("id: 1\n"), 0o644); err != nil {
		t.Fatalf("write sample.yaml: %v", err)
	}
	// Update root-collections.yaml — append or create.
	rootDir := filepath.Join(dbDir, ".ingitdb")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("mkdir .ingitdb: %v", err)
	}
	rootPath := filepath.Join(rootDir, "root-collections.yaml")
	var current map[string]string
	if existing, readErr := os.ReadFile(rootPath); readErr == nil {
		_ = yamlUnmarshalForTest(existing, &current)
	}
	if current == nil {
		current = map[string]string{}
	}
	current[name] = ".collections/" + name
	out, _ := yamlMarshalForTest(current)
	if err := os.WriteFile(rootPath, out, 0o644); err != nil {
		t.Fatalf("write root-collections.yaml: %v", err)
	}
	return colDir
}

func TestDrop_Collection_RemovesDataAndSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	colDir := seedCollection(t, dir, "cities")

	stdout, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "collection", "cities",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("drop should be silent on stdout, got: %q", stdout)
	}
	// Data side: directory gone.
	if _, statErr := os.Stat(colDir); !os.IsNotExist(statErr) {
		t.Errorf("expected collection directory gone, got stat err: %v", statErr)
	}
	// Schema side: entry gone from root-collections.yaml.
	root, readErr := os.ReadFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"))
	if readErr != nil {
		t.Fatalf("read root-collections: %v", readErr)
	}
	if strings.Contains(string(root), "cities") {
		t.Errorf("expected cities entry removed, got:\n%s", string(root))
	}
}

func TestDrop_Collection_NotFoundError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	// Seed an unrelated collection so .ingitdb/root-collections.yaml exists.
	seedCollection(t, dir, "other")

	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "collection", "nonexistent",
	)
	if err == nil {
		t.Fatal("expected error when collection does not exist")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should name the missing collection, got: %v", err)
	}
}

func TestDrop_Collection_PreservesOtherEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	seedCollection(t, dir, "alpha")
	bravoDir := seedCollection(t, dir, "bravo")

	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "collection", "alpha",
	)
	if err != nil {
		t.Fatalf("drop alpha: %v", err)
	}
	// bravo's directory must still exist.
	if _, statErr := os.Stat(bravoDir); statErr != nil {
		t.Errorf("expected bravo directory preserved, got: %v", statErr)
	}
	// bravo's entry must still be in root-collections.yaml.
	root, _ := os.ReadFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"))
	if !strings.Contains(string(root), "bravo") {
		t.Errorf("expected bravo entry preserved, got:\n%s", string(root))
	}
}

// seedView creates a view file under <colDir>/$views/<viewName>.yaml
// with optional FileName metadata. If outputContent is non-empty,
// also creates the materialized file at <colDir>/<outputRel>.
func seedView(t *testing.T, colDir, viewName, outputRel, outputContent string) string {
	t.Helper()
	viewsDir := filepath.Join(colDir, "$views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir $views: %v", err)
	}
	viewYAML := "id: " + viewName + "\n"
	if outputRel != "" {
		viewYAML += "file_name: " + outputRel + "\n"
	}
	viewPath := filepath.Join(viewsDir, viewName+".yaml")
	if err := os.WriteFile(viewPath, []byte(viewYAML), 0o644); err != nil {
		t.Fatalf("write view: %v", err)
	}
	if outputContent != "" && outputRel != "" {
		outputPath := filepath.Join(colDir, outputRel)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			t.Fatalf("mkdir output dir: %v", err)
		}
		if err := os.WriteFile(outputPath, []byte(outputContent), 0o644); err != nil {
			t.Fatalf("write output: %v", err)
		}
	}
	return viewPath
}

func TestDrop_View_RemovesViewFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	colDir := seedCollection(t, dir, "cities")
	viewPath := seedView(t, colDir, "active_cities", "", "")

	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "view", "active_cities",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if _, statErr := os.Stat(viewPath); !os.IsNotExist(statErr) {
		t.Errorf("expected view file gone, got stat err: %v", statErr)
	}
}

func TestDrop_View_RemovesMaterializedOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	colDir := seedCollection(t, dir, "cities")
	outputRel := "active_cities.csv"
	viewPath := seedView(t, colDir, "active_cities", outputRel, "name,population\n")

	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "view", "active_cities",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if _, statErr := os.Stat(viewPath); !os.IsNotExist(statErr) {
		t.Errorf("expected view file gone, got stat err: %v", statErr)
	}
	outputPath := filepath.Join(colDir, outputRel)
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Errorf("expected materialized output gone, got stat err: %v", statErr)
	}
}

func TestDrop_View_NotFoundError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	seedCollection(t, dir, "cities") // exists but has no views

	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "view", "nonexistent",
	)
	if err == nil {
		t.Fatal("expected error when view does not exist")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should name the missing view, got: %v", err)
	}
}
