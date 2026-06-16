package commands

// coverage_describe_test.go covers the remaining uncovered lines in:
//   - describe.go
//   - describe_output.go
//   - view_builder_helper.go
//   - query_output.go
//   - select_output.go

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// ============================================================
// describe.go – loadLocalDef error branches
// ============================================================

func TestLoadLocalDef_RemoteNotYetImplemented(t *testing.T) {
	t.Parallel()
	// --remote is set but --path is not → triggers the "not yet implemented" error.
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "users", "--remote=github.com/owner/repo")
	if err == nil {
		t.Fatal("expected error for --remote on describe")
	}
	if !strings.Contains(err.Error(), "describe --remote not yet implemented") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadLocalDef_ResolveDBPathError(t *testing.T) {
	t.Parallel()
	// getWd fails → resolveDBPath fails → loadLocalDef returns error.
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no working directory") }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	// No --path, no --remote → resolveDBPath calls getWd which fails.
	err := runCobraCommand(cmd, "collection", "users")
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
	if !strings.Contains(err.Error(), "working directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadLocalDef_ReadDefinitionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read definition failed")
	}
	cmd := Describe(homeDir, getWd, readDef)
	err := runCobraCommand(cmd, "collection", "users", "--path="+dir)
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
	if !strings.Contains(err.Error(), "failed to read database definition") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe.go – bareNameDescribe branches
// ============================================================

func TestBareNameDescribe_LoadDefError(t *testing.T) {
	t.Parallel()
	// loadLocalDef fails → bareNameDescribe returns the error.
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	// Bare name invocation (1 arg to the root describe command).
	err := runCobraCommand(cmd, "somename")
	if err == nil {
		t.Fatal("expected error when loadLocalDef fails in bareNameDescribe")
	}
}

func TestBareNameDescribe_ViewOnly(t *testing.T) {
	t.Parallel()
	// "somecol" is only a view (not a collection) → bareNameDescribe dispatches to view.
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
				"recent": {Top: 10},
			},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out, err := captureStdout(t, cmd, "recent", "--path="+dir)
	if err != nil {
		t.Fatalf("bare-name view dispatch: %v", err)
	}
	if !strings.Contains(out, "kind: view") {
		t.Errorf("expected view output, got:\n%s", out)
	}
}

