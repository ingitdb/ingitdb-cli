package dalgo2fsingitdb

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeOrGroupCondition builds a dal.GroupCondition with operator=Or wrapping
// the given conditions. dal.GroupCondition has unexported fields so we use
// reflect+unsafe to set them. This helper is only used in tests.
func makeOrGroupCondition(conditions ...dal.Condition) dal.GroupCondition {
	gc := dal.GroupCondition{}
	v := reflect.ValueOf(&gc).Elem()
	// field 0: operator Operator
	op := v.Field(0)
	opPtr := (*dal.Operator)(unsafe.Pointer(op.UnsafeAddr()))
	*opPtr = dal.Or
	// field 1: conditions []Condition
	conds := v.Field(1)
	condsPtr := (*[]dal.Condition)(unsafe.Pointer(conds.UnsafeAddr()))
	*condsPtr = conditions
	return gc
}

// ---------------------------------------------------------------------------
// sliceRecordsReader.Cursor
// ---------------------------------------------------------------------------

func TestSliceRecordsReader_Cursor(t *testing.T) {
	t.Parallel()
	r := newSliceRecordsReader(nil)
	cursor, err := r.Cursor()
	if err != nil {
		t.Fatalf("Cursor(): unexpected error: %v", err)
	}
	if cursor != "" {
		t.Errorf("Cursor() = %q, want empty string", cursor)
	}
}

// ---------------------------------------------------------------------------
// extractBody – []byte and default branches
// ---------------------------------------------------------------------------

func TestExtractBody_ByteSlice(t *testing.T) {
	t.Parallel()
	data := map[string]any{"body": []byte("hello")}
	got := extractBody(data, "body")
	if string(got) != "hello" {
		t.Errorf("extractBody []byte = %q, want %q", got, "hello")
	}
}

func TestExtractBody_OtherType(t *testing.T) {
	t.Parallel()
	data := map[string]any{"body": 42}
	got := extractBody(data, "body")
	if string(got) != "42" {
		t.Errorf("extractBody int = %q, want %q", got, "42")
	}
}

func TestExtractBody_Nil(t *testing.T) {
	t.Parallel()
	data := map[string]any{"body": nil}
	got := extractBody(data, "body")
	if got != nil {
		t.Errorf("extractBody nil = %v, want nil", got)
	}
}

func TestExtractBody_Missing(t *testing.T) {
	t.Parallel()
	data := map[string]any{}
	got := extractBody(data, "body")
	if got != nil {
		t.Errorf("extractBody missing = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// toFloat64 – cover all numeric types
// ---------------------------------------------------------------------------

func TestToFloat64_AllNumericTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"int", int(5), 5, true},
		{"int8", int8(5), 5, true},
		{"int16", int16(5), 5, true},
		{"int32", int32(5), 5, true},
		{"int64", int64(5), 5, true},
		{"uint", uint(5), 5, true},
		{"uint8", uint8(5), 5, true},
		{"uint16", uint16(5), 5, true},
		{"uint32", uint32(5), 5, true},
		{"uint64", uint64(5), 5, true},
		{"float32", float32(5), 5, true},
		{"float64", float64(5), 5, true},
		{"string", "abc", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotOK := toFloat64(tc.input)
			if gotOK != tc.ok {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tc.input, gotOK, tc.ok)
			}
			if tc.ok && got != tc.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// compareValues – mixed numeric/non-numeric
// ---------------------------------------------------------------------------

func TestCompareValues_MixedTypes(t *testing.T) {
	t.Parallel()
	// One numeric, one string – should fall back to string comparison.
	got := compareValues(42, "abc")
	// Both are converted to strings: "42" vs "abc"
	if got == 0 {
		t.Error("compareValues(42, \"abc\") should not be 0")
	}
}

// ---------------------------------------------------------------------------
// evaluateCondition – unsupported condition type
// ---------------------------------------------------------------------------

type unsupportedCondition struct{}

func (unsupportedCondition) String() string { return "unsupported" }

func TestEvaluateCondition_UnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := evaluateCondition(unsupportedCondition{}, map[string]any{}, "key")
	if err == nil {
		t.Fatal("evaluateCondition: expected error for unsupported condition type, got nil")
	}
}

// ---------------------------------------------------------------------------
// evaluateGroupCondition – AND, OR, and default
// ---------------------------------------------------------------------------

func TestEvaluateGroupCondition_AND(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "Alice", "score": float64(80)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "Bob", "score": float64(60)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// AND: name="Alice" AND score>70 → only "a"
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.WhereField("name", dal.Equal, "Alice")
	qb.WhereField("score", dal.GreaterThen, float64(70))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})

	var keys []string
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }()
		for {
			rec, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			keys = append(keys, rec.Key().ID.(string))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(keys) != 1 || keys[0] != "a" {
		t.Errorf("AND filter: got %v, want [a]", keys)
	}
}

