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

// makeGroupConditionWithOperator builds a dal.GroupCondition with an arbitrary operator.
func makeGroupConditionWithOperator(operator dal.Operator, conditions ...dal.Condition) dal.GroupCondition {
	gc := dal.GroupCondition{}
	v := reflect.ValueOf(&gc).Elem()
	op := v.Field(0)
	opPtr := (*dal.Operator)(unsafe.Pointer(op.UnsafeAddr()))
	*opPtr = operator
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

// TestEvaluateGroupCondition_AND_ErrorPropagation tests error propagation
// in the AND branch when a sub-condition fails.
func TestEvaluateGroupCondition_AND_ErrorPropagation(t *testing.T) {
	t.Parallel()
	gc := makeGroupConditionWithOperator(dal.And,
		unsupportedCondition{}, // triggers error
	)
	_, err := evaluateGroupCondition(gc, map[string]any{}, "k")
	if err == nil {
		t.Fatal("expected error from unsupported condition inside AND")
	}
}

// TestEvaluateGroupCondition_OR_ErrorPropagation tests error propagation
// in the OR branch when a sub-condition fails.
func TestEvaluateGroupCondition_OR_ErrorPropagation(t *testing.T) {
	t.Parallel()
	gc := makeGroupConditionWithOperator(dal.Or,
		unsupportedCondition{}, // triggers error
	)
	_, err := evaluateGroupCondition(gc, map[string]any{}, "k")
	if err == nil {
		t.Fatal("expected error from unsupported condition inside OR")
	}
}

// TestEvaluateGroupCondition_UnsupportedOperator tests the default branch.
func TestEvaluateGroupCondition_UnsupportedOperator(t *testing.T) {
	t.Parallel()
	gc := makeGroupConditionWithOperator("UNKNOWN_OP",
		dal.WhereField("name", dal.Equal, "x"),
	)
	_, err := evaluateGroupCondition(gc, map[string]any{}, "k")
	if err == nil {
		t.Fatal("expected error for unsupported group operator")
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

// TestEvaluateComparison_UnsupportedOperator tests the default branch of evaluateComparison.
func TestEvaluateComparison_UnsupportedOperator(t *testing.T) {
	t.Parallel()
	c := dal.NewComparison(
		dal.NewFieldRef("name"),
		"INVALID_OP",
		dal.NewConstant("x"),
	)
	_, err := evaluateComparison(c, map[string]any{"name": "test"}, "key")
	if err == nil {
		t.Fatal("expected error for unsupported operator")
	}
}

// TestEvaluateComparison_LeftExprError tests error when left expression is unsupported.
func TestEvaluateComparison_LeftExprError(t *testing.T) {
	t.Parallel()
	c := dal.NewComparison(
		unsupportedExpr{},
		dal.Equal,
		dal.NewConstant("x"),
	)
	_, err := evaluateComparison(c, map[string]any{}, "key")
	if err == nil {
		t.Fatal("expected error for unsupported left expression")
	}
}

// TestEvaluateComparison_RightExprError tests error when right expression is unsupported.
func TestEvaluateComparison_RightExprError(t *testing.T) {
	t.Parallel()
	c := dal.NewComparison(
		dal.NewFieldRef("name"),
		dal.Equal,
		unsupportedExpr{},
	)
	_, err := evaluateComparison(c, map[string]any{"name": "test"}, "key")
	if err == nil {
		t.Fatal("expected error for unsupported right expression")
	}
}

// ---------------------------------------------------------------------------
// resolveExpression – unsupported expression type and Constant
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

// TestResolveExpression_Constant tests the Constant branch.
func TestResolveExpression_Constant(t *testing.T) {
	t.Parallel()
	val, err := resolveExpression(dal.NewConstant(42), map[string]any{}, "key")
	if err != nil {
		t.Fatalf("resolveExpression Constant: %v", err)
	}
	if val != 42 {
		t.Errorf("resolveExpression Constant = %v, want 42", val)
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

// TestBuildKeyExtractor_NoMatch covers the regex-no-match → empty string branch.
func TestBuildKeyExtractor_NoMatch(t *testing.T) {
	t.Parallel()
	extractor, err := buildKeyExtractor("{key}.yaml")
	if err != nil {
		t.Fatalf("buildKeyExtractor: %v", err)
	}
	// Pass a path that doesn't match the *.yaml pattern.
	got := extractor("badfile.txt")
	if got != "" {
		t.Errorf("buildKeyExtractor no-match = %q, want empty string", got)
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

// ---------------------------------------------------------------------------
// readRecordFromFile – TOML parse error
// ---------------------------------------------------------------------------

func TestReadRecordFromFile_TOMLParseError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.toml")
	if err := os.WriteFile(path, []byte("= invalid\n[missing"), 0o644); err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	_, _, err := readRecordFromFile(path, ingitdb.RecordFormatTOML)
	if err == nil {
		t.Fatal("expected TOML parse error, got nil")
	}
	const prefix = "failed to parse TOML file"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// readMarkdownRecord – read permission error (non-ErrNotExist)
// ---------------------------------------------------------------------------

func TestReadMarkdownRecord_ReadPermError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	colDef := def.Collections["test.notes"]

	path := filepath.Join(dir, "noperm.md")
	if err := os.WriteFile(path, []byte("---\ntitle: X\n---\nBody\n"), 0o644); err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("setup: chmod: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0o644) }()

	_, _, err := readMarkdownRecord(path, colDef)
	if err == nil {
		t.Fatal("expected read permission error, got nil")
	}
	const prefix = "failed to read file"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// writeMarkdownRecord – MkdirAll error
// ---------------------------------------------------------------------------

func TestWriteMarkdownRecord_MkdirError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	colDef := def.Collections["test.notes"]

	// Create a file where we need a directory.
	blockingFile := filepath.Join(dir, "blocking")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}
	path := filepath.Join(blockingFile, "sub", "note.md")

	data := map[string]any{"title": "Test", ingitdb.DefaultMarkdownContentField: "body\n"}
	err := writeMarkdownRecord(path, colDef, data)
	if err == nil {
		t.Fatal("writeMarkdownRecord: expected mkdir error, got nil")
	}
	const prefix = "failed to create directory"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// writeMarkdownRecord – Serialize error (frontmatter value that fails YAML encoding)
// ---------------------------------------------------------------------------

func TestWriteMarkdownRecord_SerializeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	colDef := def.Collections["test.notes"]

	path := filepath.Join(dir, "bad.md")
	// errYAMLMarshalerValue (defined in record_file_error_test.go)
	// implements yaml.Marshaler and returns an error, causing
	// yaml.Node.Encode to propagate the error rather than panicking.
	data := map[string]any{
		"title":                             errYAMLMarshalerValue{},
		ingitdb.DefaultMarkdownContentField: "body\n",
	}
	err := writeMarkdownRecord(path, colDef, data)
	if err == nil {
		t.Fatal("writeMarkdownRecord: expected serialize error, got nil")
	}
	const prefix = "failed to serialize markdown record"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// writeRecordToFile – TOML marshal error
// ---------------------------------------------------------------------------

func TestWriteRecordToFile_TOMLMarshalError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	data := map[string]any{"key": make(chan int)}

	err := writeRecordToFile(path, ingitdb.RecordFormatTOML, data)
	if err == nil {
		t.Fatal("expected TOML marshal error, got nil")
	}
	const prefix = "failed to marshal data as TOML:"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// readMapOfRecordsFile – read permission error (non-ErrNotExist)
// ---------------------------------------------------------------------------

func TestReadMapOfRecordsFile_ReadPermError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.yaml")
	if err := os.WriteFile(path, []byte("id1:\n  field: value\n"), 0o644); err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("setup: chmod: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0o644) }()

	_, _, err := readMapOfRecordsFile(path, ingitdb.RecordFormatYAML)
	if err == nil {
		t.Fatal("expected read permission error, got nil")
	}
	const prefix = "failed to read file"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// writeMapOfRecordsFile – MkdirAll error
// ---------------------------------------------------------------------------

func TestWriteMapOfRecordsFile_MkdirError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocking")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: WriteFile: %v", err)
	}
	path := filepath.Join(blockingFile, "sub", "records.yaml")

	colDef := &ingitdb.CollectionDef{
		ID: "test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	data := map[string]map[string]any{"id1": {"field": "value"}}
	err := writeMapOfRecordsFile(path, colDef, data)
	if err == nil {
		t.Fatal("writeMapOfRecordsFile: expected mkdir error, got nil")
	}
	const prefix = "failed to create directory"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// writeMapOfRecordsFile – encode error (unsupported format)
// ---------------------------------------------------------------------------

func TestWriteMapOfRecordsFile_EncodeError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xml")
	colDef := &ingitdb.CollectionDef{
		ID: "test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "test.xml",
			Format:     "xml",
			RecordType: ingitdb.MapOfRecords,
		},
	}
	data := map[string]map[string]any{"id1": {"field": "value"}}
	err := writeMapOfRecordsFile(path, colDef, data)
	if err == nil {
		t.Fatal("writeMapOfRecordsFile: expected encode error, got nil")
	}
	const prefix = "failed to encode records"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

// ---------------------------------------------------------------------------
// executeQueryToRecordsReader – collection has nil RecordFile
// ---------------------------------------------------------------------------

func TestExecuteQueryToRecordsReader_NilRecordFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:         "test.items",
				DirPath:    dir,
				RecordFile: nil,
			},
		},
	}
	db := openTestDB(t, dir, def)
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
		t.Fatal("expected error for nil RecordFile in query, got nil")
	}
}

// ---------------------------------------------------------------------------
// readAllMapOfRecords – readMapOfRecordsFile returns error
// ---------------------------------------------------------------------------

func TestReadAllMapOfRecords_ReadError(t *testing.T) {
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
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.tags": colDef},
	}
	// Write an invalid YAML file to trigger parse error.
	if err := os.WriteFile(filepath.Join(dir, "tags.yaml"), []byte("id1: [unclosed"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.tags", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.tags", ""), map[string]any{})
	})
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		_, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		return err
	})
	if err == nil {
		t.Fatal("expected error from readMapOfRecordsFile, got nil")
	}
}