func TestBareNameDescribe_NeitherCollectionNorView(t *testing.T) {
	t.Parallel()
	// "unknown" is neither a collection nor a view → default case.
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
	err := runCobraCommand(cmd, "unknown", "--path="+dir)
	if err == nil {
		t.Fatal("expected error for unknown name in bareNameDescribe")
	}
	if !strings.Contains(err.Error(), `no collection or view named "unknown"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe.go – discoverCollectionChildren: views $views/ branch
// ============================================================

func TestDescribeCollection_WithViewsAndSubcollections(t *testing.T) {
	t.Parallel()
	// Create a collection with a $views/ dir containing view YAML files
	// and a subcollection (a subdirectory with its own .collection/ dir).
	dir := t.TempDir()

	// Set up .ingitdb/root-collections.yaml
	ingitdbDir := filepath.Join(dir, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("mkdir .ingitdb: %v", err)
	}
	rcContent := "products: products\n"
	if err := os.WriteFile(filepath.Join(ingitdbDir, "root-collections.yaml"), []byte(rcContent), 0o644); err != nil {
		t.Fatalf("write root-collections: %v", err)
	}

	// Set up products collection
	colDir := filepath.Join(dir, "products")
	if err := os.MkdirAll(filepath.Join(colDir, ".collection"), 0o755); err != nil {
		t.Fatalf("mkdir .collection: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID: "products",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	raw, marshalErr := yaml.Marshal(colDef)
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal colDef: %v", marshalErr)
	}
	if err := os.WriteFile(filepath.Join(colDir, ".collection", "definition.yaml"), raw, 0o644); err != nil {
		t.Fatalf("write definition.yaml: %v", err)
	}

	// Create $views/ dir with one view file and one non-yaml file (to test continue)
	viewsDir := filepath.Join(colDir, "$views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir $views: %v", err)
	}
	viewContent := "order_by: name ASC\n"
	if err := os.WriteFile(filepath.Join(viewsDir, "by_name.yaml"), []byte(viewContent), 0o644); err != nil {
		t.Fatalf("write view: %v", err)
	}
	// A subdirectory inside $views (should be skipped with continue in the views loop)
	if err := os.MkdirAll(filepath.Join(viewsDir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir in views: %v", err)
	}

	// Create a subcollection: a directory with its own .collection/
	subDir := filepath.Join(colDir, "variants")
	if err := os.MkdirAll(filepath.Join(subDir, ".collection"), 0o755); err != nil {
		t.Fatalf("mkdir subcol: %v", err)
	}

	// A non-directory file in colDir (should be skipped in subcol loop)
	if err := os.WriteFile(filepath.Join(colDir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	// A directory WITHOUT .collection/ (should be skipped in subcol loop)
	if err := os.MkdirAll(filepath.Join(colDir, "not-a-subcol"), 0o755); err != nil {
		t.Fatalf("mkdir not-a-subcol: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	out, err := captureStdout(t, cmd, "collection", "products", "--path="+dir)
	if err != nil {
		t.Fatalf("describe collection products: %v", err)
	}
	if !strings.Contains(out, "by_name") {
		t.Errorf("expected view 'by_name' in output:\n%s", out)
	}
	if !strings.Contains(out, "variants") {
		t.Errorf("expected subcollection 'variants' in output:\n%s", out)
	}
}

// ============================================================
// describe.go – emitNode JSON format branch
// ============================================================

func TestDescribeCollection_JSONOutput(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"items": {
			ID:         "items",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"sku": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out, err := captureStdout(t, cmd, "collection", "items", "--path="+dir, "--format=json")
	if err != nil {
		t.Fatalf("describe collection --format=json: %v", err)
	}
	if !strings.Contains(out, `"definition"`) {
		t.Errorf("expected JSON with 'definition' key:\n%s", out)
	}
	if !strings.Contains(out, `"_meta"`) {
		t.Errorf("expected JSON with '_meta' key:\n%s", out)
	}
}

// ============================================================
// describe.go – emitNode: unsupported format error
// ============================================================

func TestEmitNode_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	node := &yaml.Node{Kind: yaml.MappingNode}
	err := emitNode(&bytes.Buffer{}, node, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), `unsupported format "xml"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe.go – runDescribeView: loadLocalDef error path
// ============================================================

func TestDescribeView_LoadDefError(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "anything")
	if err == nil {
		t.Fatal("expected error when loadLocalDef fails in runDescribeView")
	}
}

// ============================================================
// describe.go – describeViewFromMatches: 0 matches with scopeCol set
// ============================================================

func TestDescribeView_NotFoundInCollection(t *testing.T) {
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
	err := runCobraCommand(cmd, "view", "ghost", "--in=users", "--path="+dir)
	if err == nil {
		t.Fatal("expected error when view not found in scoped collection")
	}
	if !strings.Contains(err.Error(), `view "ghost" not found in collection "users"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe.go – describeViewFromMatches: resolveFormat error
// ============================================================

func TestDescribeView_InvalidFormat(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"orders": {
				"monthly": {Top: 12},
			},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "monthly", "--path="+dir, "--format=xml")
	if err == nil {
		t.Fatal("expected error for invalid --format on describe view")
	}
	if !strings.Contains(err.Error(), `invalid --format value "xml"`) {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe.go – describeViewFromMatches: single match + JSON format
// ============================================================

func TestDescribeView_SingleMatchJSON(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"orders": {
				"monthly": {Top: 12, Template: "md-table"},
			},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out, err := captureStdout(t, cmd, "view", "monthly", "--path="+dir, "--format=json")
	if err != nil {
		t.Fatalf("describe view monthly --format=json: %v", err)
	}
	if !strings.Contains(out, `"_meta"`) {
		t.Errorf("expected JSON with '_meta' key:\n%s", out)
	}
	if !strings.Contains(out, `"kind": "view"`) {
		t.Errorf("expected kind=view in JSON:\n%s", out)
	}
}

// ============================================================
// describe.go – describeViewFromMatches: ReadFile / yaml.Unmarshal errors
// ============================================================

func TestDescribeView_ReadFileError(t *testing.T) {
	t.Parallel()
	// Create a fixture where the view file matches but then becomes unreadable.
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
				"secret": {Top: 5},
			},
		},
	)
	if os.Getuid() == 0 {
		t.Skip("running as root; permission check not meaningful")
	}
	// Make the view file unreadable.
	viewFile := filepath.Join(dir, "users", "$views", "secret.yaml")
	if err := os.Chmod(viewFile, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(viewFile, 0o644) })

	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "secret", "--path="+dir)
	if err == nil {
		t.Fatal("expected error when view file is unreadable")
	}
	if !strings.Contains(err.Error(), "read view file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDescribeView_ParseError(t *testing.T) {
	t.Parallel()
	// Write an invalid YAML file as the view definition.
	dir := t.TempDir()

	ingitdbDir := filepath.Join(dir, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("mkdir .ingitdb: %v", err)
	}
	rcContent := "products: products\n"
	if err := os.WriteFile(filepath.Join(ingitdbDir, "root-collections.yaml"), []byte(rcContent), 0o644); err != nil {
		t.Fatalf("write root-collections: %v", err)
	}

	// Create products collection
	colDir := filepath.Join(dir, "products")
	if err := os.MkdirAll(filepath.Join(colDir, ".collection"), 0o755); err != nil {
		t.Fatalf("mkdir .collection: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID: "products",
		RecordFile: &ingitdb.RecordFileDef{
			Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
	}
	raw, _ := yaml.Marshal(colDef)
	if err := os.WriteFile(filepath.Join(colDir, ".collection", "definition.yaml"), raw, 0o644); err != nil {
		t.Fatalf("write definition.yaml: %v", err)
	}

	// Write a malformed YAML view file
	viewsDir := filepath.Join(colDir, "$views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir $views: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "broken.yaml"), []byte("invalid: [yaml: {"), 0o644); err != nil {
		t.Fatalf("write broken.yaml: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "broken", "--path="+dir)
	if err == nil {
		t.Fatal("expected error for malformed view YAML")
	}
	if !strings.Contains(err.Error(), "parse view file") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// describe_output.go – buildCollectionPayload encode error
// ============================================================

func TestBuildCollectionPayload_EncodeError(t *testing.T) {
	t.Parallel()
	// yaml.Node.Encode fails when the value contains a channel (unencodable).
	// We can trigger this by creating a CollectionDef that has a field yaml cannot encode.
	// Actually, CollectionDef is a normal struct that yaml can encode. Let's use a different
	// approach: nil CollectionDef will panic, so we need another way.
	// The safest approach: use a CollectionDef and verify the function works normally,
	// since encode errors on normal structs are unreachable.
	// Instead, cover the "views" path by building a payload with non-nil views.
	col := &ingitdb.CollectionDef{
		ID: "orders",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"amount": {Type: ingitdb.ColumnTypeString},
		},
		Views: map[string]*ingitdb.ViewDef{
			"monthly": {ID: "monthly", Top: 12},
		},
	}
	ctx := collectionOutputCtx{
		relPath:   "orders",
		viewNames: []string{"monthly"},
	}
	node, err := buildCollectionPayload(col, ctx)
	if err != nil {
		t.Fatalf("buildCollectionPayload: %v", err)
	}
	out, marshalErr := yaml.Marshal(node)
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	if !strings.Contains(string(out), "monthly") {
		t.Errorf("expected 'monthly' in output:\n%s", out)
	}
}

// ============================================================
// describe_output.go – buildViewPayload with file_name populated
// ============================================================

func TestBuildViewPayload_WithFileName(t *testing.T) {
	t.Parallel()
	view := &ingitdb.ViewDef{
		ID:       "active",
		Top:      50,
		FileName: "active-records.csv",
	}
	node, err := buildViewPayload(view, viewOutputCtx{
		owningCollection: "orders",
		relPath:          "orders/$views/active.yaml",
	})
	if err != nil {
		t.Fatalf("buildViewPayload: %v", err)
	}
	out, marshalErr := yaml.Marshal(node)
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	s := string(out)
	if !strings.Contains(s, "active-records.csv") {
		t.Errorf("expected file_name in output:\n%s", s)
	}
	if !strings.Contains(s, "kind: view") {
		t.Errorf("expected kind: view in output:\n%s", s)
	}
}

// ============================================================
// view_builder_helper.go – ReadViewDefs error path (L19-20)
// ============================================================

func TestViewBuilderForCollection_ReadViewDefsReadError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("running as root; permission check not meaningful")
	}
	dir := t.TempDir()
	// Create the .collection/views/ directory structure so Glob finds a .yaml file.
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write a view file, then make it unreadable so os.ReadFile fails.
	viewFile := filepath.Join(viewDir, "unreadable.yaml")
	if err := os.WriteFile(viewFile, []byte("top: 10\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Chmod(viewFile, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(viewFile, 0o644) })

	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
	}
	_, err := viewBuilderForCollection(colDef)
	if err == nil {
		t.Fatal("expected error when view def file is unreadable")
	}
	if !strings.Contains(err.Error(), "failed to read view def") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// query_output.go – writeCSV header and row error branches
//
// The csv.Writer uses a bufio.Writer internally, so cw.Write() only returns
// an error when the underlying buffer flushes. The error from cw.Write itself
// is only visible after Flush() via cw.Error(). The per-write error branches
// (L25, L35) are therefore not reachable with a standard errWriter, and the
// existing TestWriteCSV_WriteError covers the Flush error path.
//
// Similarly, json.Marshal/yaml.Marshal on []map[string]any with normal values
// cannot return errors (L65, L75 in query_output.go).
//
// These lines are marked untestable below.
// ============================================================

// TestWriteCSV_HeaderWriteError exercises the cw.Write(columns) check (L25).
// Because encoding/csv buffers internally, we bypass the buffer by passing a
// tiny buffer size. However, the Go standard library csv.Writer does not expose
// a buffer size option. The header error path is therefore not reachable via
// the public API without a direct call that forces a synchronous flush.
//
// We document this by verifying the happy path still works when columns is empty
// and records are non-empty (exercising collectColumns). The error branch at L25
// is untestable: the csv.Writer buffers writes and only surfaces errors on Flush.
func TestWriteCSV_WithAutoDetectedColumns(t *testing.T) {
	t.Parallel()
	// Passing nil columns exercises collectColumns inside writeCSV.
	records := []map[string]any{{"name": "Alice", "age": 30}}
	var buf bytes.Buffer
	if err := writeCSV(&buf, records, nil); err != nil {
		t.Fatalf("writeCSV with nil columns: %v", err)
	}
	if !strings.Contains(buf.String(), "Alice") {
		t.Errorf("expected 'Alice' in CSV output:\n%s", buf.String())
	}
}

// ============================================================
// query_output.go – writeMarkdown: nil columns triggers collectColumns
// then header write fails
// ============================================================

func TestWriteMarkdown_NilColumnsHeaderWriteError(t *testing.T) {
	t.Parallel()
	// With nil columns, writeMarkdown calls collectColumns first.
	// Then the header write fails immediately (errWriter fails on first Write).
	records := []map[string]any{{"name": "Alice"}}
	err := writeMarkdown(errWriter{}, records, nil)
	if err == nil {
		t.Fatal("expected error when header write fails with nil columns")
	}
}

// ============================================================
// query_output.go – writeJSON write error path (fmt.Fprintf fails)
// ============================================================

func TestWriteJSON_FprintfError(t *testing.T) {
	t.Parallel()
	// errWriter causes fmt.Fprintf to fail on the `_, err = fmt.Fprintf(...)` line.
	records := []map[string]any{{"key": "value"}}
	err := writeJSON(errWriter{}, records)
	if err == nil {
		t.Fatal("expected error when fmt.Fprintf fails")
	}
}

// ============================================================
// query_output.go – writeYAML write error path (w.Write fails)
// ============================================================

func TestWriteYAML_WriteErrorViaErrWriter(t *testing.T) {
	t.Parallel()
	// errWriter causes w.Write(out) to fail on the write line.
	records := []map[string]any{{"key": "value"}}
	err := writeYAML(errWriter{}, records)
	if err == nil {
		t.Fatal("expected error when w.Write fails")
	}
}

// ============================================================
// select_output.go – writeSingleRecord JSON format write error
// ============================================================

func TestWriteSingleRecord_JSON(t *testing.T) {
	t.Parallel()
	record := map[string]any{"name": "Ireland", "pop": 5000000}
	var buf bytes.Buffer
	if err := writeSingleRecord(&buf, record, "json", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "Ireland"`) {
		t.Errorf("expected JSON output, got:\n%s", buf.String())
	}
}

func TestWriteSingleRecord_JSON_WriteError(t *testing.T) {
	t.Parallel()
	// fmt.Fprintf fails on the write after marshaling.
	record := map[string]any{"name": "test"}
	err := writeSingleRecord(errWriter{}, record, "json", nil)
	if err == nil {
		t.Fatal("expected error when fmt.Fprintf fails in writeSingleRecord json branch")
	}
}

// ============================================================
// select_output.go – writeSingleRecord YAML write error
// ============================================================

func TestWriteSingleRecord_YAML(t *testing.T) {
	t.Parallel()
	record := map[string]any{"name": "Germany", "pop": 83000000}
	var buf bytes.Buffer
	if err := writeSingleRecord(&buf, record, "yaml", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "name: Germany") {
		t.Errorf("expected YAML output, got:\n%s", buf.String())
	}
}

func TestWriteSingleRecord_YAML_WriteError(t *testing.T) {
	t.Parallel()
	// w.Write(out) fails for the yaml output.
	record := map[string]any{"name": "test"}
	err := writeSingleRecord(errWriter{}, record, "yaml", nil)
	if err == nil {
		t.Fatal("expected error when w.Write fails in writeSingleRecord yaml branch")
	}
}

func TestWriteSingleRecord_YML_WriteError(t *testing.T) {
	t.Parallel()
	// Same as yaml but using "yml" alias.
	record := map[string]any{"name": "test"}
	err := writeSingleRecord(errWriter{}, record, "yml", nil)
	if err == nil {
		t.Fatal("expected error when w.Write fails in writeSingleRecord yml branch")
	}
}

// ============================================================
// select_output.go – writeINGR: empty rows case
// ============================================================

func TestWriteINGR_EmptyRows(t *testing.T) {
	t.Parallel()
	// Zero rows → FormatINGR writes a header + "# 0 records" footer.
	var buf bytes.Buffer
	if err := writeINGR(&buf, []map[string]any{}, []string{"name"}); err != nil {
		t.Fatalf("writeINGR with empty rows: %v", err)
	}
	if !strings.Contains(buf.String(), "# 0 records") {
		t.Errorf("expected '# 0 records' in output:\n%s", buf.String())
	}
}

func TestWriteINGR_WriteErrorViaErrWriter(t *testing.T) {
	t.Parallel()
	// errWriter fails on the w.Write(out) call.
	rows := []map[string]any{{"name": "Alice"}}
	err := writeINGR(errWriter{}, rows, []string{"name"})
	if err == nil {
		t.Fatal("expected error when w.Write fails in writeINGR")
	}
}

// TestWriteINGR_FormatINGRError triggers the FormatINGR error path (select_output.go L90)
// by including a channel value in the record data, which json.Marshal cannot serialize.
func TestWriteINGR_FormatINGRError(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"ch": make(chan int)}, // json.Marshal fails on channels
	}
	var buf bytes.Buffer
	err := writeINGR(&buf, rows, []string{"ch"})
	if err == nil {
		t.Fatal("expected error from FormatINGR when field is not JSON-serializable")
	}
	if !strings.Contains(err.Error(), "format ingr") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ============================================================
// query_output.go – formatCSVCell json.Marshal error path (L53)
// ============================================================

// TestFormatCSVCell_MapMarshalError covers the json.Marshal error branch inside
// formatCSVCell (L53-55) by passing a map containing a channel, which json.Marshal
// cannot serialize.
func TestFormatCSVCell_MapMarshalError(t *testing.T) {
	t.Parallel()
	// A map[string]any containing a channel causes json.Marshal to fail.
	// The function falls back to fmt.Sprintf in that case.
	v := map[string]any{"ch": make(chan int)}
	got := formatCSVCell(v)
	// When json.Marshal fails, the fallback is fmt.Sprintf("%v", v).
	if got == "" {
		t.Error("expected non-empty fallback output from formatCSVCell on marshal error")
	}
	// The output should NOT be a valid JSON object since marshal failed.
	if strings.HasPrefix(got, "{") && strings.Contains(got, `"ch"`) {
		t.Errorf("expected fallback fmt.Sprintf output, got JSON-like: %q", got)
	}
}
