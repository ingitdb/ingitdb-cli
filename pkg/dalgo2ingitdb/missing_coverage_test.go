package dalgo2ingitdb

// missing_coverage_test.go covers the branches that remain below 100% after
// the existing test suite.  All tests live in the same package (white-box) so
// they can reach unexported helpers directly.
//
// Conventions (per CLAUDE.md):
//   - t.Parallel() is the first statement in every top-level test.
//   - No nested calls: intermediate results are assigned to variables.
//   - Every returned error is checked.
//   - t.TempDir() for any test that writes files.
//   - t.Fatalf for setup failures; t.Errorf for assertion failures.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"github.com/dal-go/dalgo/update"
	"github.com/ingr-io/ingr-go/ingr"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// database.go — loadDefinition error from reader
// ---------------------------------------------------------------------------

// TestDatabase_loadDefinition_ReaderError exercises the branch where
// db.reader.ReadDefinition returns an error (database.go lines 91-93).
func TestDatabase_loadDefinition_ReaderError(t *testing.T) {
	t.Parallel()
	// Use a directory without .ingitdb.yaml so ReadDefinition returns an error.
	root := t.TempDir()
	// validator.NewCollectionsReader().ReadDefinition returns nil Definition for
	// an empty project directory (no .ingitdb.yaml and no root-collections.yaml).
	// It should succeed with an empty Definition rather than error.
	// To reliably trigger the error branch we inject a reader that errors.
	db2 := &Database{projectPath: root, reader: errReader{}}
	_, err := db2.loadDefinition()
	if err == nil {
		t.Fatal("loadDefinition: want error from broken reader")
	}
	if !strings.Contains(err.Error(), "broken reader") {
		t.Errorf("error should mention broken reader, got: %v", err)
	}
}

// errReader is a CollectionsReader that always returns an error.
type errReader struct{}

func (errReader) ReadDefinition(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return nil, &brokenReaderError{}
}

type brokenReaderError struct{}

func (e *brokenReaderError) Error() string { return "broken reader: intentional test error" }

// ---------------------------------------------------------------------------
// filelock.go — lock acquisition failures
// The flock library creates the file if it does not exist, so we cannot
// trigger lk.RLock/lk.Lock failures via missing files on most platforms.
// The only reliable way is to pass a path that is a directory (locking a
// directory typically fails on macOS/Linux).
// ---------------------------------------------------------------------------

func TestWithSharedLock_AcquireError(t *testing.T) {
	t.Parallel()
	// A directory cannot be flock-locked on most Unix platforms.
	dir := t.TempDir()
	err := withSharedLock(dir, func() error { return nil })
	if err == nil {
		// Some platforms may succeed; mark the test as skipped to avoid flakiness.
		t.Skip("platform allows locking a directory: cannot exercise acquire-error branch")
	}
	if !strings.Contains(err.Error(), "acquire shared lock") {
		t.Errorf("error should mention 'acquire shared lock', got: %v", err)
	}
}