// ---------------------------------------------------------------------------
// readAllSingleRecords – glob error from malformed pattern
// ---------------------------------------------------------------------------

func TestReadAllSingleRecords_GlobError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "[{key}.yaml" → glob pattern "[*.yaml" which is a malformed bracket expression.
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "[{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.items": colDef},
	}
	db := openTestDB(t, dir, def)
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
		t.Fatal("expected error from invalid glob pattern, got nil")
	}
}

// ---------------------------------------------------------------------------
// readAllSingleRecords – read error from record file
// ---------------------------------------------------------------------------

func TestReadAllSingleRecords_RecordReadError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Write an invalid YAML record to trigger read error.
	if err := os.WriteFile(filepath.Join(recordsDir, "bad.yaml"), []byte("key: [unclosed"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	db := openTestDB(t, dir, def)
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
		t.Fatal("expected error from invalid record file, got nil")
	}
}

// ---------------------------------------------------------------------------
// executeQueryToRecordsReader – WHERE evaluateCondition error
// ---------------------------------------------------------------------------

func TestExecuteQueryToRecordsReader_WhereConditionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(1)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// Use an unsupported operator to trigger evaluateComparison error.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.WhereField("name", "INVALID_OP", "A")
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		_, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		return err
	})
	if err == nil {
		t.Fatal("expected error from invalid operator in WHERE, got nil")
	}
}

