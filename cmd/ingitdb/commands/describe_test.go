package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// stdoutMu serialises captureStdout across the parallel tests below.
// They each swap os.Stdout temporarily; concurrent swaps would race.
var stdoutMu sync.Mutex

var ingitdbValidatorReadDef = func(p string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return validator.ReadDefinition(p, opts...)
}

func TestDescribe_ReturnsCommand(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	if cmd == nil {
		t.Fatal("Describe() returned nil")
	}
	if cmd.Use != "describe <kind> <name>" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
	gotAliases := cmd.Aliases
	if len(gotAliases) != 1 || gotAliases[0] != "desc" {
		t.Errorf("expected desc alias; got %v", gotAliases)
	}
	if len(cmd.Commands()) != 2 {
		t.Fatalf("expected 2 subcommands (collection, view); got %d", len(cmd.Commands()))
	}
}

func TestDescribe_RootRequiresKind(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when invoked without a kind")
	}
}

// describeFixtureDB builds an on-disk database with one or more
// collections; each collection has an empty $views/ dir unless views
// is non-empty. Returns the absolute root dir.
func describeFixtureDB(t *testing.T, collections map[string]*ingitdb.CollectionDef, views map[string]map[string]*ingitdb.ViewDef) string {
	t.Helper()
	root := t.TempDir()
	// Write .ingitdb/root-collections.yaml
	rootColls := make(map[string]string)
	for id := range collections {
		rootColls[id] = id
	}
	if err := os.MkdirAll(filepath.Join(root, ".ingitdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	rcBytes, _ := yaml.Marshal(rootColls)
	if err := os.WriteFile(filepath.Join(root, ".ingitdb", "root-collections.yaml"), rcBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// Write each collection's .collection/definition.yaml + view files
	for id, def := range collections {
		colDir := filepath.Join(root, id)
		if err := os.MkdirAll(filepath.Join(colDir, ".collection"), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, _ := yaml.Marshal(def)
		if err := os.WriteFile(filepath.Join(colDir, ".collection", "definition.yaml"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		for vName, vDef := range views[id] {
			viewsDir := filepath.Join(colDir, "$views")
			if err := os.MkdirAll(viewsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			vRaw, _ := yaml.Marshal(vDef)
			if err := os.WriteFile(filepath.Join(viewsDir, vName+".yaml"), vRaw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	stdoutMu.Lock()
	defer stdoutMu.Unlock()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

func TestDescribeCollection_LocalYAML_Shape(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
			},
		},
	}, nil)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		if err := runCobraCommand(cmd, "collection", "users", "--path="+dir); err != nil {
			t.Fatalf("collection users: %v", err)
		}
	})
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse yaml: %v\nout:\n%s", err, out)
	}
	if _, ok := parsed["definition"]; !ok {
		t.Errorf("missing definition key")
	}
	meta, ok := parsed["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing _meta map; got %T", parsed["_meta"])
	}
	checks := map[string]any{
		"id":              "users",
		"kind":            "collection",
		"definition_path": "users",
		"data_path":       "users",
	}
	for k, want := range checks {
		if meta[k] != want {
			t.Errorf("_meta.%s = %v; want %v", k, meta[k], want)
		}
	}
}

func TestDescribeCollection_TableAliasEquivalent(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	collOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	tableOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "table", "users", "--path="+dir)
	})
	if collOut != tableOut {
		t.Errorf("table alias produced different output:\n--collection--\n%s\n--table--\n%s", collOut, tableOut)
	}
}

func TestDescribeCollection_NotFound(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "widgets", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `collection "widgets" not found`) {
		t.Fatalf("want not-found error; got: %v", err)
	}
}

func TestDescribeCollection_JSONFormat(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=json")
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse json: %v\nout:\n%s", err, out)
	}
	if _, ok := parsed["definition"]; !ok {
		t.Errorf("json missing definition key")
	}
	if _, ok := parsed["_meta"]; !ok {
		t.Errorf("json missing _meta key")
	}
}

func TestDescribeCollection_SQLFormatErrors(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=sql")
	if err == nil || !strings.Contains(err.Error(),
		`engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`) {
		t.Fatalf("want SQL-on-ingitdb error; got: %v", err)
	}
}

func TestDescribeCollection_NativeResolvesToYAML(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	yamlOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=yaml")
	})
	nativeOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=native")
	})
	if yamlOut != nativeOut {
		t.Errorf("--format=native ≠ --format=yaml on ingitdb")
	}
}