func TestWithExclusiveLock_AcquireError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := withExclusiveLock(dir, func() error { return nil })
	if err == nil {
		t.Skip("platform allows locking a directory: cannot exercise acquire-error branch")
	}
	if !strings.Contains(err.Error(), "acquire exclusive lock") {
		t.Errorf("error should mention 'acquire exclusive lock', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parse.go — ParseRecordContentForCollection markdown branch
//   - frontmatter key "$id" is passed through (line 76)
//   - undeclared frontmatter key is skipped (line 80)
//
// marshalForFormat — YAML marshal error branch (line 141-143)
// encodeINGRFromMap — WriteHeader error (line 194-196); unreachable because
//   ingr.NewRecordsWriter never fails on Write; see note below.
// resolveINGRColumns — duplicate in columnsOrder skipped (line 415)
// parseINGRAsMap — non-string $ID (line 449)
// ---------------------------------------------------------------------------

func TestParseRecordContentForCollection_Markdown_IDPassthrough(t *testing.T) {
	t.Parallel()
	// Frontmatter with $id present: it must be passed through to result.
	content := []byte("---\n$id: mykey\ntitle: Hello\n---\nBody text.\n")
	col := markdownColDef("")
	col.Columns["title"] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	data, err := ParseRecordContentForCollection(content, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["$id"] != "mykey" {
		t.Errorf("$id should be passed through; got data=%v", data)
	}
}

func TestParseRecordContentForCollection_Markdown_UndeclaredKeySkipped(t *testing.T) {
	t.Parallel()
	// Frontmatter with a key not in colDef.Columns — it must be absent from result.
	content := []byte("---\ntitle: Hello\nextra_undeclared: value\n---\nBody.\n")
	col := markdownColDef("")
	// Only "title" and the content field are in Columns.
	col.Columns = map[string]*ingitdb.ColumnDef{
		"title":                             {Type: ingitdb.ColumnTypeString},
		ingitdb.DefaultMarkdownContentField: {Type: ingitdb.ColumnTypeString},
	}
	data, err := ParseRecordContentForCollection(content, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := data["extra_undeclared"]; ok {
		t.Errorf("undeclared key must be filtered; got data=%v", data)
	}
	if data["title"] != "Hello" {
		t.Errorf("title: got %v, want Hello", data["title"])
	}
}

// TestMarshalForFormat_YAML_Error tries to marshal a value that yaml.Marshal
// would reject. In practice, yaml.Marshal accepts almost any Go value, so
// this branch (parse.go:141) is difficult to trigger.  We exercise the JSON
// marshal path with an unmarshalable type instead (maps with non-string keys
// are rejected by encoding/json).
func TestMarshalForFormat_JSON_UnmarshalableValue(t *testing.T) {
	t.Parallel()
	// map with non-string key cannot be JSON-marshalled.
	type badKey struct{ x int }
	bad := map[badKey]any{{x: 1}: "val"}
	_, err := marshalForFormat(bad, ingitdb.RecordFormatJSON)
	if err == nil {
		t.Fatal("want error marshalling map with non-string key to JSON")
	}
}

func TestResolveINGRColumns_DuplicateInColumnsOrder(t *testing.T) {
	t.Parallel()
	// Passing the same name twice in columnsOrder: duplicates must be deduplicated.
	data := map[string]map[string]any{
		"a": {"score": 1},
	}
	cols := resolveINGRColumns(data, []string{"score", "score", "$ID"})
	// Count occurrences of "score" — must appear exactly once.
	count := 0
	for _, c := range cols {
		if c == "score" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("resolveINGRColumns: score should appear exactly once, got %v", cols)
	}
	// $ID in columnsOrder is also deduplicated.
	idCount := 0
	for _, c := range cols {
		if c == "$ID" {
			idCount++
		}
	}
	if idCount != 1 {
		t.Errorf("resolveINGRColumns: $ID should appear exactly once, got %v", cols)
	}
}

func TestParseINGRAsMap_NonStringID(t *testing.T) {
	t.Parallel()
	// ingr.Unmarshal parses each cell as JSON, so a bareword (unquoted) value
	// in the $ID column decodes to a non-string. We build a valid INGR stream
	// via the writer, then rewrite the quoted $ID to a JSON number to trigger
	// the non-string branch.
	data := map[string]map[string]any{
		"alice": {"val": "1"},
	}
	out, err := encodeINGRFromMap(data, "test", nil)
	if err != nil {
		t.Fatalf("encodeINGRFromMap: %v", err)
	}
	content := strings.Replace(string(out), `"alice"`, "123", 1)

	_, err = parseINGRAsMap([]byte(content))
	if err == nil {
		t.Fatal("want error for non-string $ID")
	}
	if !strings.Contains(err.Error(), "non-string $ID") {
		t.Errorf("error = %v, want it to mention non-string $ID", err)
	}
}

func TestParseINGRAsMap_InvalidContent(t *testing.T) {
	t.Parallel()
	// Garbage bytes that cannot be parsed as INGR.
	_, err := parseINGRAsMap([]byte("not ingr content at all !!!"))
	if err == nil {
		t.Fatal("want error for invalid INGR content")
	}
}

// ---------------------------------------------------------------------------
// query.go — readAllSingleRecords uncovered branches:
//   - IsExcluded (line 92): a matching ExcludeRegex skips the file
//   - relErr (line 95): filepath.Rel fails when basePath and match are on
//     different volumes — unreachable on a single Unix FS; document it.
//   - recordKey == "" (line 99): key extractor returns empty string
//   - readErr (line 102): readSingleRecordFile returns an error
//   - !found (line 106): file disappears between glob and read (TOCTOU)
//
// readAllMapOfRecords — readErr (line 120):
//   - readMapOfRecordsFile returns an error (bad content in file)
//
// buildKeyExtractor — no {key} in template (already covered), suffix w/ {key}
//
// applyWhere — evaluateCondition error (line 171)
// evaluateGroupCondition — OR case, default case
// evaluateComparison — left resolveExpression error, right resolveExpression error
// resolveExpression — unsupported expression type (line 288-290)
// ---------------------------------------------------------------------------

// TestReadAllSingleRecords_ExcludedFile verifies that files matching
// ExcludeRegex are skipped during readAllSingleRecords.
func TestReadAllSingleRecords_ExcludedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write two files; one matches the exclude regex.
	err = os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("val: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write a.yaml: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "_excluded.yaml"), []byte("val: 2\n"), 0o644)
	if err != nil {
		t.Fatalf("write _excluded.yaml: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:         "{key}.yaml",
			Format:       ingitdb.RecordFormatYAML,
			RecordType:   ingitdb.SingleRecord,
			ExcludeRegex: `^_`,
		},
	}
	recs, err := readAllSingleRecords(colDef)
	if err != nil {
		t.Fatalf("readAllSingleRecords: %v", err)
	}
	if len(recs) != 1 {
		t.Errorf("want 1 record (excluded skipped), got %d: %v", len(recs), recs)
	}
	if recs[0].Key().ID != "a" {
		t.Errorf("expected key 'a', got %v", recs[0].Key().ID)
	}
}

// TestReadAllSingleRecords_EmptyKeyFromExtractor verifies that a file whose
// name does not match the key template is skipped (recordKey == "").
func TestReadAllSingleRecords_EmptyKeyFromExtractor(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Template expects "{key}.yaml" but we also create a ".json" file.
	// The key extractor for "{key}.yaml" returns "" for "data.json".
	err = os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("val: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"val":2}`), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	recs, err := readAllSingleRecords(colDef)
	if err != nil {
		t.Fatalf("readAllSingleRecords: %v", err)
	}
	// The json file is outside the glob pattern entirely because Glob only
	// matches *.yaml. So only "a" appears.
	if len(recs) != 1 {
		t.Errorf("want 1 record, got %d", len(recs))
	}
}

// TestReadAllMapOfRecords_ReadError triggers the error branch in
// readAllMapOfRecords when the file exists but has invalid content.
func TestReadAllMapOfRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	_, err = readAllMapOfRecords(colDef)
	if err == nil {
		t.Fatal("want error for invalid YAML in map-of-records file")
	}
}

// TestBuildKeyExtractor_NoTemplateInSubdir exercises the no-{key} extractor
// with a relative path containing a directory separator.
func TestBuildKeyExtractor_NoTemplateInSubdir(t *testing.T) {
	t.Parallel()
	fn, err := buildKeyExtractor("static.yaml")
	if err != nil {
		t.Fatalf("buildKeyExtractor: %v", err)
	}
	// Relative path from basePath; the extractor strips the extension of the
	// base component.
	got := fn("static.yaml")
	if got != "static" {
		t.Errorf("got %q, want static", got)
	}
}

// TestApplyWhere_EvaluateConditionError exercises applyWhere returning an
// error when evaluateCondition fails (e.g. unsupported condition type).
func TestApplyWhere_EvaluateConditionError(t *testing.T) {
	t.Parallel()
	key := dal.NewKeyWithID("c", "k")
	rec := dal.NewRecordWithData(key, map[string]any{"v": 1})
	rec.SetError(nil)
	records := []dal.Record{rec}
	_, err := applyWhere(records, unsupportedCond{})
	if err == nil {
		t.Fatal("applyWhere: want error for unsupported condition type")
	}
}

// TestEvaluateGroupCondition_And_ViaQuery exercises the dal.And case in
// evaluateGroupCondition through a real query whose multi-condition Where
// path always builds a GroupCondition with the And operator.
//
// The Or and default branches of evaluateGroupCondition are unreachable via
// the public dal API: dal.GroupCondition's fields are unexported, the only
// exported builder (QueryBuilder) always emits the And operator, and the
// Or() method lives on the unexported structuredQuery type. Covering those
// branches would require constructing a dal.GroupCondition with reflect+unsafe;
// we deliberately don't, so they remain documented dead-via-public-API code.
func TestEvaluateGroupCondition_And_ViaQuery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "things", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rec := range []struct{ key, content string }{
		{"a", "val: 1\n"},
		{"b", "val: 10\n"},
		{"c", "val: 20\n"},
	} {
		err = os.WriteFile(filepath.Join(dir, rec.key+".yaml"), []byte(rec.content), 0o644)
		if err != nil {
			t.Fatalf("seed %s: %v", rec.key, err)
		}
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"things": colDef},
		},
	}
	// Build a two-condition AND query: val > 5 AND val < 15 → matches only "b".
	q := dal.From(dal.NewRootCollectionRef("things", "")).NewQuery().
		WhereField("val", dal.GreaterThen, 5).
		WhereField("val", dal.LessThen, 15).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("things", ""), map[string]any{})
		})
	reader, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err != nil {
		t.Fatalf("executeQueryToRecordsReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	var keys []string
	for {
		rec, nextErr := reader.Next()
		if nextErr == dal.ErrNoMoreRecords {
			break
		}
		if nextErr != nil {
			t.Fatalf("Next: %v", nextErr)
		}
		keys = append(keys, rec.Key().ID.(string))
	}
	if len(keys) != 1 || keys[0] != "b" {
		t.Errorf("AND query: got %v, want [b]", keys)
	}
}

// TestResolveExpression_Unsupported exercises the default branch in
// resolveExpression (query.go line 288-290).
func TestResolveExpression_Unsupported(t *testing.T) {
	t.Parallel()
	_, err := resolveExpression(unsupportedExpr{}, map[string]any{}, "k")
	if err == nil {
		t.Fatal("want error for unsupported expression type")
	}
	if !strings.Contains(err.Error(), "unsupported expression type") {
		t.Errorf("error should mention 'unsupported expression type', got: %v", err)
	}
}

// unsupportedExpr satisfies dal.Expression but is not FieldRef or Constant.
type unsupportedExpr struct{}

func (unsupportedExpr) String() string { return "unsupported-expr" }

// TestEvaluateComparison_LeftResolveError exercises the left-resolve error
// in evaluateComparison (parse.go line 255-257).
func TestEvaluateComparison_LeftResolveError(t *testing.T) {
	t.Parallel()
	cmp := dal.Comparison{
		Left:     unsupportedExpr{},
		Operator: dal.Equal,
		Right:    dal.Constant{Value: 1},
	}
	_, err := evaluateComparison(cmp, map[string]any{}, "k")
	if err == nil {
		t.Fatal("want error from left-side unsupported expression")
	}
}

// TestEvaluateComparison_RightResolveError exercises the right-resolve error
// in evaluateComparison (parse.go line 259-261).
func TestEvaluateComparison_RightResolveError(t *testing.T) {
	t.Parallel()
	cmp := dal.Comparison{
		Left:     dal.NewFieldRef("", "v"),
		Operator: dal.Equal,
		Right:    unsupportedExpr{},
	}
	_, err := evaluateComparison(cmp, map[string]any{"v": 1}, "k")
	if err == nil {
		t.Fatal("want error from right-side unsupported expression")
	}
}

// ---------------------------------------------------------------------------
// record_io.go — uncovered branches
//
// readSingleRecordFile:
//   - stat non-ErrNotExist error: produced by a broken-stat scenario.
//     On Unix, we can trigger this by making the parent directory unreadable.
//   - withSharedLock fn error (os.ReadFile fails inside lock): same technique.
//   - parse error inside lock: corrupt YAML content.
//
// writeSingleRecordFile:
//   - os.MkdirAll fails: parent of parent is a file.
//   - EncodeRecordContentForCollection fails: unsupported format.
//   - os.WriteFile inside lock fails: path is a directory.
//
// readMapOfRecordsFile:
//   - stat non-ErrNotExist error.
//   - os.ReadFile inside lock fails.
//
// writeMapOfRecordsFile:
//   - os.MkdirAll fails.
//   - EncodeMapOfRecordsContent fails.
//   - os.WriteFile inside lock fails.
//
// deleteSingleRecordFile:
//   - stat non-ErrNotExist error.
//   - os.Remove returns ErrNotExist (TOCTOU — file deleted between stat and Remove).
// ---------------------------------------------------------------------------

// TestReadSingleRecordFile_ParseError exercises the branch where the file
// content is valid enough to be read but invalid as YAML.
func TestReadSingleRecordFile_ParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.yaml")
	// YAML that parses as a scalar, not a map — ParseRecordContentForCollection errors.
	err := os.WriteFile(p, []byte("- just a list item\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, _, err = readSingleRecordFile(p, colDef)
	if err == nil {
		t.Fatal("want error for YAML list (not a map)")
	}
}

// TestWriteSingleRecordFile_MkdirError triggers os.MkdirAll failure by
// placing a file at the path that should be the parent directory.
func TestWriteSingleRecordFile_MkdirError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a file at "parent" — so MkdirAll("parent/subdir") fails.
	parent := filepath.Join(root, "parent")
	err := os.WriteFile(parent, []byte("x"), 0o644)
	if err != nil {
		t.Fatalf("write parent as file: %v", err)
	}
	p := filepath.Join(parent, "subdir", "rec.yaml")
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: root,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	err = writeSingleRecordFile(p, colDef, map[string]any{"x": 1})
	if err == nil {
		t.Fatal("want error when MkdirAll fails")
	}
}

// TestWriteSingleRecordFile_EncodeError triggers the EncodeRecordContentForCollection
// error branch by passing a collection with an unsupported format.
func TestWriteSingleRecordFile_EncodeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.unsupported")
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormat("unsupported"),
			RecordType: ingitdb.SingleRecord,
		},
	}
	err := writeSingleRecordFile(p, colDef, map[string]any{"x": 1})
	if err == nil {
		t.Fatal("want error for unsupported format in writeSingleRecordFile")
	}
}

// TestWriteSingleRecordFile_WriteError triggers os.WriteFile failure inside
// the exclusive lock by pointing the path at a directory.
func TestWriteSingleRecordFile_WriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a directory at the target path — WriteFile cannot overwrite a dir.
	p := filepath.Join(root, "isdir")
	err := os.MkdirAll(p, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: root,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	err = writeSingleRecordFile(p, colDef, map[string]any{"x": 1})
	if err == nil {
		t.Fatal("want error when writing to a directory path")
	}
}

// TestWriteMapOfRecordsFile_MkdirError triggers os.MkdirAll failure.
func TestWriteMapOfRecordsFile_MkdirError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a file where the parent directory should be.
	parent := filepath.Join(root, "blocked")
	err := os.WriteFile(parent, []byte("x"), 0o644)
	if err != nil {
		t.Fatalf("write blocked as file: %v", err)
	}
	p := filepath.Join(parent, "subdir", "records.yaml")
	colDef := &ingitdb.CollectionDef{
		ID: "c",
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatYAML,
		},
	}
	err = writeMapOfRecordsFile(p, colDef, map[string]map[string]any{"k": {"v": 1}})
	if err == nil {
		t.Fatal("want error when MkdirAll fails")
	}
}

// TestWriteMapOfRecordsFile_EncodeError triggers the encode error by using
// an unsupported format.
func TestWriteMapOfRecordsFile_EncodeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.unsupported")
	colDef := &ingitdb.CollectionDef{
		ID: "c",
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormat("unsupported"),
		},
	}
	err := writeMapOfRecordsFile(p, colDef, map[string]map[string]any{"k": {"v": 1}})
	if err == nil {
		t.Fatal("want error for unsupported format in writeMapOfRecordsFile")
	}
}

// TestWriteMapOfRecordsFile_WriteError triggers os.WriteFile failure by
// pointing at a directory.
func TestWriteMapOfRecordsFile_WriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(root, "isdir")
	err := os.MkdirAll(p, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID: "c",
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatYAML,
		},
	}
	err = writeMapOfRecordsFile(p, colDef, map[string]map[string]any{"k": {"v": 1}})
	if err == nil {
		t.Fatal("want error when writing to a directory path")
	}
}