// TestEvaluateGroupCondition_OR tests the Or branch of evaluateGroupCondition
// by using the dal.QueryBuilder's WhereField called twice (which produces an And
// GroupCondition) to extract its structure, then re-invoking evaluateCondition
// directly with a condition that has operator=Or.
// Since dal.GroupCondition has unexported fields and no public Or constructor,
// we test the Or path by calling evaluateGroupCondition directly using the
// orGroupCondition helper defined below.
func TestEvaluateGroupCondition_OR(t *testing.T) {
	t.Parallel()

	data := map[string]any{"name": "Alice", "score": float64(80)}

	// Build the OR group using the helper that constructs via reflection.
	gc := makeOrGroupCondition(
		dal.WhereField("name", dal.Equal, "Alice"),
		dal.WhereField("name", dal.Equal, "Carol"),
	)

	// "Alice" matches first condition → OR should return true.
	match, err := evaluateGroupCondition(gc, data, "k")
	if err != nil {
		t.Fatalf("evaluateGroupCondition OR: %v", err)
	}
	if !match {
		t.Error("evaluateGroupCondition OR: expected true for matching first condition")
	}

	// Neither condition matches → should return false.
	dataNone := map[string]any{"name": "Zara", "score": float64(10)}
	match, err = evaluateGroupCondition(gc, dataNone, "k")
	if err != nil {
		t.Fatalf("evaluateGroupCondition OR (no match): %v", err)
	}
	if match {
		t.Error("evaluateGroupCondition OR: expected false when no condition matches")
	}
}

// ---------------------------------------------------------------------------
// evaluateComparison – GreaterOrEqual, LessOrEqual, unsupported operator
// ---------------------------------------------------------------------------

func TestEvaluateComparison_Operators(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(70)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "B", "score": float64(80)})
	writeQueryFixture(t, filepath.Join(recordsDir, "c.yaml"), map[string]any{"name": "C", "score": float64(90)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	tests := []struct {
		name     string
		operator dal.Operator
		value    float64
		wantN    int
	}{
		{"GreaterOrEqual_80", dal.GreaterOrEqual, 80, 2},
		{"LessOrEqual_80", dal.LessOrEqual, 80, 2},
		{"LessThen_80", dal.LessThen, 80, 1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
			qb.WhereField("score", tc.operator, tc.value)
			q := qb.SelectIntoRecord(func() dal.Record {
				return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
			})
			var count int
			err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
				reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
				if err != nil {
					return err
				}
				defer func() { _ = reader.Close() }()
				for {
					_, nextErr := reader.Next()
					if nextErr != nil {
						break
					}
					count++
				}
				return nil
			})
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if count != tc.wantN {
				t.Errorf("%s: got %d records, want %d", tc.name, count, tc.wantN)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveExpression – unsupported expression type
// ---------------------------------------------------------------------------

type unsupportedExpr struct{}

func (unsupportedExpr) String() string { return "unsupported" }

func TestResolveExpression_UnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := resolveExpression(unsupportedExpr{}, map[string]any{}, "key")
	if err == nil {
		t.Fatal("resolveExpression: expected error for unsupported type, got nil")
	}
}

// ---------------------------------------------------------------------------
// readAllMapOfRecords via query (exercises readAllRecordsFromDisk for MapOfRecords)
// ---------------------------------------------------------------------------

func TestReadAllMapOfRecords_Query(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.tags",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "tags.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.tags": colDef},
	}
	// Write a map-of-records YAML file (top-level key = record ID).
	content := "alpha:\n  title: Alpha\nbeta:\n  title: Beta\n"
	if err := os.WriteFile(filepath.Join(dir, "tags.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.tags", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.tags", ""), map[string]any{})
	})

	var count int
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }()
		for {
			_, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 2 {
		t.Errorf("readAllMapOfRecords via query: got %d records, want 2", count)
	}
}

// TestReadAllMapOfRecords_EmptyFile covers the "file not found → nil" branch.
func TestReadAllMapOfRecords_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.tags",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "tags.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.tags": colDef},
	}
	// No file written — should return nil records.
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.tags", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.tags", ""), map[string]any{})
	})

	var count int
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }()
		for {
			_, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Errorf("readAllMapOfRecords empty file: got %d records, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// readAllRecordsFromDisk – unsupported record type
// ---------------------------------------------------------------------------

func TestReadAllRecordsFromDisk_UnsupportedRecordType(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: t.TempDir(),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: "unsupported_type",
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.items": colDef},
	}
	db := openTestDB(t, colDef.DirPath, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		_, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		return err
	})
	if err == nil {
		t.Fatal("expected error for unsupported record type, got nil")
	}
}

