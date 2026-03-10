package dalgo2fsingitdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeQueryTestDef builds a SingleRecord YAML definition with name and score columns.
func makeQueryTestDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name":  {Type: ingitdb.ColumnTypeString},
			"score": {Type: ingitdb.ColumnTypeFloat},
		},
	}
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": colDef,
		},
	}
}

// writeQueryFixture writes a YAML fixture for query tests.
func writeQueryFixture(t *testing.T, path string, data map[string]any) {
	t.Helper()
	writeYAMLFixture(t, path, data)
}

func TestExecuteQueryToRecordsReader_AllRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha", "score": 90})
	writeQueryFixture(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta", "score": 70})
	writeQueryFixture(t, filepath.Join(recordsDir, "gamma.yaml"), map[string]any{"name": "Gamma", "score": 80})

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
	if count != 3 {
		t.Errorf("expected 3 records, got %d", count)
	}
}

func TestExecuteQueryToRecordsReader_WhereFilter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha", "score": float64(90)})
	writeQueryFixture(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta", "score": float64(70)})
	writeQueryFixture(t, filepath.Join(recordsDir, "gamma.yaml"), map[string]any{"name": "Gamma", "score": float64(80)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.WhereField("score", dal.GreaterThen, float64(75))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})

	var results []string
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
			results = append(results, rec.Key().ID.(string))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 records with score>75, got %d: %v", len(results), results)
	}
}

func TestExecuteQueryToRecordsReader_OrderByAscending(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta", "score": float64(70)})
	writeQueryFixture(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha", "score": float64(90)})
	writeQueryFixture(t, filepath.Join(recordsDir, "gamma.yaml"), map[string]any{"name": "Gamma", "score": float64(80)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.AscendingField("score"))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("test.items", ""), map[string]any{})
	})

	var scores []any
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
			data := rec.Data().(map[string]any)
			scores = append(scores, data["score"])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(scores) != 3 {
		t.Fatalf("expected 3 records, got %d", len(scores))
	}
	// Verify ascending order by converting to float64 for comparison.
	toF := func(v any) float64 {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		default:
			return 0
		}
	}
	if toF(scores[0]) > toF(scores[1]) || toF(scores[1]) > toF(scores[2]) {
		t.Errorf("expected ascending order, got %v", scores)
	}
}

func TestExecuteQueryToRecordsReader_OrderByDescending(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeQueryFixture(t, filepath.Join(recordsDir, "alpha.yaml"), map[string]any{"name": "Alpha", "score": float64(90)})
	writeQueryFixture(t, filepath.Join(recordsDir, "beta.yaml"), map[string]any{"name": "Beta", "score": float64(70)})

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.OrderBy(dal.DescendingField("score"))
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
	if len(keys) < 2 {
		t.Fatalf("expected >=2 records, got %d", len(keys))
	}
	if keys[0] != "alpha" {
		t.Errorf("expected 'alpha' (score 90) first, got %q", keys[0])
	}
}

func TestExecuteQueryToRecordsReader_Limit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		writeQueryFixture(t, filepath.Join(recordsDir, key+".yaml"), map[string]any{"name": key})
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("test.items", "")))
	qb.Limit(3)
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
	if count != 3 {
		t.Errorf("expected 3 records (limit), got %d", count)
	}
}

func TestExecuteQueryToRecordsReader_UnknownCollection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeQueryTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef("no.such.collection", "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("no.such.collection", ""), map[string]any{})
	})

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		_, err := tx.ExecuteQueryToRecordsReader(ctx, q)
		return err
	})
	if err == nil {
		t.Fatal("expected error for unknown collection, got nil")
	}
}