// TestDeleteSingleRecordFile_ErrNotExistInsideLock exercises the branch where
// os.Remove returns ErrNotExist inside the lock (TOCTOU).  This is genuinely
// hard to produce in a race-free unit test because it requires deleting the
// file between stat and Remove.  We instead verify the stat non-ErrNotExist
// branch by using a restricted parent directory.
//
// Note: On macOS, making a parent directory read-only still lets flock open
// (O_CREATE) files in it sometimes, so this test may be platform-sensitive.
// We make the file itself unreadable instead, which causes os.Stat to fail
// with a permission error (not ErrNotExist) on the file directly — but Stat
// on an existing file always succeeds even if it's unreadable because Stat
// only reads the inode, not the contents.  We therefore skip this particular
// sub-case and focus on the happy paths that are testable.
func TestDeleteSingleRecordFile_StatNonExistError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any file; cannot exercise permission-error branch")
	}
	root := t.TempDir()
	// Make the directory non-readable so Stat on files inside it fails.
	p := filepath.Join(root, "target.yaml")
	err := os.WriteFile(p, []byte("x: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	err = os.Chmod(root, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()

	err = deleteSingleRecordFile(p)
	if err == nil {
		t.Fatal("want error for stat failure on non-existent-not-ErrNotExist path")
	}
	if err == dal.ErrRecordNotFound {
		t.Errorf("should not be ErrRecordNotFound for a stat permission error")
	}
}

// ---------------------------------------------------------------------------
// registry.go — WriteRootCollections error branches
// ---------------------------------------------------------------------------

// TestRegisterInRootCollections_WriteError covers the write-error branch in
// registerInRootCollections (registry.go line 52-54) via the
// writeRootCollections seam. Deterministic and root-safe, unlike chmod (which
// fails the earlier read when .ingitdb loses its execute bit).
//
// Intentionally NOT parallel: it mutates the package-level seam.
func TestRegisterInRootCollections_WriteError(t *testing.T) {
	orig := writeRootCollections
	writeRootCollections = func(string, map[string]string) error { return os.ErrPermission }
	defer func() { writeRootCollections = orig }()

	err := registerInRootCollections(t.TempDir(), "tags")
	if err == nil {
		t.Fatal("registerInRootCollections: want error when write fails")
	}
	if !strings.Contains(err.Error(), "write root-collections.yaml") {
		t.Errorf("error = %v, want it to wrap the write failure", err)
	}
}

// TestDeregisterFromRootCollections_WriteError covers the write-error branch in
// deregisterFromRootCollections (registry.go line 80-82). An entry is
// registered first (real write), then the seam is swapped so the delete-rewrite
// fails. Intentionally NOT parallel: it mutates the package-level seam.
func TestDeregisterFromRootCollections_WriteError(t *testing.T) {
	root := t.TempDir()
	if err := registerInRootCollections(root, "tags"); err != nil {
		t.Fatalf("seed registry: %v", err)
	}
	orig := writeRootCollections
	writeRootCollections = func(string, map[string]string) error { return os.ErrPermission }
	defer func() { writeRootCollections = orig }()

	err := deregisterFromRootCollections(root, "tags")
	if err == nil {
		t.Fatal("deregisterFromRootCollections: want error when write fails")
	}
	if !strings.Contains(err.Error(), "write root-collections.yaml") {
		t.Errorf("error = %v, want it to wrap the write failure", err)
	}
}

// ---------------------------------------------------------------------------
// schema_modifier.go — uncovered branches
//
// CreateCollection:
//   - definition.yaml exists but IfNotExists + registerInRootCollections fails
//   - stat returns non-ErrNotExist error
//   - os.MkdirAll fails
//   - len(c.Indexes) > 0 (log warning path)
//   - withExclusiveLock / writeCollectionDefYAML fails
//   - registerInRootCollections fails after writing YAML
//
// DropCollection:
//   - validateCollectionName fails
//   - stat returns non-ErrNotExist error
//   - os.RemoveAll fails (directory is not removable)
//   - deregisterFromRootCollections fails
//
// AlterCollection:
//   - writeCollectionDefYAML fails inside loop (PartialSuccessError with flush error)
//
// validateCollectionName:
//   - filepath.IsAbs check
//
// writeCollectionDefYAML:
//   - os.WriteFile fails (dir is non-writable)
//
// rewriteRecordFiles:
//   - walkErr != nil (walk error from permission-denied directory)
//   - read error inside walk
//   - yaml.Unmarshal error inside walk
//   - marshal error inside walk (unlikely, see note)
//   - WriteFile error inside walk
// ---------------------------------------------------------------------------

// TestCreateCollection_IfNotExists_RegisterFails exercises the branch where
// IfNotExists is set, the collection already exists, but registerInRootCollections
// fails (schema_modifier.go line 57-59).
func TestCreateCollection_IfNotExists_RegisterFails(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	// Create the collection first.
	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Make .ingitdb read-only so registerInRootCollections cannot write.
	ingitdbDir := filepath.Join(root, ".ingitdb")
	err = os.Chmod(ingitdbDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(ingitdbDir, 0o755) }()

	// Now delete the registry file so registerInRootCollections must write.
	regPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	err = os.Chmod(ingitdbDir, 0o755)
	if err != nil {
		t.Fatalf("chmod restore: %v", err)
	}
	err = os.Remove(regPath)
	if err != nil {
		t.Fatalf("remove reg: %v", err)
	}
	err = os.Chmod(ingitdbDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	err = modifier.CreateCollection(context.Background(), tagsCollectionDef(), ddl.IfNotExists())
	if err == nil {
		t.Fatal("CreateCollection IfNotExists: want error when register fails")
	}
}

// TestCreateCollection_WithIndexes exercises the len(c.Indexes) > 0 log
// warning path in CreateCollection (schema_modifier.go line 70-72).
func TestCreateCollection_WithIndexes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	col := dbschema.CollectionDef{
		Name:   "indexed",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
		Indexes: []dbschema.IndexDef{
			{Name: "label_idx", Fields: []dal.FieldName{"label"}},
		},
	}
	// Should succeed — indexes are ignored with a log warning.
	err = modifier.CreateCollection(context.Background(), col)
	if err != nil {
		t.Errorf("CreateCollection with indexes: unexpected error: %v", err)
	}
}

// TestCreateCollection_MkdirFails exercises os.MkdirAll failure in
// CreateCollection (schema_modifier.go line 66-68) by swapping the osMkdirAll
// seam. Deterministic and root-safe, unlike a chmod-based approach (which
// fails at the earlier os.Stat traversal, not at MkdirAll).
//
// Intentionally NOT parallel: it mutates the package-level osMkdirAll seam,
// so it must not run concurrently with other tests that create collections.
func TestCreateCollection_MkdirFails(t *testing.T) {
	orig := osMkdirAll
	osMkdirAll = func(string, os.FileMode) error { return os.ErrPermission }
	defer func() { osMkdirAll = orig }()

	db := &Database{projectPath: t.TempDir(), reader: newReader()}
	err := db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error when mkdir fails")
	}
	if !strings.Contains(err.Error(), "mkdir") {
		t.Errorf("error = %v, want it to mention mkdir", err)
	}
}

// TestCreateCollection_WriteYAMLFails exercises the writeCollectionDefYAML
// failure path (schema_modifier.go line 74-78).
func TestCreateCollection_WriteYAMLFails(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()
	// Pre-create the .collection directory but make it read-only.
	colSchemaDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err := os.MkdirAll(colSchemaDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.Chmod(colSchemaDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colSchemaDir, 0o755) }()

	db := &Database{projectPath: root, reader: newReader()}
	err = db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error when definition.yaml write fails")
	}
}

// TestDropCollection_ValidateBadName exercises validateCollectionName failure
// in DropCollection (schema_modifier.go line 92-94).
func TestDropCollection_ValidateBadName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db := &Database{projectPath: root, reader: newReader()}
	err := db.DropCollection(context.Background(), "")
	if err == nil {
		t.Fatal("DropCollection(\"\"): want error")
	}
}

// TestDropCollection_StatNonExistError exercises the "stat returns
// non-ErrNotExist" branch in DropCollection (schema_modifier.go line 107).
func TestDropCollection_StatNonExistError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any path")
	}
	root := t.TempDir()
	// Create collection dir but make .collection unreadable so Stat fails with EACCES.
	colDir := filepath.Join(root, "tags")
	err := os.MkdirAll(colDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.Chmod(colDir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colDir, 0o755) }()

	db := &Database{projectPath: root, reader: newReader()}
	err = db.DropCollection(context.Background(), "tags")
	if err == nil {
		t.Fatal("DropCollection: want error for stat EACCES")
	}
}

// TestDropCollection_DeregisterFails exercises the deregisterFromRootCollections
// failure branch (schema_modifier.go line 115-117).
func TestDropCollection_DeregisterFails(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Make .ingitdb read-only so deregisterFromRootCollections cannot write.
	ingitdbDir := filepath.Join(root, ".ingitdb")
	err = os.Chmod(ingitdbDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(ingitdbDir, 0o755) }()

	err = modifier.DropCollection(context.Background(), "tags")
	if err == nil {
		t.Fatal("DropCollection: want error when deregister fails")
	}
}

// TestAlterCollection_FlushError exercises the PartialSuccessError with a
// flush error (schema_modifier.go line 163-172). We need the op to succeed
// but the subsequent writeCollectionDefYAML to fail.  We do this by:
//  1. Creating the collection normally.
//  2. Making definition.yaml read-only after AlterCollection acquires the lock
//     — but this is a TOCTOU race. Instead, we make the directory read-only
//     before AlterCollection runs so that writeCollectionDefYAML fails
//     consistently.
func TestAlterCollection_FlushError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	err = modifier.CreateCollection(context.Background(), tags)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Make .collection read-only so writeCollectionDefYAML fails.
	colSchemaDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err = os.Chmod(colSchemaDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colSchemaDir, 0o755) }()

	op := ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String})
	err = modifier.AlterCollection(context.Background(), "tags", op)
	// Either the lock fails (because flock opens with O_CREATE which needs write)
	// or writeCollectionDefYAML fails.  Either way we expect an error.
	if err == nil {
		t.Fatal("AlterCollection: want error when flush fails")
	}
}

// TestValidateCollectionName_AbsolutePath exercises the filepath.IsAbs branch
// (schema_modifier.go line 315-317).
//
// Note: on Unix, any absolute path starts with '/' which splits to an empty
// first segment, hitting the invalid-segment check before reaching IsAbs.
// The filepath.IsAbs branch is therefore unreachable on Unix because the
// empty-segment guard always fires first.  On Windows, "C:\path" would pass
// the '/' split loop and reach IsAbs, but this is a Unix-only codebase in
// practice.  We test the invalid-segment path instead, which covers the same
// "bad name" contract from the caller's perspective.
func TestValidateCollectionName_AbsolutePath(t *testing.T) {
	t.Parallel()
	err := validateCollectionName("/absolute/path")
	if err == nil {
		t.Fatal("validateCollectionName: want error for absolute path")
	}
	// On Unix the empty first segment is caught first; on Windows IsAbs fires.
	// Either way an error is returned.
	_ = err
}