// ---------------------------------------------------------------------------
// OrderBy edge cases: equal values (cmp==0 → continue), return false, non-FieldRef
// ---------------------------------------------------------------------------

// TestExecuteQueryToRecordsReader_OrderByEqualValues exercises the
// "cmp == 0 → continue" path and the "return false" fallback when
// all fields are equal.
func TestExecuteQueryToRecordsReader_OrderByEqualValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Two records with the same score — exercises cmp==0 path.
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(80)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "B", "score": float64(80)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// Order by score (ties), then by name (breaks tie).
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.AscendingField("score"), dal.AscendingField("name"))
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
	if len(keys) != 2 {
		t.Fatalf("expected 2 records, got %d", len(keys))
	}
	// A < B alphabetically, so A should come first.
	if keys[0] != "a" || keys[1] != "b" {
		t.Errorf("order: got %v, want [a b]", keys)
	}
}

// TestExecuteQueryToRecordsReader_OrderByAllEqual exercises the
// "return false" at the end of the orderBy comparator when all
// sort fields are equal.
func TestExecuteQueryToRecordsReader_OrderByAllEqual(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// All scores identical.
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "Same", "score": float64(50)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "Same", "score": float64(50)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.AscendingField("score"), dal.AscendingField("name"))
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
	if count != 2 {
		t.Errorf("expected 2 records, got %d", count)
	}
}

