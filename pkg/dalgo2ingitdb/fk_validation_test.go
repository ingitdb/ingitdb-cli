package dalgo2ingitdb_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

func setupForeignKeyDB(t *testing.T, foreignKeyTarget string) (dal.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)
	parent := dbschema.CollectionDef{
		Name: "parents",
		Fields: []dbschema.FieldDef{
			{Name: "name", Type: dbschema.String, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), parent); err != nil {
		t.Fatalf("CreateCollection parents: %v", err)
	}
	child := dbschema.CollectionDef{
		Name: "children",
		Fields: []dbschema.FieldDef{
			{Name: "name", Type: dbschema.String, Nullable: false},
			{Name: "parent_id", Type: dbschema.String, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), child); err != nil {
		t.Fatalf("CreateCollection children: %v", err)
	}
	writeChildDefinitionWithForeignKey(t, root, foreignKeyTarget)
	return db, root
}

func writeChildDefinitionWithForeignKey(t *testing.T, root, foreignKeyTarget string) {
	t.Helper()
	definition := `record_file:
    name: "{key}.yaml"
    format: yaml
    type: map[string]any
columns:
    name:
        type: string
        required: true
    parent_id:
        type: string
        required: true
        foreign_key: ` + foreignKeyTarget + `
columns_order:
    - name
    - parent_id
`
	schemaDir := filepath.Join(root, "children", ".collection")
	definitionPath := filepath.Join(schemaDir, "definition.yaml")
	if err := os.WriteFile(definitionPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("write child definition: %v", err)
	}
}

func insertForeignKeyRecord(t *testing.T, db dal.DB, collection, key string, data map[string]any) error {
	t.Helper()
	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		record := dal.NewRecordWithData(recordKey, data)
		return tx.Insert(ctx, record)
	})
	return err
}

func childRecordPath(root, key string) string {
	recordsDir := filepath.Join(root, "children", "$records")
	return filepath.Join(recordsDir, key+".yaml")
}

func requireNoRecordFile(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("child record file: stat err = %v, want not exist", err)
	}
}

func requireErrorContainsAll(t *testing.T, err error, substrings ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("error: got nil, want non-nil")
	}
	message := err.Error()
	for _, substring := range substrings {
		if !strings.Contains(message, substring) {
			t.Fatalf("error %q does not contain %q", message, substring)
		}
	}
}

func TestReadwriteTx_InsertValidForeignKeyPersistsChild(t *testing.T) {
	t.Parallel()
	db, root := setupForeignKeyDB(t, "parents")
	parentData := map[string]any{"name": "Parent"}
	if err := insertForeignKeyRecord(t, db, "parents", "parent-1", parentData); err != nil {
		t.Fatalf("insert parent: %v", err)
	}
	childData := map[string]any{"name": "Child", "parent_id": "parent-1"}
	if err := insertForeignKeyRecord(t, db, "children", "child-1", childData); err != nil {
		t.Fatalf("insert child: %v", err)
	}
	childPath := childRecordPath(root, "child-1")
	if _, err := os.Stat(childPath); err != nil {
		t.Fatalf("child record file: stat: %v", err)
	}
}

func TestReadwriteTx_InsertMissingForeignKeyParentFailsWithoutWritingChild(t *testing.T) {
	t.Parallel()
	db, root := setupForeignKeyDB(t, "parents")
	childData := map[string]any{"name": "Child", "parent_id": "missing-parent"}
	err := insertForeignKeyRecord(t, db, "children", "child-1", childData)
	requireErrorContainsAll(t, err, "Insert", "children", "parent_id", "parents", "missing-parent")
	childPath := childRecordPath(root, "child-1")
	requireNoRecordFile(t, childPath)
}

func TestReadwriteTx_InsertInvalidForeignKeyTargetCollectionFailsWithoutWritingChild(t *testing.T) {
	t.Parallel()
	db, root := setupForeignKeyDB(t, "missing_collection")
	childData := map[string]any{"name": "Child", "parent_id": "parent-1"}
	err := insertForeignKeyRecord(t, db, "children", "child-1", childData)
	requireErrorContainsAll(t, err, "configuration error", "missing_collection")
	childPath := childRecordPath(root, "child-1")
	requireNoRecordFile(t, childPath)
}