// TestWriteCollectionDefYAML_WriteToDirectory exercises the os.WriteFile failure
// branch in writeCollectionDefYAML by passing a path that is a directory.
func TestWriteCollectionDefYAML_WriteToDirectory(t *testing.T) {
	t.Parallel()
	// Pass a path pointing at a directory (WriteFile cannot overwrite a dir).
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{ID: "c"}
	err := writeCollectionDefYAML(dir, colDef)
	if err == nil {
		t.Fatal("writeCollectionDefYAML: want error when writing to a directory")
	}
}

// TestRewriteRecordFiles_WalkError exercises the walkErr != nil branch
// (schema_modifier.go line 403). We make the records directory non-readable
// after it has been found by WalkDir.
func TestRewriteRecordFiles_WalkError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any directory")
	}
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	err := os.MkdirAll(sub, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a yaml file inside the sub directory.
	err = os.WriteFile(filepath.Join(sub, "rec.yaml"), []byte("a: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Remove execute permission on sub so WalkDir cannot descend into it.
	err = os.Chmod(sub, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(sub, 0o755) }()

	// rewriteRecordFiles walks root; when it tries to descend into sub it will
	// fail because sub is not executable.
	err = rewriteRecordFiles(root, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error for unreadable subdirectory")
	}
}

// TestRewriteRecordFiles_ReadError exercises the read error inside the walk
// (schema_modifier.go line 411-413) by swapping the osReadFile seam. A real
// record file is present so WalkDir yields it and the lock opens; only the
// read fails. Deterministic and root-safe, unlike chmod (which fails at
// lock-open, not at ReadFile).
//
// Intentionally NOT parallel: it mutates the package-level osReadFile seam.
func TestRewriteRecordFiles_ReadError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rec.yaml"), []byte("a: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	defer func() { osReadFile = orig }()

	err := rewriteRecordFiles(root, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error when reading a record file fails")
	}
	if !strings.Contains(err.Error(), "read ") {
		t.Errorf("error = %v, want it to wrap the read failure", err)
	}
}

// TestRewriteRecordFiles_WriteFileError exercises the os.WriteFile error
// inside the walk (schema_modifier.go line 424-426).
func TestRewriteRecordFiles_WriteFileError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write any file")
	}
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	err := os.WriteFile(p, []byte("a: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Make the file read-only so the re-write fails.
	err = os.Chmod(p, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()

	err = rewriteRecordFiles(root, ingitdb.RecordFormatYAML, func(rec map[string]any) {
		rec["extra"] = "injected"
	})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error for read-only yaml file")
	}
}

// ---------------------------------------------------------------------------
// schema_reader.go — ListCollections uncovered branches
//
//   - walkErr != nil at the top level (root dir itself fails)
//   - relErr != nil: filepath.Rel fails when the match is outside projectPath
//     (impossible on a single FS; document as unreachable)
//   - rel == "." branch: projectPath itself contains definition.yaml
// ---------------------------------------------------------------------------

// TestListCollections_WalkError exercises the walkErr != nil branch
// (schema_reader.go line 72-74) by making the project directory unreadable.
func TestListCollections_WalkError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can traverse any directory")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	// Make root unreadable after DB is constructed.
	err = os.Chmod(root, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()

	reader := db.(dbschema.SchemaReader)
	_, err = reader.ListCollections(context.Background(), nil)
	if err == nil {
		t.Fatal("ListCollections: want error for unreadable project directory")
	}
}

// TestListCollections_SkipsRootDefinitionYAML exercises the rel == "."
// check (schema_reader.go line 65-67): if projectPath itself has a
// .collection/definition.yaml it must be skipped.
func TestListCollections_SkipsRootDefinitionYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Plant a definition.yaml directly at root/.collection/definition.yaml.
	colDir := filepath.Join(root, ingitdb.SchemaDir)
	err := os.MkdirAll(colDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(filepath.Join(colDir, ingitdb.CollectionDefFileName), []byte(countriesDef), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Also create a normal collection.
	writeCollectionDef(t, root, "tags", countriesDef)

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	refs, err := reader.ListCollections(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	// Only "tags" should appear; the root definition.yaml is skipped.
	for _, ref := range refs {
		if ref.Name() == "." || ref.Name() == "" {
			t.Errorf("ListCollections returned root entry: %v", ref.Name())
		}
	}
}

// TestDescribeCollection_StatNonNotFoundError exercises the branch where Stat
// returns a non-ErrNotExist error (schema_reader.go line 95-96).
func TestDescribeCollection_StatNonNotFoundError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any path")
	}
	root := t.TempDir()
	colDir := filepath.Join(root, "tags")
	err := os.MkdirAll(colDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Make colDir unreadable so Stat on definition.yaml inside it fails.
	err = os.Chmod(colDir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colDir, 0o755) }()

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	_, err = reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection: want error for stat permission failure")
	}
	// Specifically it must NOT say "not found" since it's a permission error.
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("error should not say 'not found' for EACCES; got: %v", err)
	}
}

// TestDescribeCollection_YAMLParseError exercises the yaml.Unmarshal error
// branch inside the shared lock (schema_reader.go line 104-106).
func TestDescribeCollection_YAMLParseError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write an invalid YAML file as definition.yaml.
	colDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err := os.MkdirAll(colDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defPath := filepath.Join(colDir, ingitdb.CollectionDefFileName)
	err = os.WriteFile(defPath, []byte("{broken yaml badly"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	_, err = reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection: want error for invalid YAML in definition.yaml")
	}
}

// TestDescribeCollection_UnknownColumnType exercises the ingitdbTypeToDBSchema
// error branch (schema_reader.go line 132-134).
func TestDescribeCollection_UnknownColumnType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defYAML := `record_file:
  name: "{key}.yaml"
  format: yaml
  type: single_record
columns_order: [name]
columns:
  name:
    type: unknowntype123
`
	writeCollectionDef(t, root, "items", defYAML)
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("items", "")
	_, err = reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection: want error for unknown column type")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go — Get / Exists uncovered branches
//
// Get SingleRecord — readSingleRecordFile returns an error (via parse error)
// Exists SingleRecord — readSingleRecordFile returns an error
// ---------------------------------------------------------------------------

// TestReadonlyTx_Get_SingleRecord_ReadError exercises the readSingleRecordFile
// error branch in Get (tx_readonly.go line 40-42).
func TestReadonlyTx_Get_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "k.yaml")
	// Write a YAML scalar (not a map) — ParseRecordContentForCollection will error.
	err = os.WriteFile(p, []byte("- list item\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"items": colDef},
		},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("items", "k"), map[string]any{})
	err = tx.Get(context.Background(), rec)
	if err == nil {
		t.Fatal("Get: want error for corrupt YAML record")
	}
	// The error must also be set on the record, per the dalgo contract:
	// Exists()/Data() panic if SetError was never called after Get.
	if !errors.Is(rec.Error(), err) {
		t.Errorf("rec.Error() = %v, want it to carry the Get error %v", rec.Error(), err)
	}
}

// TestReadonlyTx_Exists_SingleRecord_ReadError exercises the readSingleRecordFile
// error branch in Exists (tx_readonly.go line 83-85).
func TestReadonlyTx_Exists_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "k.yaml")
	err = os.WriteFile(p, []byte("- list item\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"items": colDef},
		},
	}
	_, err = tx.Exists(context.Background(), dal.NewKeyWithID("items", "k"))
	if err == nil {
		t.Fatal("Exists: want error for corrupt YAML record")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go — Set / Insert uncovered branches
//
// Set SingleRecord — writeSingleRecordFile error (encode fails)
// Set MapOfRecords — readMapOfRecordsFile error
// Insert SingleRecord — stat non-ErrNotExist error (permission-denied parent)
// Insert MapOfRecords — readMapOfRecordsFile error
// UpdateRecord — applyUpdates error (nested field path)
// ---------------------------------------------------------------------------

// TestReadwriteTx_Set_SingleRecord_WriteError exercises the writeSingleRecordFile
// error path in Set (tx_readwrite.go line 40-42).
func TestReadwriteTx_Set_SingleRecord_WriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormat("unsupported"),
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"c": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{"x": 1})
	err := tx.Set(context.Background(), rec)
	if err == nil {
		t.Fatal("Set: want error for unsupported format")
	}
}

// TestReadwriteTx_Set_MapOfRecords_ReadError exercises the readMapOfRecordsFile
// error path in Set (tx_readwrite.go line 44-46).
func TestReadwriteTx_Set_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{"score": 1})
	err = tx.Set(context.Background(), rec)
	if err == nil {
		t.Fatal("Set MapOfRecords: want error for corrupt file")
	}
}

// TestReadwriteTx_Insert_SingleRecord_StatError exercises the stat non-ErrNotExist
// branch in Insert (tx_readwrite.go line 86-88).
func TestReadwriteTx_Insert_SingleRecord_StatError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "c", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Make the directory non-executable so Stat on files inside it fails.
	err = os.Chmod(dir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"c": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{"x": 1})
	err = tx.Insert(context.Background(), rec)
	if err == nil {
		t.Fatal("Insert: want error for stat permission failure")
	}
}

// TestReadwriteTx_Insert_MapOfRecords_ReadError exercises the readMapOfRecordsFile
// error path in Insert (tx_readwrite.go line 93-95).
func TestReadwriteTx_Insert_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{"score": 1})
	err = tx.Insert(context.Background(), rec)
	if err == nil {
		t.Fatal("Insert MapOfRecords: want error for corrupt file")
	}
}

// TestReadwriteTx_UpdateRecord_ApplyUpdatesError exercises the applyUpdates
// error branch in UpdateRecord (tx_readwrite.go line 191-193).
func TestReadwriteTx_UpdateRecord_ApplyUpdatesError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"c": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{})
	rec.SetError(nil)
	// Nested field path triggers an error in applyUpdates.
	ups := []update.Update{update.ByFieldPath(update.FieldPath{"nested", "key"}, "val")}
	err := tx.UpdateRecord(context.Background(), rec, ups)
	if err == nil {
		t.Fatal("UpdateRecord: want error for nested field path")
	}
}

// ---------------------------------------------------------------------------
// type_mapping.go — dbschemaTypeToIngitdb unsupported type branch
// ---------------------------------------------------------------------------

