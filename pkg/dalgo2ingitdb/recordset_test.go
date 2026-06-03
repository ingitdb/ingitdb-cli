package dalgo2ingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// countingEvaluator is a recordset.Evaluator that records how many times it is
// invoked, so tests can prove lazy, once-per-row resolution.
type countingEvaluator struct {
	calls *int
}

func (e countingEvaluator) Eval(_ map[string]any) (any, error) {
	*e.calls++
	return int64(99), nil
}

// lazyTestColDef returns a collection with one stored column (qty) and one
// computed column (ratio).
func lazyTestColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"qty":   {Type: ingitdb.ColumnTypeInt},
			"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty / 0"},
		},
		ColumnsOrder: []string{"qty", "ratio"},
	}
}

// AC: stored-only-projection-evaluates-nothing — reading only the stored column
// must never invoke the computed column's evaluator (no eager bake).
func TestBuildRecordset_StoredOnlyProjectionEvaluatesNothing(t *testing.T) {
	t.Parallel()
	calls := 0
	records := []KeyedStored{{Key: "a", Stored: map[string]any{"qty": int64(3)}}}
	rs := buildRecordset(lazyTestColDef(), records, func(string) recordset.Evaluator {
		return countingEvaluator{calls: &calls}
	})
	row := rs.GetRow(0)
	got, err := row.GetValueByName("qty", rs)
	if err != nil {
		t.Fatalf("GetValueByName(qty): %v", err)
	}
	if got != int64(3) {
		t.Errorf("qty = %v, want 3", got)
	}
	if calls != 0 {
		t.Errorf("evaluator invoked %d times, want 0 — computed column must not be evaluated", calls)
	}
}

// AC: evaluator-invoked-once-per-row — reading the same row's computed value via
// more than one access path must invoke the evaluator exactly once (per-row
// memoization is relied upon, not defeated).
func TestBuildRecordset_EvaluatorInvokedOncePerRow(t *testing.T) {
	t.Parallel()
	calls := 0
	records := []KeyedStored{{Key: "a", Stored: map[string]any{"qty": int64(3)}}}
	rs := buildRecordset(lazyTestColDef(), records, func(string) recordset.Evaluator {
		return countingEvaluator{calls: &calls}
	})
	row := rs.GetRow(0)
	// Simulate a --where predicate read and an output-projection read of the
	// same computed column on the same row instance.
	if _, err := row.GetValueByName("ratio", rs); err != nil {
		t.Fatalf("GetValueByName(ratio) #1: %v", err)
	}
	if _, err := row.GetValueByName("ratio", rs); err != nil {
		t.Fatalf("GetValueByName(ratio) #2: %v", err)
	}
	if _, err := row.Data(rs); err != nil {
		t.Fatalf("Data: %v", err)
	}
	if calls != 1 {
		t.Errorf("evaluator invoked %d times, want 1 (per-row memoization)", calls)
	}
}

func TestRowKey(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "people",
		Columns:      map[string]*ingitdb.ColumnDef{"first_name": {Type: ingitdb.ColumnTypeString}},
		ColumnsOrder: []string{"first_name"},
	}
	rs := BuildRecordset(colDef, []KeyedStored{{Key: "ada", Stored: map[string]any{"first_name": "Ada"}}})
	if got := RowKey(rs.GetRow(0), rs); got != "ada" {
		t.Errorf("RowKey = %q, want ada", got)
	}
}

func TestFormulaEvaluator_Eval(t *testing.T) {
	t.Parallel()
	e := formulaEvaluator{formula: "a + b"}
	got, err := e.Eval(map[string]any{"a": int64(2), "b": int64(3)})
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != int64(5) {
		t.Errorf("a + b = %v, want 5", got)
	}
}

func TestFormulaEvaluator_StripsIDColumn(t *testing.T) {
	t.Parallel()
	// The reserved "$id" entry must be stripped so it is not bound as a Starlark
	// field; the formula sees exactly the stored siblings.
	e := formulaEvaluator{formula: `first_name + " " + last_name`}
	got, err := e.Eval(map[string]any{
		IDColumn:     "ada",
		"first_name": "Ada",
		"last_name":  "Lovelace",
	})
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != "Ada Lovelace" {
		t.Errorf("full_name = %v, want %q", got, "Ada Lovelace")
	}
}