// TestExecuteQueryToRecordsReader_OrderByNonFieldRef exercises the
// "!isRef" → continue branch in the orderBy comparator by ordering
// with a Constant expression (which is not a FieldRef).
func TestExecuteQueryToRecordsReader_OrderByNonFieldRef(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(1)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "B", "score": float64(2)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// Order by a Constant (non-FieldRef) expression — will be skipped.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.Ascending(dal.NewConstant(1)))
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
	if count != 2 {
		t.Errorf("expected 2 records, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// nonStructuredQuery – exercises the "only StructuredQuery supported" branch
// ---------------------------------------------------------------------------

// nonStructuredQuery implements dal.Query but not dal.StructuredQuery.
type nonStructuredQuery struct{}

func (nonStructuredQuery) String() string { return "" }
func (nonStructuredQuery) Offset() int    { return 0 }
func (nonStructuredQuery) Limit() int     { return 0 }
func (nonStructuredQuery) GetRecordsReader(context.Context, dal.QueryExecutor) (dal.RecordsReader, error) {
	return nil, nil
}
func (nonStructuredQuery) GetRecordsetReader(context.Context, dal.QueryExecutor) (dal.RecordsetReader, error) {
	return nil, nil
}

func TestExecuteQueryToRecordsReader_NonStructuredQuery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		rtx := tx.(readonlyTx)
		_, err := executeQueryToRecordsReader(ctx, rtx, nonStructuredQuery{})
		return err
	})
	if err == nil {
		t.Fatal("expected error for non-StructuredQuery, got nil")
	}
}

// ---------------------------------------------------------------------------
// nilFromQuery – exercises the "query has no FROM clause" branch
// ---------------------------------------------------------------------------

type nilFromQuery struct {
	nonStructuredQuery
}

func (nilFromQuery) From() dal.FromSource           { return nil }
func (nilFromQuery) Where() dal.Condition           { return nil }
func (nilFromQuery) GroupBy() []dal.Expression      { return nil }
func (nilFromQuery) OrderBy() []dal.OrderExpression { return nil }
func (nilFromQuery) Columns() []dal.Column          { return nil }
func (nilFromQuery) IntoRecord() dal.Record         { return nil }
func (nilFromQuery) IDKind() reflect.Kind           { return reflect.String }
func (nilFromQuery) StartFrom() dal.Cursor          { return "" }

// Compile-time check: nilFromQuery must satisfy dal.StructuredQuery.
var _ dal.StructuredQuery = nilFromQuery{}