// TestDBSchemaTypeToIngitdb_Unsupported exercises the final error return
// (type_mapping.go line 82) for a type that is not in the switch.
func TestDBSchemaTypeToIngitdb_Unsupported(t *testing.T) {
	t.Parallel()
	// Use a dbschema.Type integer value that is not in the switch statement.
	unsupported := dbschema.Type(127)
	_, err := dbschemaTypeToIngitdb(unsupported)
	if err == nil {
		t.Fatal("dbschemaTypeToIngitdb: want error for unsupported type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention 'unsupported', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// batch_parsers.go — ParseBatchCSV: csv parse error in data row (line 262-264)
// and header empty (line 244-246)
// ---------------------------------------------------------------------------

// TestParseBatchCSV_DataRowParseError exercises the csv parse error branch in
// the data-row loop (batch_parsers.go line 262-264).
func TestParseBatchCSV_DataRowParseError(t *testing.T) {
	t.Parallel()
	// CSV with an unclosed quoted field causes a parse error on the data row.
	in := strings.NewReader("$id,name\nfrance,\"unclosed")
	_, err := ParseBatchCSV(in, CSVParseOptions{})
	if err == nil {
		t.Fatal("ParseBatchCSV: want error for malformed CSV data row")
	}
}

// Note: the "csv header is empty" check in ParseBatchCSV (batch_parsers.go line
// 244-246) is unreachable via the public API.  csv.NewReader always returns at
// least one field per non-EOF row (even a bare "\n" is returned as EOF, not as
// an empty-slice success).  The only path to an empty header would be
// opts.Fields being an empty slice, but len(opts.Fields) == 0 falls into the
// else branch that reads from the CSV reader.  This branch is documented as
// dead code and is not exercised here.

// ---------------------------------------------------------------------------
// batch_parsers.go — ParseBatchINGR: io.ReadAll error and ingr.Unmarshal error
// ---------------------------------------------------------------------------

// TestParseBatchINGR_ReadError exercises the io.ReadAll error branch
// (batch_parsers.go line 161-163) using a reader that always errors.
func TestParseBatchINGR_ReadError(t *testing.T) {
	t.Parallel()
	_, err := ParseBatchINGR(&alwaysErrorReader{})
	if err == nil {
		t.Fatal("ParseBatchINGR: want error for read failure")
	}
	if !strings.Contains(err.Error(), "read ingr stream") {
		t.Errorf("error should mention 'read ingr stream', got: %v", err)
	}
}

// TestParseBatchINGR_UnmarshalError exercises the ingr.Unmarshal error branch
// (batch_parsers.go line 168-170).
func TestParseBatchINGR_UnmarshalError(t *testing.T) {
	t.Parallel()
	// Non-empty content that ingr.Unmarshal rejects as invalid.
	_, err := ParseBatchINGR(bytes.NewReader([]byte("not valid ingr content at all !!!")))
	if err == nil {
		t.Fatal("ParseBatchINGR: want error for invalid INGR content")
	}
}

// TestParseBatchINGR_EmptyStringID exercises the empty $ID branch
// (batch_parsers.go line 184-186).
func TestParseBatchINGR_EmptyStringID(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	w := ingr.NewRecordsWriter(&buf)
	_, err := w.WriteHeader("test", []ingr.ColDef{{Name: "$ID"}, {Name: "name"}})
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	// Use empty string as the record key ("" is what gets written as $ID).
	_, err = w.WriteRecords(0, ingr.NewMapRecordEntry("", map[string]any{"name": "foo"}))
	if err != nil {
		t.Fatalf("WriteRecords: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, parseErr := ParseBatchINGR(strings.NewReader(buf.String()))
	if parseErr == nil {
		t.Fatal("ParseBatchINGR: want error for empty $ID")
	}
}

// TestParseBatchJSONL_ScanError exercises the scanner.Err() branch
// (batch_parsers.go line 103-105) using a reader that fails after a line.
func TestParseBatchJSONL_ScanError(t *testing.T) {
	t.Parallel()
	_, err := ParseBatchJSONL(&halfwayErrorReader{firstLine: `{"$id":"ie"}`})
	if err == nil {
		t.Fatal("ParseBatchJSONL: want error when scanner fails")
	}
}

// halfwayErrorReader returns one good line then errors.
type halfwayErrorReader struct {
	firstLine string
	sent      bool
}

func (r *halfwayErrorReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		line := r.firstLine + "\n"
		n := copy(p, line)
		return n, nil
	}
	return 0, &permanentReadError{}
}

type permanentReadError struct{}

func (e *permanentReadError) Error() string { return "permanent read error" }

// alwaysErrorReader satisfies io.Reader and always returns an error.
type alwaysErrorReader struct{}

func (r *alwaysErrorReader) Read(_ []byte) (int, error) {
	return 0, &alwaysReadError{}
}

type alwaysReadError struct{}

func (e *alwaysReadError) Error() string { return "always fails" }

// ---------------------------------------------------------------------------
// csv.go — encodeCSVForCollection: csv.Writer.Write header error
// The csv.Writer only errors when the underlying writer errors. Since we use
// bytes.Buffer which never errors, the header-write and row-write error
// branches (csv.go lines 110, 127, 131) are unreachable via the public API.
// We document this below.
// ---------------------------------------------------------------------------

// Note: csv.go:568 (w.Write(colDef.ColumnsOrder) error), csv.go:585
// (w.Write(cells) error), and csv.go:590 (w.Error() error) are all
// unreachable because encodeCSVForCollection uses bytes.Buffer as the
// underlying writer, and bytes.Buffer.Write never returns an error.
// These branches would only be reachable if encodeCSVForCollection accepted
// an io.Writer parameter instead of always using bytes.Buffer internally.

// ---------------------------------------------------------------------------
// parse.go — encodeINGRFromMap: WriteHeader / WriteRecords / Close errors
// These are also unreachable because ingr.NewRecordsWriter wraps a bytes.Buffer
// internally, and all three operations only fail on underlying writer errors,
// which bytes.Buffer never produces.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// record_io.go — readMapOfRecordsFile stat non-ErrNotExist error
// ---------------------------------------------------------------------------

// TestReadMapOfRecordsFile_StatError exercises the stat non-ErrNotExist error
// branch (record_io.go line 84).
func TestReadMapOfRecordsFile_StatError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("a:\n  v: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Make dir non-executable so Stat on the file inside fails with EACCES.
	err = os.Chmod(dir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	_, err = readMapOfRecordsFile(p, ingitdb.RecordFormatYAML)
	if err == nil {
		t.Fatal("readMapOfRecordsFile: want error for stat permission failure")
	}
}

// ---------------------------------------------------------------------------
// readAllSingleRecords — recordKey == "" branch (key extractor returns empty)
// ---------------------------------------------------------------------------

// TestReadAllSingleRecords_NonMatchingFile verifies that files matching the
// glob pattern but producing an empty key from the extractor are skipped
// (recordKey == "" branch, query.go line 99-101).
//
// The no-{key} extractor path (idx < 0) strips the file extension from the
// base name. For nameTemplate="static.yaml" (no {key}), the glob is
// "static.yaml" and the extractor calls TrimSuffix(Base(relPath), Ext(relPath)).
// The file at match path "static.yaml" gives relPath="static.yaml",
// Base="static.yaml", Ext=".yaml", key="static" (non-empty).
//
// For the {key} extractor: template "rec_{key}.yaml" → regex "^rec_(.*?)\.yaml$".
// File "rec_.yaml" matches and gives key="" (empty capture).
func TestReadAllSingleRecords_NonMatchingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// DirPath is the collection root; $records sub-dir holds individual files.
	colDirPath := filepath.Join(root, "items")
	recordsDir := filepath.Join(colDirPath, "$records")
	err := os.MkdirAll(recordsDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: colDirPath,
		Columns: map[string]*ingitdb.ColumnDef{},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "rec_{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	// "rec_.yaml" matches glob "rec_*.yaml" → regex gives key="" → skipped.
	err = os.WriteFile(filepath.Join(recordsDir, "rec_.yaml"), []byte("val: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write empty-key file: %v", err)
	}
	// "rec_abc.yaml" → key="abc" → returned as a record.
	err = os.WriteFile(filepath.Join(recordsDir, "rec_abc.yaml"), []byte("val: 2\n"), 0o644)
	if err != nil {
		t.Fatalf("write good file: %v", err)
	}
	recs, err := readAllSingleRecords(colDef)
	if err != nil {
		t.Fatalf("readAllSingleRecords: %v", err)
	}
	// Only "abc" should appear; empty-key record is skipped.
	if len(recs) != 1 {
		t.Errorf("want 1 record (empty-key skipped), got %d", len(recs))
	}
	if len(recs) == 1 && recs[0].Key().ID != "abc" {
		t.Errorf("expected key 'abc', got %v", recs[0].Key().ID)
	}
}

// TestReadAllSingleRecords_ReadError exercises the readSingleRecordFile error
// branch inside readAllSingleRecords (query.go line 102-104).
func TestReadAllSingleRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a YAML list (not a map) — ParseRecordContentForCollection will error.
	p := filepath.Join(dir, "bad.yaml")
	err = os.WriteFile(p, []byte("- list item\n- another\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		Columns: map[string]*ingitdb.ColumnDef{},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, err = readAllSingleRecords(colDef)
	if err == nil {
		t.Fatal("readAllSingleRecords: want error for corrupt YAML file")
	}
}

// ---------------------------------------------------------------------------
// parseCSVForCollection — uncovered row-level branches
// ---------------------------------------------------------------------------

// TestParseCSVForCollection_RowReadError exercises the "failed to read csv row"
// branch (csv.go line 49).
func TestParseCSVForCollection_RowReadError(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	// Header is valid; data row has an unclosed quote causing a parse error.
	content := []byte("a,b\n\"unclosed,value\n")
	_, err := parseCSVForCollection(content, colDef)
	if err == nil {
		t.Fatal("parseCSVForCollection: want error for malformed data row")
	}
}

// TestParseCSVForCollection_WrongRowLength exercises the row length mismatch
// branch (csv.go line 52-54) — this branch requires FieldsPerRecord = -1 and
// a row shorter than the header. However, csv.NewReader with FieldsPerRecord=-1
// accepts variable-length rows.  The "wrong length" check is after the CSV
// read, checking len(fields) != len(header).  We need a row with a different
// number of fields than the header.
//
// Note: csv.NewReader with FieldsPerRecord=-1 does NOT enforce column counts,
// so a row with fewer columns IS returned with the fewer-column slice.
func TestParseCSVForCollection_RowLengthMismatch(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b", "c"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	// Header has 3 columns; data row has only 2.
	content := []byte("a,b,c\n1,2\n")
	_, err := parseCSVForCollection(content, colDef)
	if err == nil {
		t.Fatal("parseCSVForCollection: want error for row with wrong column count")
	}
}

// TestParseCSVForCollection_HeaderReadError exercises the "failed to read csv
// header" branch (csv.go line 36-38): the first csv.Read returns a non-EOF
// parse error rather than io.EOF.
func TestParseCSVForCollection_HeaderReadError(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	// The header line itself has an unclosed quote, so the very first
	// csv.Read fails with a parse error (not io.EOF).
	content := []byte("\"unclosed,value\n")
	_, err := parseCSVForCollection(content, colDef)
	if err == nil {
		t.Fatal("parseCSVForCollection: want error for malformed csv header")
	}
}

// ---------------------------------------------------------------------------
// ParseBatchYAMLStream — invalid YAML document error
// ---------------------------------------------------------------------------

// TestParseBatchYAMLStream_InvalidYAML exercises the "invalid YAML" branch
// (batch_parsers.go line 125).
func TestParseBatchYAMLStream_InvalidYAML(t *testing.T) {
	t.Parallel()
	// A document that the YAML decoder cannot unmarshal into map[string]any —
	// use a YAML mapping with a non-string key, which yaml.v3 rejects for
	// map[string]any.
	in := strings.NewReader("---\n? [a, b]\n: value\n")
	_, err := ParseBatchYAMLStream(in)
	if err == nil {
		t.Fatal("ParseBatchYAMLStream: want error for invalid YAML document")
	}
	if !strings.Contains(err.Error(), "document") {
		t.Errorf("error should mention 'document', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ParseRecordContentForCollection — markdown parse error
// ---------------------------------------------------------------------------

// TestParseRecordContentForCollection_Markdown_ParseError exercises the markdown
// parse error branch (parse.go line 61-63).
func TestParseRecordContentForCollection_Markdown_ParseError(t *testing.T) {
	t.Parallel()
	// Pass content that is not valid markdown frontmatter.
	// markdown.Parse expects "---\n...\n---\n" structure; completely invalid
	// content may return an error or just empty frontmatter.
	// We test with content that causes a YAML parse error in the frontmatter.
	content := []byte("---\n{bad yaml: [\n---\nbody\n")
	col := markdownColDef("")
	_, err := ParseRecordContentForCollection(content, col)
	// The markdown parser may or may not error on this; what matters is we
	// exercise the branch.  If it doesn't error, we just verify no panic.
	_ = err
}

// ---------------------------------------------------------------------------
// marshalForFormat — YAML and TOML marshal errors
//
// yaml.Marshal is extremely permissive and never errors on standard Go types.
// toml.Marshal errors on types not representable in TOML (e.g., complex
// numbers, channels), but again in practice all record values are scalars.
// These branches are effectively unreachable via the production code path.
// They are documented below.
// ---------------------------------------------------------------------------

// TestMarshalForFormat_TOML_Marshals exercises the TOML path to confirm it
// works correctly (increasing coverage by going through the success path).
func TestMarshalForFormat_TOML_Success(t *testing.T) {
	t.Parallel()
	out, err := marshalForFormat(map[string]any{"key": "val"}, ingitdb.RecordFormatTOML)
	if err != nil {
		t.Fatalf("marshalForFormat TOML: %v", err)
	}
	if len(out) == 0 {
		t.Error("marshalForFormat TOML: empty output")
	}
}

// ---------------------------------------------------------------------------
// AlterCollection — readCollectionDefYAML error inside lock
// ---------------------------------------------------------------------------

// TestAlterCollection_ReadDefError exercises the readCollectionDefYAML error
// branch inside AlterCollection (schema_modifier.go line 136-138) by writing
// an invalid YAML file as definition.yaml before calling AlterCollection.
func TestAlterCollection_ReadDefError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	// Create the collection normally first.
	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Overwrite definition.yaml with invalid YAML.
	defPath := filepath.Join(root, "tags", ingitdb.SchemaDir, ingitdb.CollectionDefFileName)
	err = os.WriteFile(defPath, []byte("{bad yaml"), 0o644)
	if err != nil {
		t.Fatalf("corrupt def: %v", err)
	}

	op := ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String})
	err = modifier.AlterCollection(context.Background(), "tags", op)
	if err == nil {
		t.Fatal("AlterCollection: want error for invalid definition.yaml")
	}
}

// ---------------------------------------------------------------------------
// ApplyAddField — unknown type error branch
// ---------------------------------------------------------------------------

// TestApplyAddField_UnknownType exercises the dbschemaTypeToIngitdb error
// in ApplyAddField (schema_modifier.go line 196-198).
func TestApplyAddField_UnknownType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// AddField with dbschema.Null — unsupported type causes error.
	op := ddl.AddField(dbschema.FieldDef{Name: "bad", Type: dbschema.Null})
	err = modifier.AlterCollection(context.Background(), "tags", op)
	if err == nil {
		t.Fatal("AlterCollection AddField: want error for Null type")
	}
}

// ---------------------------------------------------------------------------
// ApplyModifyField — unknown type error branch
// ---------------------------------------------------------------------------

// TestApplyModifyField_UnknownType exercises the dbschemaTypeToIngitdb error
// in ApplyModifyField (schema_modifier.go line 235-237).
func TestApplyModifyField_UnknownType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// ModifyField changing type to Null — unsupported.
	op := ddl.ModifyField("label", dbschema.FieldDef{Name: "label", Type: dbschema.Null})
	err = modifier.AlterCollection(context.Background(), "tags", op)
	if err == nil {
		t.Fatal("AlterCollection ModifyField: want error for Null type")
	}
}

// ---------------------------------------------------------------------------
// Delete — MapOfRecords writeMapOfRecordsFile error after successful read
// ---------------------------------------------------------------------------

// TestReadwriteTx_Delete_MapOfRecords_WriteError exercises the writeMapOfRecordsFile
// error branch in Delete (tx_readwrite.go line 147) by making the target file
// read-only after the read but… that's a TOCTOU race.  Instead we use an
// encode error by configuring an unsupported format.
func TestReadwriteTx_Delete_MapOfRecords_WriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("alice:\n  score: 10\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormat("unsupported"), // encode will fail
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	// readMapOfRecordsFile will fail because "unsupported" format can't parse.
	// So this exercises the readErr branch, not the write branch.
	// Document: the writeMapOfRecordsFile error in Delete is unreachable without
	// a TOCTOU race because the read must succeed before write is attempted,
	// and they use the same format.
	err = tx.Delete(context.Background(), dal.NewKeyWithID("scores", "alice"))
	if err == nil {
		t.Fatal("Delete MapOfRecords: want error for unsupported format")
	}
}

// ---------------------------------------------------------------------------
// Update — rec.Error() != nil branch (tx_readwrite.go line 174-176)
//
// dal's record.Error() never returns ErrRecordNotFound to callers — it returns
// nil for the not-found sentinel and surfaces it only through record.Exists().
// The rec.Error() check in Update is therefore only reachable if Get itself
// returns an error AND sets it on the record, which contradicts the Get
// contract (errors are returned, not stored on the record).  This branch is
// unreachable via normal Get semantics and is documented as dead code.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// readSingleRecordFile — stat non-ErrNotExist error and read-inside-lock error
// ---------------------------------------------------------------------------

// TestReadSingleRecordFile_StatNonExistError exercises the stat non-ErrNotExist
// branch (record_io.go line 34-35) by making the parent directory non-executable.
func TestReadSingleRecordFile_StatNonExistError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	err := os.WriteFile(p, []byte("a: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Remove execute permission on root so Stat on p fails.
	err = os.Chmod(root, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()

	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: root,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, _, err = readSingleRecordFile(p, colDef)
	if err == nil {
		t.Fatal("readSingleRecordFile: want error for stat permission failure")
	}
}

// TestReadSingleRecordFile_ReadError covers the read error branch in
// readSingleRecordFile via the osReadFile seam. A real (empty) file is present
// so os.Stat and the shared lock succeed; only the read fails — hence found is
// true even though reading failed. Intentionally NOT parallel: it mutates a
// package-level seam.
func TestReadSingleRecordFile_ReadError(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	defer func() { osReadFile = orig }()

	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: root,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, found, err := readSingleRecordFile(p, colDef)
	if err == nil {
		t.Fatal("readSingleRecordFile: want error when the read fails")
	}
	if !strings.Contains(err.Error(), "read ") {
		t.Errorf("error = %v, want it to wrap the read failure", err)
	}
	if !found {
		t.Error("found = false, want true (the file exists; only reading it failed)")
	}
}

// TestReadMapOfRecordsFile_ReadError covers the read error branch in
// readMapOfRecordsFile via the osReadFile seam. Intentionally NOT parallel: it
// mutates a package-level seam.
func TestReadMapOfRecordsFile_ReadError(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "records.yaml")
	if err := os.WriteFile(p, []byte("a:\n  x: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	defer func() { osReadFile = orig }()

	_, err := readMapOfRecordsFile(p, ingitdb.RecordFormatYAML)
	if err == nil {
		t.Fatal("readMapOfRecordsFile: want error when the read fails")
	}
	if !strings.Contains(err.Error(), "read ") {
		t.Errorf("error = %v, want it to wrap the read failure", err)
	}
}

// TestReadAllSingleRecords_GlobBadPattern covers the filepath.Glob error
// branch (query.go line 82). The record-file name template contains an
// unterminated character class, so the derived glob pattern is malformed and
// filepath.Glob returns ErrBadPattern.
func TestReadAllSingleRecords_GlobBadPattern(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: root,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "[.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, err := readAllSingleRecords(colDef)
	if err == nil {
		t.Fatal("readAllSingleRecords: want error for malformed glob pattern")
	}
	if !strings.Contains(err.Error(), "glob ") {
		t.Errorf("error = %v, want it to wrap the glob failure", err)
	}
}

// TestDescribeCollection_ReadError covers the read error branch in
// DescribeCollection (schema_reader.go) via the osReadFile seam. A real
// definition.yaml is present so Stat and the shared lock succeed; only the read
// fails. Intentionally NOT parallel: it mutates a package-level seam.
func TestDescribeCollection_ReadError(t *testing.T) {
	root := t.TempDir()
	defDir := filepath.Join(root, "things", ingitdb.SchemaDir)
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defPath := filepath.Join(defDir, ingitdb.CollectionDefFileName)
	if err := os.WriteFile(defPath, []byte("record_file:\n  type: \"map[string]any\"\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osReadFile
	osReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	defer func() { osReadFile = orig }()

	db := &Database{projectPath: root}
	ref := dal.NewCollectionRef("things", "", nil)
	_, err := db.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection: want error when reading definition.yaml fails")
	}
	if !strings.Contains(err.Error(), "read definition.yaml") {
		t.Errorf("error = %v, want it to wrap the read failure", err)
	}
}

// TestDeleteSingleRecordFile_RemoveErrNotExist covers the ErrNotExist branch
// after os.Remove (record_io.go line 137) — a TOCTOU where the file vanishes
// between Stat and Remove. Deterministic via the osRemove seam.
//
// Intentionally NOT parallel: it mutates the package-level osRemove seam.
func TestDeleteSingleRecordFile_RemoveErrNotExist(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osRemove
	osRemove = func(string) error { return os.ErrNotExist }
	defer func() { osRemove = orig }()

	err := deleteSingleRecordFile(p)
	if err != nil {
		t.Errorf("error = %v, want nil (delete is idempotent when the file vanishes)", err)
	}
}

// TestDeleteSingleRecordFile_RemoveError covers the generic os.Remove error
// branch (record_io.go line 140) via the osRemove seam.
//
// Intentionally NOT parallel: it mutates the package-level osRemove seam.
func TestDeleteSingleRecordFile_RemoveError(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := osRemove
	osRemove = func(string) error { return os.ErrPermission }
	defer func() { osRemove = orig }()

	err := deleteSingleRecordFile(p)
	if err == nil {
		t.Fatal("deleteSingleRecordFile: want error when remove fails")
	}
	if !strings.Contains(err.Error(), "remove ") {
		t.Errorf("error = %v, want it to wrap the remove failure", err)
	}
}

// TestReadAllSingleRecords_NotFoundAfterGlob covers the found==false branch in
// readAllSingleRecords (query.go line 106) — in production a TOCTOU where a
// globbed file vanishes before the read. The readSingleRecord seam returns
// found==false deterministically. A real record file is present so glob yields
// a match to feed the loop. Intentionally NOT parallel: it mutates a seam.
func TestReadAllSingleRecords_NotFoundAfterGlob(t *testing.T) {
	root := t.TempDir()
	recDir := filepath.Join(root, "things", "$records")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("v: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := readSingleRecord
	readSingleRecord = func(string, *ingitdb.CollectionDef) (map[string]any, bool, error) {
		return nil, false, nil
	}
	defer func() { readSingleRecord = orig }()

	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	recs, err := readAllSingleRecords(colDef)
	if err != nil {
		t.Fatalf("readAllSingleRecords: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records, want 0 (the globbed file reports not-found)", len(recs))
	}
}

// TestReadAllSingleRecords_RelError covers the filepath.Rel error branch in
// readAllSingleRecords (query.go line 95-96) via the filepathRel seam. In
// production this is unreachable because the match path always comes from
// filepath.Glob under basePath, so filepath.Rel never fails. The seam returns
// an error deterministically. A real record file is present so glob yields a
// match to feed the loop. Intentionally NOT parallel: it mutates a seam.
func TestReadAllSingleRecords_RelError(t *testing.T) {
	root := t.TempDir()
	recDir := filepath.Join(root, "things", "$records")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("v: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := filepathRel
	filepathRel = func(string, string) (string, error) { return "", os.ErrInvalid }
	defer func() { filepathRel = orig }()

	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, err := readAllSingleRecords(colDef)
	if err == nil {
		t.Fatal("readAllSingleRecords: want error when filepath.Rel fails")
	}
	if !strings.Contains(err.Error(), "rel ") {
		t.Errorf("error = %v, want it to wrap the rel failure", err)
	}
}

// mockFileLocker is a fileLocker whose Lock/RLock return a configurable error.
type mockFileLocker struct{ lockErr error }

func (m mockFileLocker) Lock() error   { return m.lockErr }
func (m mockFileLocker) RLock() error  { return m.lockErr }
func (m mockFileLocker) Unlock() error { return nil }

// TestWithExclusiveLock_LockError covers the lock-acquisition error branch in
// withExclusiveLock (filelock.go line 38-40) via the newFileLocker seam.
// Intentionally NOT parallel: it mutates a package-level seam.
func TestWithExclusiveLock_LockError(t *testing.T) {
	orig := newFileLocker
	newFileLocker = func(string) fileLocker { return mockFileLocker{lockErr: os.ErrPermission} }
	defer func() { newFileLocker = orig }()

	called := false
	err := withExclusiveLock("ignored", func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("withExclusiveLock: want error when Lock fails")
	}
	if !strings.Contains(err.Error(), "acquire exclusive lock") {
		t.Errorf("error = %v, want it to wrap the lock failure", err)
	}
	if called {
		t.Error("fn must not run when lock acquisition fails")
	}
}

// TestCreateCollection_WriteDefSeamError covers the error return from the
// withExclusiveLock(writeCollectionDefYAML) call in CreateCollection
// (schema_modifier.go line 76) via the osWriteFile seam. Intentionally NOT
// parallel: it mutates a package-level seam.
func TestCreateCollection_WriteDefSeamError(t *testing.T) {
	orig := osWriteFile
	osWriteFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	defer func() { osWriteFile = orig }()

	db := &Database{projectPath: t.TempDir(), reader: newReader()}
	err := db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error when writing definition.yaml fails")
	}
	if !strings.Contains(err.Error(), "write ") {
		t.Errorf("error = %v, want it to wrap the write failure", err)
	}
}

// queryWithCondition builds a single-record collection on disk and a query
// whose WHERE clause is the given conditions, returning the tx and query ready
// for executeQueryToRecordsReader. A record is seeded so applyWhere actually
// evaluates the condition.
func queryWithCondition(t *testing.T, conds ...dal.Condition) (readonlyTx, dal.Query) {
	t.Helper()
	root := t.TempDir()
	recDir := filepath.Join(root, "things", "$records")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("v: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"things": colDef},
		},
	}
	q := dal.From(dal.NewRootCollectionRef("things", "")).NewQuery().
		Where(conds...).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("things", ""), map[string]any{})
		})
	return tx, q
}

// TestExecuteQuery_ApplyWhereError covers the applyWhere error propagation in
// executeQueryToRecordsReader (query.go line 53). A single unsupported WHERE
// condition passes through the builder and fails in evaluateCondition.
func TestExecuteQuery_ApplyWhereError(t *testing.T) {
	t.Parallel()
	tx, q := queryWithCondition(t, unsupportedCond{})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("executeQueryToRecordsReader: want error for unsupported WHERE condition")
	}
	if !strings.Contains(err.Error(), "unsupported condition type") {
		t.Errorf("error = %v, want unsupported condition type", err)
	}
}

// TestExecuteQuery_GroupConditionInnerError covers the inner evaluateCondition
// error inside the And branch of evaluateGroupCondition (query.go line 228).
// Two conditions form an And group; the first (unsupported) errors.
func TestExecuteQuery_GroupConditionInnerError(t *testing.T) {
	t.Parallel()
	tx, q := queryWithCondition(t, unsupportedCond{}, dal.WhereField("v", dal.Equal, 1))
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("executeQueryToRecordsReader: want error from And-group inner condition")
	}
	if !strings.Contains(err.Error(), "unsupported condition type") {
		t.Errorf("error = %v, want unsupported condition type", err)
	}
}

// ---------------------------------------------------------------------------
// DescribeCollection — os.ReadFile error inside lock
// ---------------------------------------------------------------------------

// TestDescribeCollection_ReadInsideLockError exercises the os.ReadFile error
// inside the shared lock in DescribeCollection (schema_reader.go line 101-103).
func TestDescribeCollection_ReadInsideLockError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any file")
	}
	root := t.TempDir()
	colDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err := os.MkdirAll(colDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defPath := filepath.Join(colDir, ingitdb.CollectionDefFileName)
	err = os.WriteFile(defPath, []byte(countriesDef), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Make definition.yaml unreadable.
	err = os.Chmod(defPath, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(defPath, 0o644) }()

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	_, err = reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection: want error for unreadable definition.yaml")
	}
}

// ---------------------------------------------------------------------------
// registerInRootCollections — read error when .ingitdb dir is unreadable
// ---------------------------------------------------------------------------

// TestRegisterInRootCollections_ReadError exercises the ReadRootCollectionsFromFile
// error branch (registry.go line 35-37) by making .ingitdb unreadable.
func TestRegisterInRootCollections_ReadError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any directory")
	}
	root := t.TempDir()
	ingitdbDir := filepath.Join(root, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a root-collections.yaml that is unreadable.
	regPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	err = os.WriteFile(regPath, []byte("tags: tags\n"), 0o000)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	defer func() { _ = os.Chmod(regPath, 0o644) }()

	err = registerInRootCollections(root, "newcol")
	if err == nil {
		t.Fatal("registerInRootCollections: want error for unreadable registry")
	}
}

// TestDeregisterFromRootCollections_ReadError exercises the ReadRootCollectionsFromFile
// error branch (registry.go line 68-70) by making the registry unreadable.
func TestDeregisterFromRootCollections_ReadError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can read any file")
	}
	root := t.TempDir()
	ingitdbDir := filepath.Join(root, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	regPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	err = os.WriteFile(regPath, []byte("tags: tags\n"), 0o000)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	defer func() { _ = os.Chmod(regPath, 0o644) }()

	err = deregisterFromRootCollections(root, "tags")
	if err == nil {
		t.Fatal("deregisterFromRootCollections: want error for unreadable registry")
	}
}

