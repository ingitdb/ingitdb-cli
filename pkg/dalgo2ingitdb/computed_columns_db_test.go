package dalgo2ingitdb_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// setupComputedColumnDB creates a "people" collection whose "full_name" column
// is computed from "first_name" and "last_name".
func setupComputedColumnDB(t *testing.T) (dal.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)
	people := dbschema.CollectionDef{
		Name: "people",
		Fields: []dbschema.FieldDef{
			{Name: "first_name", Type: dbschema.String, Nullable: false},
			{Name: "last_name", Type: dbschema.String, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), people); err != nil {
		t.Fatalf("CreateCollection people: %v", err)
	}
	definition := `record_file:
    name: "{key}.yaml"
    format: yaml
    type: map[string]any
columns:
    first_name:
        type: string
        required: true
    last_name:
        type: string
        required: true
    full_name:
        type: string
        formula: 'first_name + " " + last_name'
columns_order:
    - first_name
    - last_name
    - full_name
`
	schemaDir := filepath.Join(root, "people", ".collection")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	definitionPath := filepath.Join(schemaDir, "definition.yaml")
	if err := os.WriteFile(definitionPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("write people definition: %v", err)
	}
	return db, root
}

func insertRecord(t *testing.T, db dal.DB, collection, key string, data map[string]any) error {
	t.Helper()
	ctx := context.Background()
	return db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		record := dal.NewRecordWithData(recordKey, data)
		return tx.Insert(ctx, record)
	})
}

func setRecord(t *testing.T, db dal.DB, collection, key string, data map[string]any) error {
	t.Helper()
	ctx := context.Background()
	return db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		record := dal.NewRecordWithData(recordKey, data)
		return tx.Set(ctx, record)
	})
}

func peopleRecordPath(root, key string) string {
	return filepath.Join(root, "people", "$records", key+".yaml")
}

func TestReadwriteTx_InsertComputedColumnValueFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedColumnDB(t)
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace", "full_name": "Ada Lovelace"}
	err := insertRecord(t, db, "people", "ada", data)
	requireErrorContainsAll(t, err, "people", "ada", "full_name")
	requireNoRecordFile(t, peopleRecordPath(root, "ada"))
}

func TestReadwriteTx_SetComputedColumnValueFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedColumnDB(t)
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace", "full_name": "Ada Lovelace"}
	err := setRecord(t, db, "people", "ada", data)
	requireErrorContainsAll(t, err, "people", "ada", "full_name")
	requireNoRecordFile(t, peopleRecordPath(root, "ada"))
}

func TestReadwriteTx_InsertWithoutComputedColumnSucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupComputedColumnDB(t)
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace"}
	if err := insertRecord(t, db, "people", "ada", data); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := os.Stat(peopleRecordPath(root, "ada")); err != nil {
		t.Fatalf("record file: stat: %v", err)
	}
}

func TestReadwriteTx_SetWithoutComputedColumnSucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupComputedColumnDB(t)
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace"}
	if err := setRecord(t, db, "people", "ada", data); err != nil {
		t.Fatalf("set: %v", err)
	}
	if _, err := os.Stat(peopleRecordPath(root, "ada")); err != nil {
		t.Fatalf("record file: stat: %v", err)
	}
}