func TestExecuteQueryToRecordsReader_NilFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		rtx := tx.(readonlyTx)
		_, err := executeQueryToRecordsReader(ctx, rtx, nilFromQuery{})
		return err
	})
	if err == nil {
		t.Fatal("expected error for nil FROM clause, got nil")
	}
	const want = "query has no FROM clause"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// WHERE filter using $id – exercises the $id branch in resolveExpression
// ---------------------------------------------------------------------------

func TestExecuteQueryToRecordsReader_WhereByID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"name": "A", "score": float64(1)})
	writeQueryFixture(t, filepath.Join(recordsDir, "b.yaml"), map[string]any{"name": "B", "score": float64(2)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// WHERE $id = "a" — exercises the $id FieldRef branch in resolveExpression.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.WhereField("$id", dal.Equal, "a")
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
		t.Errorf("WHERE $id='a': got %v, want [a]", keys)
	}
}

// ---------------------------------------------------------------------------
// readAllSingleRecords – !found branch via dangling symlink
// ---------------------------------------------------------------------------

func TestReadAllSingleRecords_DanglingSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Create a real record and a dangling symlink.
	writeQueryFixture(t, filepath.Join(recordsDir, "real.yaml"), map[string]any{"name": "Real", "score": float64(1)})
	symlinkPath := filepath.Join(recordsDir, "ghost.yaml")
	if err := os.Symlink(filepath.Join(recordsDir, "nonexistent.yaml"), symlinkPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
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
	// Only the real record should be returned; dangling symlink is skipped.
	if count != 1 {
		t.Errorf("expected 1 record (dangling symlink skipped), got %d", count)
	}
}

// ---------------------------------------------------------------------------
// FROM source that is not a CollectionRef — exercises the "FROM source must
// be a CollectionRef" branch.  We embed dal.CollectionRef inside a wrapper
// struct so the wrapper satisfies dal.RecordsetSource (including its
// unexported method), but the concrete type is *not* dal.CollectionRef and
// the type assertion in executeQueryToRecordsReader fails.
// ---------------------------------------------------------------------------

type wrappedRecordsetSource struct {
	dal.CollectionRef
}

type wrappedFromSource struct{}

func (wrappedFromSource) Base() dal.RecordsetSource            { return wrappedRecordsetSource{} }
func (wrappedFromSource) Join(dal.JoinedSource) dal.FromSource { return wrappedFromSource{} }
func (wrappedFromSource) Joins() []dal.JoinedSource            { return nil }
func (wrappedFromSource) NewQuery() *dal.QueryBuilder          { return nil }

type nonCollectionRefFromQuery struct {
	nonStructuredQuery
}

func (nonCollectionRefFromQuery) From() dal.FromSource           { return wrappedFromSource{} }
func (nonCollectionRefFromQuery) Where() dal.Condition           { return nil }
func (nonCollectionRefFromQuery) GroupBy() []dal.Expression      { return nil }
func (nonCollectionRefFromQuery) OrderBy() []dal.OrderExpression { return nil }
func (nonCollectionRefFromQuery) Columns() []dal.Column          { return nil }
func (nonCollectionRefFromQuery) IntoRecord() dal.Record         { return nil }
func (nonCollectionRefFromQuery) IDKind() reflect.Kind           { return reflect.String }
func (nonCollectionRefFromQuery) StartFrom() dal.Cursor          { return "" }

func TestExecuteQueryToRecordsReader_NonCollectionRefFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		rtx := tx.(readonlyTx)
		_, err := executeQueryToRecordsReader(ctx, rtx, nonCollectionRefFromQuery{})
		return err
	})
	if err == nil {
		t.Fatal("expected error for non-CollectionRef FROM, got nil")
	}
	const want = "FROM source must be a CollectionRef"
	if len(err.Error()) < len(want) || err.Error()[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", err.Error(), want)
	}
}