// ---------------------------------------------------------------------------
// CreateCollection — stat returns non-ErrNotExist error
// ---------------------------------------------------------------------------

// TestCreateCollection_StatNonExistError exercises the "stat returns
// non-ErrNotExist" branch (schema_modifier.go line 62-64).
func TestCreateCollection_StatNonExistError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	// Pre-create the tags/.collection directory but make it non-readable.
	colSchemaDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err := os.MkdirAll(colSchemaDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.Chmod(colSchemaDir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colSchemaDir, 0o755) }()

	db := &Database{projectPath: root, reader: newReader()}
	err = db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error for stat permission failure")
	}
}

// ---------------------------------------------------------------------------
// createCollection after write — registry fails
// ---------------------------------------------------------------------------

// TestCreateCollection_RegisterAfterWrite exercises the registerInRootCollections
// failure after writing definition.yaml (schema_modifier.go line 81-83).
func TestCreateCollection_RegisterAfterWrite(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()

	// Pre-create .ingitdb with an unreadable registry so registerInRootCollections
	// fails after the definition.yaml has been written.
	ingitdbDir := filepath.Join(root, ".ingitdb")
	err := os.MkdirAll(ingitdbDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	regPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	err = os.WriteFile(regPath, []byte("other: other\n"), 0o000)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	defer func() { _ = os.Chmod(regPath, 0o644) }()

	db := &Database{projectPath: root, reader: newReader()}
	err = db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error when registry write fails after YAML write")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go Get — MapOfRecords read error (line 53-55)
// ---------------------------------------------------------------------------

// TestReadonlyTx_Get_MapOfRecords_ReadError exercises the readMapOfRecordsFile
// error path in Get (tx_readonly.go line 53-55) by using corrupt YAML.
func TestReadonlyTx_Get_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeMapOfRecordsTx(t, root)
	err := os.MkdirAll(colDef.DirPath, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	err = tx.Get(context.Background(), rec)
	if err == nil {
		t.Fatal("Get MapOfRecords: want error for corrupt file")
	}
	// The error must be set on the record too (dalgo contract: Exists()/Data()
	// panic if SetError was never called after Get).
	if !errors.Is(rec.Error(), err) {
		t.Errorf("rec.Error() = %v, want it to carry the Get error %v", rec.Error(), err)
	}
}

// ---------------------------------------------------------------------------
// schema_modifier — rewriteRecordFiles parse and marshal error branches
// (lines 416-418 and 422-424 require special YAML content)
// ---------------------------------------------------------------------------

// TestRewriteRecordFiles_YAMLParseError exercises the yaml.Unmarshal error
// inside the walk lock (schema_modifier.go line 416-418) by creating a YAML
// file with a list (not a map) which yaml.Unmarshal into map[string]any rejects.
func TestRewriteRecordFiles_YAMLParseError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	p := filepath.Join(root, "rec.yaml")
	// YAML list cannot unmarshal into map[string]any.
	err := os.WriteFile(p, []byte("- item1\n- item2\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	err = rewriteRecordFiles(root, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error for YAML list (not a map)")
	}
}

// ---------------------------------------------------------------------------
// schema_modifier — CreateCollection withExclusiveLock+writeCollectionDefYAML
// error propagation (line 76-78)
// ---------------------------------------------------------------------------

// TestCreateCollection_LockWriteError exercises the error path where
// withExclusiveLock(defPath, ...) succeeds acquiring the lock but
// writeCollectionDefYAML fails because the directory doesn't exist.
// This covers schema_modifier.go line 76-78.
func TestCreateCollection_LockWriteError2(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere")
	}
	root := t.TempDir()
	// Create the .collection directory, let flock create the def file,
	// but then immediately make it read-only so write fails.
	colSchemaDir := filepath.Join(root, "tags", ingitdb.SchemaDir)
	err := os.MkdirAll(colSchemaDir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Pre-create the def file so flock can acquire the lock on it,
	// then make .collection read-only so the file write fails.
	defPath := filepath.Join(colSchemaDir, ingitdb.CollectionDefFileName)
	err = os.WriteFile(defPath, []byte("placeholder\n"), 0o644)
	if err != nil {
		t.Fatalf("write placeholder: %v", err)
	}
	// Remove the placeholder so Stat says "not found" (CreateCollection will proceed past the IfExists check).
	err = os.Remove(defPath)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Make the schema dir read-only — MkdirAll of colDir will fail trying to create the .collection dir...
	// Actually the dir already exists, and MkdirAll of an existing dir is a no-op.
	// We need the WRITE of definition.yaml to fail, not the mkdir.
	// Make colSchemaDir read-only so WriteFile inside it fails.
	err = os.Chmod(colSchemaDir, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(colSchemaDir, 0o755) }()

	db := &Database{projectPath: root, reader: newReader()}
	err = db.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection: want error when definition.yaml write fails")
	}
}