func TestNewRecordsetReader(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "people",
		Columns:      map[string]*ingitdb.ColumnDef{"first_name": {Type: ingitdb.ColumnTypeString}},
		ColumnsOrder: []string{"first_name"},
	}
	records := []KeyedStored{
		{Key: "ada", Stored: map[string]any{"first_name": "Ada"}},
		{Key: "alan", Stored: map[string]any{"first_name": "Alan"}},
	}
	rs := BuildRecordset(colDef, records)
	reader := NewRecordsetReader(rs)

	if reader.Recordset() != rs {
		t.Error("Recordset() did not return the underlying recordset")
	}
	if c, err := reader.Cursor(); err != nil || c != "" {
		t.Errorf("Cursor() = (%q, %v), want (\"\", nil)", c, err)
	}

	var keys []string
	for {
		row, gotRS, err := reader.Next()
		if errors.Is(err, dal.ErrNoMoreRecords) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if gotRS != rs {
			t.Error("Next() returned a different recordset")
		}
		id, err := row.GetValueByName(IDColumn, rs)
		if err != nil {
			t.Fatalf("GetValueByName($id): %v", err)
		}
		keys = append(keys, id.(string))
	}
	if len(keys) != 2 || keys[0] != "ada" || keys[1] != "alan" {
		t.Errorf("keys = %v, want [ada alan]", keys)
	}
	if err := reader.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// recordsetQueryTx builds a disk-backed readonlyTx with a "people" SingleRecord
// collection declaring a computed full_name, plus a structured query for it.
func recordsetQueryTx(t *testing.T) (readonlyTx, dal.Query) {
	t.Helper()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: filepath.Join(root, "people"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"first_name": {Type: ingitdb.ColumnTypeString},
			"last_name":  {Type: ingitdb.ColumnTypeString},
			"full_name":  {Type: ingitdb.ColumnTypeString, Formula: `first_name + " " + last_name`},
		},
		ColumnsOrder: []string{"first_name", "last_name", "full_name"},
	}
	recDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", recDir, err)
	}
	if err := os.WriteFile(filepath.Join(recDir, "ada.yaml"),
		[]byte("first_name: Ada\nlast_name: Lovelace\n"), 0o644); err != nil {
		t.Fatalf("write record: %v", err)
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"people": colDef}}
	query := dal.From(dal.NewRootCollectionRef("people", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("people", ""), map[string]any{})
		})
	return readonlyTx{def: def}, query
}

func TestReadonlyTx_ExecuteQueryToRecordsetReader_Success(t *testing.T) {
	t.Parallel()
	tx, query := recordsetQueryTx(t)
	reader, err := tx.ExecuteQueryToRecordsetReader(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteQueryToRecordsetReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	row, rs, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	first, err := row.GetValueByName("first_name", rs)
	if err != nil {
		t.Fatalf("GetValueByName(first_name): %v", err)
	}
	if first != "Ada" {
		t.Errorf("first_name = %v, want Ada", first)
	}
	full, err := row.GetValueByName("full_name", rs)
	if err != nil {
		t.Fatalf("GetValueByName(full_name): %v", err)
	}
	if full != "Ada Lovelace" {
		t.Errorf("full_name = %v, want %q", full, "Ada Lovelace")
	}
	if _, _, err := reader.Next(); !errors.Is(err, dal.ErrNoMoreRecords) {
		t.Errorf("second Next err = %v, want ErrNoMoreRecords", err)
	}
}

func TestAnyColumn(t *testing.T) {
	t.Parallel()
	c := &anyColumn{name: "x"}
	if c.Name() != "x" {
		t.Errorf("Name = %q, want x", c.Name())
	}
	if c.DefaultValue() != nil {
		t.Error("DefaultValue should be nil")
	}
	if c.DbType() != "" {
		t.Errorf("DbType = %q, want empty", c.DbType())
	}
	if c.IsBitmap() {
		t.Error("IsBitmap should be false")
	}
	if c.ValueType() == nil {
		t.Error("ValueType should not be nil")
	}
	// Out-of-range access before any Add.
	if _, err := c.GetValue(0); err == nil {
		t.Error("GetValue out of range should error")
	}
	if err := c.SetValue(0, "v"); err == nil {
		t.Error("SetValue out of range should error")
	}
	// Add (including a nil value) then read back.
	if err := c.Add(nil); err != nil {
		t.Fatalf("Add(nil): %v", err)
	}
	if err := c.Add("a"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := c.GetValue(0)
	if err != nil || got != nil {
		t.Errorf("GetValue(0) = (%v, %v), want (nil, nil)", got, err)
	}
	if err := c.SetValue(0, "z"); err != nil {
		t.Errorf("SetValue(0): %v", err)
	}
	vals := c.Values()
	if len(vals) != 2 || vals[0] != "z" || vals[1] != "a" {
		t.Errorf("Values = %v, want [z a]", vals)
	}
}

// TestBuildRecordset_PreservesUndeclaredStoredKeys covers the union branch: a
// stored key that is not a declared column is preserved as a stored column so
// reads stay byte-identical with the eager pipeline.
func TestBuildRecordset_PreservesUndeclaredStoredKeys(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "a", Stored: map[string]any{
		"qty":     int64(3),
		"unknown": "kept",
	}}}
	rs := BuildRecordset(lazyTestColDef(), records)
	if rs.GetColumnByName("unknown") == nil {
		t.Fatal("undeclared stored key must be preserved as a recordset column")
	}
	row := rs.GetRow(0)
	if v, err := row.GetValueByName("unknown", rs); err != nil || v != "kept" {
		t.Errorf("unknown = (%v, %v), want (kept, nil)", v, err)
	}
	if v, err := row.GetValueByName("qty", rs); err != nil || v != int64(3) {
		t.Errorf("qty = (%v, %v), want (3, nil)", v, err)
	}
}