// ---------------------------------------------------------------------------
// readMarkdownRecord – parse error
// ---------------------------------------------------------------------------

func TestReadMarkdownRecord_ParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	colDef := def.Collections["test.notes"]

	// Write a markdown file whose frontmatter YAML is invalid.
	badMD := "---\ntitle: [unclosed\n---\nBody.\n"
	path := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(path, []byte(badMD), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := readMarkdownRecord(path, colDef)
	if err == nil {
		t.Fatal("readMarkdownRecord: expected parse error, got nil")
	}
}

// ---------------------------------------------------------------------------
// writeMarkdownRecord – write error (directory blocking file creation)
// ---------------------------------------------------------------------------

func TestWriteMarkdownRecord_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	colDef := def.Collections["test.notes"]

	// Create a directory at the target path to make WriteFile fail.
	target := filepath.Join(dir, "note.md")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	data := map[string]any{"title": "Test", ingitdb.DefaultMarkdownContentField: "body\n"}
	err := writeMarkdownRecord(target, colDef, data)
	if err == nil {
		t.Fatal("writeMarkdownRecord: expected write error, got nil")
	}
}

// ---------------------------------------------------------------------------
// writeMapOfRecordsFile – write error
// ---------------------------------------------------------------------------

func TestWriteMapOfRecordsFile_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a directory at the target file path to make WriteFile fail.
	target := filepath.Join(dir, "records.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	colDef := &ingitdb.CollectionDef{
		ID: "test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	data := map[string]map[string]any{"id1": {"field": "value"}}
	err := writeMapOfRecordsFile(target, colDef, data)
	if err == nil {
		t.Fatal("writeMapOfRecordsFile: expected write error, got nil")
	}
}

// ---------------------------------------------------------------------------
// executeQueryToRecordsReader – no definition (nil def path)
// ---------------------------------------------------------------------------

func TestExecuteQueryToRecordsReader_NilDef(t *testing.T) {
	t.Parallel()
	db, err := NewLocalDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDB: %v", err)
	}
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		_, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		return err
	})
	if err == nil {
		t.Fatal("expected error for nil definition, got nil")
	}
}

// ---------------------------------------------------------------------------
// buildKeyExtractor – fixed filename (no {key})
// ---------------------------------------------------------------------------

func TestBuildKeyExtractor_FixedFilename(t *testing.T) {
	t.Parallel()
	extractor, err := buildKeyExtractor("records.yaml")
	if err != nil {
		t.Fatalf("buildKeyExtractor: %v", err)
	}
	got := extractor("some/path/records.yaml")
	if got != "records" {
		t.Errorf("buildKeyExtractor fixed = %q, want %q", got, "records")
	}
}

// ---------------------------------------------------------------------------
// OrderBy with non-FieldRef expression (covers "!isRef" branch)
// ---------------------------------------------------------------------------

func TestExecuteQueryToRecordsReader_OrderByID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "B", "score": float64(2)})
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(1)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// Order by $id ascending.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.AscendingField("$id"))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})

	var keys []string
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }()
		for {
			rec, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			keys = append(keys, rec.Key().ID.(string))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(keys) < 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("order by $id: got %v, want [a b]", keys)
	}
}
