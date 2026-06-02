package dalgo2ingitdb_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// setupFormulaDB creates a temp project with a "people" SingleRecord/YAML
// collection that declares two computed columns: a string full_name and an
// int safe_ratio (qty / divisor). It returns the dal.DB and project root.
func setupFormulaDB(t *testing.T) (dal.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defDir := filepath.Join(root, "people", ".collection")
	if err := os.MkdirAll(defDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", defDir, err)
	}
	def := `id: people
record_file:
  name: "{key}.yaml"
  format: yaml
  type: map[string]any
columns:
  first_name:
    type: string
  last_name:
    type: string
  qty:
    type: int
  divisor:
    type: int
  full_name:
    type: string
    formula: 'first_name + " " + last_name'
  safe_ratio:
    type: int
    formula: 'qty / divisor'
`
	defPath := filepath.Join(defDir, "definition.yaml")
	if err := os.WriteFile(defPath, []byte(def), 0o644); err != nil {
		t.Fatalf("write %s: %v", defPath, err)
	}
	dir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	regPath := filepath.Join(dir, "root-collections.yaml")
	if err := os.WriteFile(regPath, []byte("people: people\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", regPath, err)
	}
	return db, root
}

func writePersonRecord(t *testing.T, root, key, content string) {
	t.Helper()
	dir := filepath.Join(root, "people", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, key+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestDB_Get_ComputesFormulaColumns verifies the read path computes and
// coerces formula columns: full_name (string) and safe_ratio (int).
func TestDB_Get_ComputesFormulaColumns(t *testing.T) {
	t.Parallel()
	db, root := setupFormulaDB(t)
	writePersonRecord(t, root, "ada", "first_name: Ada\nlast_name: Lovelace\nqty: 12\ndivisor: 4\n")

	rec := dal.NewRecordWithData(dal.NewKeyWithID("people", "ada"), map[string]any{})
	if err := db.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: %v", err)
	}
	data := rec.Data().(map[string]any)
	if data["full_name"] != "Ada Lovelace" {
		t.Errorf("full_name: got %v, want 'Ada Lovelace'", data["full_name"])
	}
	if data["safe_ratio"] != int64(3) {
		t.Errorf("safe_ratio: got %v (%T), want int64(3)", data["safe_ratio"], data["safe_ratio"])
	}
}

// TestDB_Get_FormulaRuntimeErrorFailsRead verifies a runtime formula error
// (division by zero) aborts the read with an error naming collection, key,
// and column, and no partial row is exposed.
func TestDB_Get_FormulaRuntimeErrorFailsRead(t *testing.T) {
	t.Parallel()
	db, root := setupFormulaDB(t)
	writePersonRecord(t, root, "bad", "first_name: Bob\nlast_name: Zero\nqty: 5\ndivisor: 0\n")

	rec := dal.NewRecordWithData(dal.NewKeyWithID("people", "bad"), map[string]any{})
	err := db.Get(context.Background(), rec)
	if err == nil {
		t.Fatal("expected read to fail on division by zero")
	}
	for _, want := range []string{"people", "bad", "safe_ratio"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// TestDB_Query_ComputesFormulaColumns exercises the query read path so the
// formula wiring in executeQueryToRecordsReader is covered end-to-end.
func TestDB_Query_ComputesFormulaColumns(t *testing.T) {
	t.Parallel()
	db, root := setupFormulaDB(t)
	writePersonRecord(t, root, "ada", "first_name: Ada\nlast_name: Lovelace\nqty: 12\ndivisor: 4\n")

	query := dal.From(dal.NewRootCollectionRef("people", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("people", ""), map[string]any{})
		})
	reader, err := db.ExecuteQueryToRecordsReader(context.Background(), query)
	if err != nil {
		t.Fatalf("ExecuteQueryToRecordsReader: %v", err)
	}
	rec, err := reader.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	data := rec.Data().(map[string]any)
	if data["full_name"] != "Ada Lovelace" {
		t.Errorf("full_name: got %v, want 'Ada Lovelace'", data["full_name"])
	}
}

// TestDB_Query_FormulaRuntimeErrorFailsRead covers the query-path error
// branch when a formula fails at runtime.
func TestDB_Query_FormulaRuntimeErrorFailsRead(t *testing.T) {
	t.Parallel()
	db, root := setupFormulaDB(t)
	writePersonRecord(t, root, "bad", "first_name: Bob\nlast_name: Zero\nqty: 5\ndivisor: 0\n")

	query := dal.From(dal.NewRootCollectionRef("people", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("people", ""), map[string]any{})
		})
	_, err := db.ExecuteQueryToRecordsReader(context.Background(), query)
	if err == nil {
		t.Fatal("expected query to fail on division by zero")
	}
	if !strings.Contains(err.Error(), "safe_ratio") {
		t.Errorf("error %q missing 'safe_ratio'", err.Error())
	}
}