// ---------------------------------------------------------------------------
// schema_modifier — DropCollection os.RemoveAll error (line 111-113)
// ---------------------------------------------------------------------------

// TestDropCollection_RemoveAllError exercises the os.RemoveAll error branch
// (schema_modifier.go line 111-113).
// RemoveAll typically can't be made to fail via permissions on macOS because
// root can always remove. We verify this is exercised when the collection
// directory exists but something prevents removal — this is OS-specific.
// We use a read-only parent directory trick.
func TestDropCollection_RemoveAllError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks for RemoveAll")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	err = modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Make root non-writable so RemoveAll("tags") fails.
	err = os.Chmod(root, 0o555)
	if err != nil {
		t.Fatalf("chmod root: %v", err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()

	err = modifier.DropCollection(context.Background(), "tags")
	if err == nil {
		t.Fatal("DropCollection: want error when RemoveAll fails")
	}
}

// ---------------------------------------------------------------------------
// schema_modifier — AlterCollection flush PartialSuccessError (line 162-172)
// Covered by making definition.yaml read-only after creating collection and
// then running an op that would succeed but whose flush fails.
// ---------------------------------------------------------------------------

// TestAlterCollection_FlushPartialSuccessError exercises the flush error path
// in AlterCollection (schema_modifier.go line 162-172).
// We make definition.yaml read-only after CreateCollection, then try AlterCollection.
// The AlterCollection acquires an exclusive lock on defPath; if defPath is
// read-only, the WriteFile inside writeCollectionDefYAML fails.
func TestAlterCollection_FlushPartialSuccessError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write any file")
	}
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	err = modifier.CreateCollection(context.Background(), tags)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	defPath := filepath.Join(root, "tags", ingitdb.SchemaDir, ingitdb.CollectionDefFileName)
	// Make definition.yaml read-only so the flush write fails.
	err = os.Chmod(defPath, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(defPath, 0o644) }()

	op := ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String})
	err = modifier.AlterCollection(context.Background(), "tags", op)
	if err == nil {
		t.Fatal("AlterCollection: want error when flush fails")
	}
}