// TestBuildRecordset_SkipsFormulaColumnStoredKeys covers the storedSet guard: a
// stored key that collides with a computed (formula) column is not set on the
// recordset (which would otherwise error), leaving the column to resolve lazily.
func TestBuildRecordset_SkipsFormulaColumnStoredKeys(t *testing.T) {
	t.Parallel()
	calls := 0
	records := []KeyedStored{{Key: "a", Stored: map[string]any{
		"qty":   int64(3),
		"ratio": int64(999), // collides with the computed "ratio" column
	}}}
	rs := buildRecordset(lazyTestColDef(), records, func(string) recordset.Evaluator {
		return countingEvaluator{calls: &calls}
	})
	row := rs.GetRow(0)
	got, err := row.GetValueByName("ratio", rs)
	if err != nil {
		t.Fatalf("GetValueByName(ratio): %v", err)
	}
	if got != int64(99) || calls != 1 {
		t.Errorf("ratio = %v (calls=%d), want computed 99 (calls=1), not the stored 999", got, calls)
	}
}

func TestReadonlyTx_ExecuteQueryToRecordsetReader_MapOfRecords(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: filepath.Join(root, "scores"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"score":   {Type: ingitdb.ColumnTypeInt},
			"doubled": {Type: ingitdb.ColumnTypeInt, Formula: "score * 2"},
		},
		ColumnsOrder: []string{"score", "doubled"},
	}
	recDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recDir, "scores.yaml"),
		[]byte("alpha:\n  score: 5\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"scores": colDef}}
	tx := readonlyTx{def: def}
	query := dal.From(dal.NewRootCollectionRef("scores", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("scores", ""), map[string]any{})
		})
	reader, err := tx.ExecuteQueryToRecordsetReader(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteQueryToRecordsetReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	row, rs, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	doubled, err := row.GetValueByName("doubled", rs)
	if err != nil {
		t.Fatalf("GetValueByName(doubled): %v", err)
	}
	if doubled != int64(10) {
		t.Errorf("doubled = %v, want 10", doubled)
	}
}

func TestReadonlyTx_ExecuteQueryToRecordsetReader_QueryErrors(t *testing.T) {
	t.Parallel()
	tx, query := recordsetQueryTx(t)
	// Read error: point the collection at a record type the query path rejects.
	tx.def.Collections["people"].RecordFile.RecordType = "bogus"
	if _, err := tx.ExecuteQueryToRecordsetReader(context.Background(), query); err == nil {
		t.Fatal("want error for unsupported record type")
	}
}
