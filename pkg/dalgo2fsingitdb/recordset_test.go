package dalgo2fsingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// peopleComputedDef builds a SingleRecord YAML definition with a computed
// full_name column derived from stored first_name/last_name.
func peopleComputedDef(dirPath string) *ingitdb.Definition {
	colDef := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: dirPath,
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
	return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"people": colDef}}
}

func peopleQuery() dal.Query {
	return dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("people", ""))).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("people", ""), map[string]any{})
		})
}

func TestExecuteQueryToRecordsetReader_SingleRecords_Computed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := peopleComputedDef(dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "ada.yaml"),
		map[string]any{"first_name": "Ada", "last_name": "Lovelace"})

	db := openTestDB(t, dir, def)
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, qerr := tx.ExecuteQueryToRecordsetReader(ctx, peopleQuery())
		if qerr != nil {
			return qerr
		}
		defer func() { _ = reader.Close() }()
		row, rs, nerr := reader.Next()
		if nerr != nil {
			return nerr
		}
		first, ferr := row.GetValueByName("first_name", rs)
		if ferr != nil {
			return ferr
		}
		if first != "Ada" {
			t.Errorf("first_name = %v, want Ada", first)
		}
		full, cerr := row.GetValueByName("full_name", rs)
		if cerr != nil {
			return cerr
		}
		if full != "Ada Lovelace" {
			t.Errorf("full_name = %v, want %q", full, "Ada Lovelace")
		}
		if _, _, eerr := reader.Next(); !errors.Is(eerr, dal.ErrNoMoreRecords) {
			t.Errorf("second Next err = %v, want ErrNoMoreRecords", eerr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestExecuteQueryToRecordsetReader_MapOfRecords_Computed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: dir,
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
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"scores": colDef}}
	// A fixed map-of-records file (Name has no {key}) lives at the collection
	// root, not under $records.
	if err := os.WriteFile(filepath.Join(dir, "scores.yaml"),
		[]byte("alpha:\n  score: 5\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	db := openTestDB(t, dir, def)
	query := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("scores", ""))).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("scores", ""), map[string]any{})
		})
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, qerr := tx.ExecuteQueryToRecordsetReader(ctx, query)
		if qerr != nil {
			return qerr
		}
		defer func() { _ = reader.Close() }()
		row, rs, nerr := reader.Next()
		if nerr != nil {
			return nerr
		}
		doubled, derr := row.GetValueByName("doubled", rs)
		if derr != nil {
			return derr
		}
		if doubled != int64(10) {
			t.Errorf("doubled = %v, want 10", doubled)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestExecuteQueryToRecordsetReader_UnknownCollection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := peopleComputedDef(dir)
	db := openTestDB(t, dir, def)
	query := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("nope", ""))).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("nope", ""), map[string]any{})
		})
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		_, qerr := tx.ExecuteQueryToRecordsetReader(ctx, query)
		return qerr
	})
	if err == nil {
		t.Fatal("want error for unknown collection")
	}
}

func TestExecuteQueryToRecordsetReader_UnsupportedRecordType(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: "bogus",
		},
		Columns: map[string]*ingitdb.ColumnDef{"first_name": {Type: ingitdb.ColumnTypeString}},
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"people": colDef}}
	db := openTestDB(t, dir, def)
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		_, qerr := tx.ExecuteQueryToRecordsetReader(ctx, peopleQuery())
		return qerr
	})
	if err == nil {
		t.Fatal("want error for unsupported record type")
	}
}

// TestExecuteQueryToRecordsReader_FormulaError covers the eager-bake error
// branch (bakeStoredRecords): a computed column whose formula fails at runtime
// aborts the records-reader read.
func TestExecuteQueryToRecordsReader_FormulaError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"qty":   {Type: ingitdb.ColumnTypeInt},
			"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty / 0"},
		},
		ColumnsOrder: []string{"qty", "ratio"},
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"people": colDef}}
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"qty": 3})

	db := openTestDB(t, dir, def)
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		_, qerr := tx.ExecuteQueryToRecordsReader(ctx, peopleQuery())
		return qerr
	})
	if err == nil {
		t.Fatal("want formula runtime error")
	}
	if !strings.Contains(err.Error(), "ratio") {
		t.Errorf("error should name the ratio column, got: %v", err)
	}
}