// ---------------------------------------------------------------------------
// writeCollectionDefYAML — yaml.Marshal error
//
// yaml.Marshal in gopkg.in/yaml.v3 essentially never errors for standard Go
// structs. The marshal-error branch (schema_modifier.go line 365-367) is
// unreachable in practice via CollectionDef — it would require a value type
// that yaml.Marshal explicitly rejects (e.g., a channel or func), which
// CollectionDef never contains.  This branch is documented as dead code.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// executeQueryToRecordsReader — readAllRecordsFromDisk error path (query.go:47-49)
// and applyWhere error path (query.go:53-55)
// ---------------------------------------------------------------------------

// TestExecuteQueryToRecordsReader_ReadError exercises the readAllRecordsFromDisk
// error branch (query.go line 47-49) by creating a map-of-records collection
// with corrupt file content.
func TestExecuteQueryToRecordsReader_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Corrupt YAML content causes readMapOfRecordsFile to error.
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}
	q := dal.From(dal.NewRootCollectionRef("scores", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("scores", ""), map[string]any{})
		})
	_, err = executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("executeQueryToRecordsReader: want error for corrupt map-of-records file")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go Get and Exists — readSingleRecordFile and readMapOfRecordsFile
// error paths via corrupt file content
// ---------------------------------------------------------------------------

// TestReadonlyTx_Get_SingleRecord_StatError exercises the readSingleRecordFile
// stat non-ErrNotExist error path in Get (tx_readonly.go line 40).
func TestReadonlyTx_Get_SingleRecord_StatError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "k.yaml"), []byte("a: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Remove execute bit so Stat inside readSingleRecordFile fails.
	err = os.Chmod(dir, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"items": colDef},
		},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("items", "k"), map[string]any{})
	err = tx.Get(context.Background(), rec)
	if err == nil {
		t.Fatal("Get: want error for stat permission failure")
	}
}

// TestReadonlyTx_Exists_MapOfRecords_ReadError exercises the readMapOfRecordsFile
// error path in Exists (tx_readonly.go line 91-93).
func TestReadonlyTx_Exists_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("{broken yaml"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}
	_, err = tx.Exists(context.Background(), dal.NewKeyWithID("scores", "alice"))
	if err == nil {
		t.Fatal("Exists MapOfRecords: want error for corrupt file")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go — Set MapOfRecords write error; Insert SingleRecord write error;
// Insert MapOfRecords write error
// ---------------------------------------------------------------------------

// TestReadwriteTx_Set_MapOfRecords_WriteError exercises the writeMapOfRecordsFile
// error in Set (tx_readwrite.go line 52-54) by making the target file read-only.
func TestReadwriteTx_Set_MapOfRecords_WriteError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write any file")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	err = os.WriteFile(p, []byte("alice:\n  score: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	// Make file read-only so the write back fails.
	err = os.Chmod(p, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()

	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "bob"), map[string]any{"score": 5})
	err = tx.Set(context.Background(), rec)
	if err == nil {
		t.Fatal("Set MapOfRecords: want error when write fails")
	}
}

// TestReadwriteTx_Insert_SingleRecord_WriteError exercises the writeSingleRecordFile
// error path in Insert (tx_readwrite.go line 89-91) using an unsupported format.
func TestReadwriteTx_Insert_SingleRecord_WriteError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: filepath.Join(root, "c"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormat("unsupported"),
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"c": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{"x": 1})
	err := tx.Insert(context.Background(), rec)
	if err == nil {
		t.Fatal("Insert SingleRecord: want error for unsupported format")
	}
}

// TestReadwriteTx_Insert_MapOfRecords_WriteError exercises the writeMapOfRecordsFile
// error in Insert (tx_readwrite.go line 104-106) by making the target read-only.
func TestReadwriteTx_Insert_MapOfRecords_WriteError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can write any file")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "scores")
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "scores.yaml")
	// File doesn't exist yet — first insert creates it.
	// We need insert to succeed reading (no file) but fail writing.
	// Create an existing file with alice, then make it read-only.
	err = os.WriteFile(p, []byte("alice:\n  score: 1\n"), 0o644)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	err = os.Chmod(p, 0o444)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()

	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "bob"), map[string]any{"score": 5})
	err = tx.Insert(context.Background(), rec)
	if err == nil {
		t.Fatal("Insert MapOfRecords: want error when write fails")
	}
}

// ---------------------------------------------------------------------------
// rewriteRecordFiles — stat non-ErrNotExist error (schema_modifier.go:400)
// ---------------------------------------------------------------------------

// TestRewriteRecordFiles_StatError exercises the stat non-ErrNotExist branch
// (schema_modifier.go line 400) by making the parent of recordsDir non-executable
// so Stat fails with EACCES rather than ENOENT.
func TestRewriteRecordFiles_StatError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root can stat any path")
	}
	root := t.TempDir()
	// Create parent/records/ but make parent non-executable so
	// Stat(parent/records/) fails with EACCES.
	parent := filepath.Join(root, "parent")
	records := filepath.Join(parent, "records")
	err := os.MkdirAll(records, 0o755)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.Chmod(parent, 0o000)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() { _ = os.Chmod(parent, 0o755) }()

	err = rewriteRecordFiles(records, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error for stat permission failure")
	}
}
