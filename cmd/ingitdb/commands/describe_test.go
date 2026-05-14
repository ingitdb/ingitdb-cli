package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

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

// captureStdout runs cmd as a subcommand under a fresh root and
// returns whatever it wrote via cobra's writer. Avoids mutating
// the process-global os.Stdout, so it's safe under t.Parallel().
func captureStdout(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{
		Use:           "app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(cmd)
	argv := append([]string{cmd.Name()}, args...)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(argv)
	err := root.ExecuteContext(context.Background())
	return buf.String(), err
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
	out, err := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	if err != nil {
		t.Fatalf("collection users: %v", err)
	}
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
	collOut, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	tableOut, _ := captureStdout(t, cmd, "table", "users", "--path="+dir)
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
	out, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir, "--format=json")
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
	yamlOut, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir, "--format=yaml")
	nativeOut, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir, "--format=native")
	if yamlOut != nativeOut {
		t.Errorf("--format=native ≠ --format=yaml on ingitdb")
	}
}

func TestDescribeView_BasicShape(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users": {
				"top_buyers": {OrderBy: "total_spend DESC", Top: 100, Template: "md-table"},
			},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out, err := captureStdout(t, cmd, "view", "top_buyers", "--path="+dir)
	if err != nil {
		t.Fatalf("view top_buyers: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	meta := parsed["_meta"].(map[string]any)
	if meta["kind"] != "view" || meta["collection"] != "users" {
		t.Errorf("unexpected meta: %v", meta)
	}
	if meta["definition_path"] != "users/$views/top_buyers.yaml" {
		t.Errorf("unexpected definition_path: %v", meta["definition_path"])
	}
}

func TestDescribeView_AmbiguousRequiresIn(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users":  {"recent": {Top: 10}},
			"orders": {"recent": {Top: 10}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "recent", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(),
		`view "recent" is ambiguous — exists in collections: [orders, users]; use --in=<collection>`) {
		t.Fatalf("want ambiguous error; got: %v", err)
	}
}

func TestDescribeView_ResolvedByIn(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users":  {"recent": {Top: 10}},
			"orders": {"recent": {Top: 20}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out, _ := captureStdout(t, cmd, "view", "recent", "--in=orders", "--path="+dir)
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	meta := parsed["_meta"].(map[string]any)
	if meta["collection"] != "orders" {
		t.Errorf("want collection=orders; got %v", meta["collection"])
	}
}

func TestDescribeView_InCollectionMissing(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		nil,
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "anything", "--in=ghosts", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `collection "ghosts" (from --in) not found`) {
		t.Fatalf("want missing --in error; got: %v", err)
	}
}

func TestDescribeView_NotFoundAnywhere(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		nil,
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "ghost", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `view "ghost" not found in any collection`) {
		t.Fatalf("want view-not-found error; got: %v", err)
	}
}

func TestDescribeBareName_ResolvesToCollection(t *testing.T) {
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
	bareOut, _ := captureStdout(t, cmd, "users", "--path="+dir)
	collOut, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	if bareOut != collOut {
		t.Errorf("bare-name output differs from explicit collection")
	}
}

func TestDescribeBareName_AmbiguousErrors(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"archive": {
				ID:         "archive",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users": {"archive": {Top: 10}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "archive", "--path="+dir)
	if err == nil ||
		!strings.Contains(err.Error(), `name "archive" is ambiguous`) ||
		!strings.Contains(err.Error(), `'describe collection archive'`) ||
		!strings.Contains(err.Error(), `'describe view archive'`) {
		t.Fatalf("want ambiguous-name error; got: %v", err)
	}
}

func TestDescribeCollection_MutualExclusion(t *testing.T) {
	t.Parallel()
	cmd := Describe(
		func() (string, error) { return "/tmp", nil },
		func() (string, error) { return "/tmp", nil },
		ingitdbValidatorReadDef,
	)
	err := runCobraCommand(cmd, "collection", "users", "--path=/tmp", "--remote=github.com/owner/repo")
	if err == nil || !strings.Contains(err.Error(), "--path and --remote are mutually exclusive") {
		t.Fatalf("want mutual-exclusion error; got: %v", err)
	}
}

func TestDescribeCollection_UnknownFormatRejected(t *testing.T) {
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
	err := runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=xml")
	if err == nil ||
		!strings.Contains(err.Error(), `invalid --format value "xml"`) ||
		!strings.Contains(err.Error(), "valid values: yaml, json, native, sql") {
		t.Fatalf("want unknown-format error; got: %v", err)
	}
}

func TestDescribeCollection_ColumnsOrderRespected_CLI(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
				"name":  {Type: ingitdb.ColumnTypeString},
			},
			ColumnsOrder: []string{"email", "id", "name"},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	first, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	second, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	if first != second {
		t.Errorf("output is not byte-identical across runs")
	}
	columnsBlock := columnsSection(t, first)
	emailIdx := strings.Index(columnsBlock, "email:")
	idIdx := strings.Index(columnsBlock, "id:")
	nameIdx := strings.Index(columnsBlock, "name:")
	if emailIdx < 0 || idIdx <= emailIdx || nameIdx <= idIdx {
		t.Errorf("expected columns ordered email, id, name; got:\n%s", first)
	}
}

// columnsSection extracts the substring of the YAML output between the
// `columns:` key and the next top-level key (`columns_order:` or
// `_meta:`). Used by columns-order CLI tests to disambiguate the
// `name:` field inside record_file from a column named `name`.
func columnsSection(t *testing.T, yamlOut string) string {
	t.Helper()
	start := strings.Index(yamlOut, "columns:")
	if start < 0 {
		t.Fatalf("no columns block in output:\n%s", yamlOut)
	}
	rest := yamlOut[start:]
	for _, marker := range []string{"\n    columns_order:", "\n_meta:"} {
		if before, _, ok := strings.Cut(rest, marker); ok {
			return before
		}
	}
	return rest
}

func TestDescribeCollection_ColumnsOrderAlphaFallback_CLI(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
				"name":  {Type: ingitdb.ColumnTypeString},
			},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	first, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	second, _ := captureStdout(t, cmd, "collection", "users", "--path="+dir)
	if first != second {
		t.Errorf("output is not byte-identical across runs")
	}
	columnsBlock := columnsSection(t, first)
	emailIdx := strings.Index(columnsBlock, "email:")
	idIdx := strings.Index(columnsBlock, "id:")
	nameIdx := strings.Index(columnsBlock, "name:")
	if emailIdx < 0 || idIdx <= emailIdx || nameIdx <= idIdx {
		t.Errorf("expected alphabetical email, id, name; got:\n%s", first)
	}
}
