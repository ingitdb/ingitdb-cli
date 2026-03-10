package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// testQueryNewDB returns a newDB seam backed by dalgo2fsingitdb.
func testQueryNewDB(t *testing.T) func(string, *ingitdb.Definition) (dal.DB, error) {
	t.Helper()
	return func(path string, def *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(path, def)
	}
}

// writeTestYAML writes a YAML fixture file for query tests.
func writeTestYAML(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
}

func TestQuery_ReturnsCommand(t *testing.T) {
	t.Parallel()

	noop := func(...any) {}
	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return "/tmp", nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return &ingitdb.Definition{}, nil },
		func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil },
		noop,
	)
	if cmd == nil {
		t.Fatal("Query() returned nil")
	}
	if cmd.Name != "query" {
		t.Errorf("expected name 'query', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}

	// Verify all expected flags are present.
	flagNames := make(map[string]bool)
	for _, f := range cmd.Flags {
		for _, name := range f.Names() {
			flagNames[name] = true
		}
	}
	for _, required := range []string{"collection", "c", "fields", "f", "where", "w", "order-by", "format", "path"} {
		if !flagNames[required] {
			t.Errorf("expected flag %q to be present", required)
		}
	}
}

func TestQuery_MissingCollection(t *testing.T) {
	t.Parallel()

	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return t.TempDir(), nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return &ingitdb.Definition{}, nil },
		func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil },
		func(...any) {},
	)
	err := runCLICommand(cmd) // no --collection flag
	if err == nil {
		t.Fatal("expected error when --collection is missing")
	}
}

func TestQuery_UnknownCollection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return dir, nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil },
		func(...any) {},
	)
	err := runCLICommand(cmd, "--collection=nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestQuery_Integration_CSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestYAML(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha"})
	writeTestYAML(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta"})

	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return dir, nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		testQueryNewDB(t),
		func(...any) {},
	)
	err := runCLICommand(cmd, "--collection=test.items", "--fields=$id")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
}

func TestQuery_Integration_InvalidWhereExpr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return dir, nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		testQueryNewDB(t),
		func(...any) {},
	)
	err := runCLICommand(cmd, "--collection=test.items", "--where=badexpr")
	if err == nil {
		t.Fatal("expected error for malformed where expression")
	}
}

func TestQuery_Integration_InvalidFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return dir, nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		testQueryNewDB(t),
		func(...any) {},
	)
	err := runCLICommand(cmd, "--collection=test.items", "--format=xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestQuery_Integration_UppercaseFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestYAML(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha"})

	newDB := testQueryNewDB(t)
	for _, format := range []string{"CSV", "JSON", "YAML", "MD"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			cmd := Query(
				func() (string, error) { return "/home/test", nil },
				func() (string, error) { return dir, nil },
				func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
				newDB,
				func(...any) {},
			)
			err := runCLICommand(cmd, "--collection=test.items", "--fields=$id", "--format="+format)
			if err != nil {
				t.Errorf("format %q should be accepted, got error: %v", format, err)
			}
		})
	}
}

func TestQuery_Integration_Where(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeTestYAML(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha"})
	writeTestYAML(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta"})

	cmd := Query(
		func() (string, error) { return "/home/test", nil },
		func() (string, error) { return dir, nil },
		func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		testQueryNewDB(t),
		func(...any) {},
	)
	err := runCLICommand(cmd, "--collection=test.items", "--fields=$id,name", "--where=name==Alpha")
	if err != nil {
		t.Fatalf("query with where failed: %v", err)
	}
}
