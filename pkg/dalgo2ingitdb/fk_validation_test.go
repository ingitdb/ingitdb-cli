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
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

func setupForeignKeyDB(t *testing.T, foreignKeyTarget string) (dal.DB, string) {
	t.Helper()
	db, root := setupForeignKeyDBWithParentRequired(t, foreignKeyTarget, true)
	return db, root
}

func setupForeignKeyDBWithParentRequired(t *testing.T, foreignKeyTarget string, parentRequired bool) (dal.DB, string) {
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
	writeChildDefinitionWithForeignKeyRequired(t, root, foreignKeyTarget, parentRequired)
	return db, root
}

func writeChildDefinitionWithForeignKeyRequired(t *testing.T, root, foreignKeyTarget string, parentRequired bool) {
	t.Helper()
	required := "true"
	if !parentRequired {
		required = "false"
	}
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
        required: ` + required + `
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

func setForeignKeyRecord(t *testing.T, db dal.DB, collection, key string, data map[string]any) error {
	t.Helper()
	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		record := dal.NewRecordWithData(recordKey, data)
		return tx.Set(ctx, record)
	})
	return err
}

func updateForeignKeyRecord(t *testing.T, db dal.DB, collection, key string, updates []update.Update) error {
	t.Helper()
	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		return tx.Update(ctx, recordKey, updates)
	})
	return err
}

func deleteForeignKeyRecord(t *testing.T, db dal.DB, collection, key string) error {
	t.Helper()
	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		return tx.Delete(ctx, recordKey)
	})
	return err
}

func readForeignKeyRecordData(t *testing.T, db dal.DB, collection, key string) map[string]any {
	t.Helper()
	recordKey := dal.NewKeyWithID(collection, key)
	record := dal.NewRecordWithData(recordKey, map[string]any{})
	if err := db.Get(context.Background(), record); err != nil {
		t.Fatalf("Get %s/%s: %v", collection, key, err)
	}
	data, ok := record.Data().(map[string]any)
	if !ok {
		t.Fatalf("Get %s/%s data type = %T, want map[string]any", collection, key, record.Data())
	}
	return data
}

func collectionRecordPath(root, collection, key string) string {
	recordsDir := filepath.Join(root, collection, "$records")
	return filepath.Join(recordsDir, key+".yaml")
}

func childRecordPath(root, key string) string {
	return collectionRecordPath(root, "children", key)
}

func requireNoRecordFile(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("record file %s: stat err = %v, want not exist", path, err)
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

func TestReadwriteTx_InsertOptionalEmptyForeignKeyValuesPersistChild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "missing",
			data: map[string]any{"name": "Child"},
		},
		{
			name: "nil",
			data: map[string]any{"name": "Child", "parent_id": nil},
		},
		{
			name: "empty string",
			data: map[string]any{"name": "Child", "parent_id": ""},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			db, root := setupForeignKeyDBWithParentRequired(t, "parents", false)
			err := insertForeignKeyRecord(t, db, "children", "child-1", test.data)
			if err != nil {
				t.Fatalf("insert child: %v", err)
			}
			childPath := childRecordPath(root, "child-1")
			if _, err := os.Stat(childPath); err != nil {
				t.Fatalf("child record file: stat: %v", err)
			}
		})
	}
}

func TestReadwriteTx_InsertRequiredEmptyForeignKeyValuesFailWithoutWritingChild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "missing",
			data: map[string]any{"name": "Child"},
		},
		{
			name: "nil",
			data: map[string]any{"name": "Child", "parent_id": nil},
		},
		{
			name: "empty string",
			data: map[string]any{"name": "Child", "parent_id": ""},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			db, root := setupForeignKeyDB(t, "parents")
			err := insertForeignKeyRecord(t, db, "children", "child-1", test.data)
			requireErrorContainsAll(t, err, "Insert", "children", "parent_id", "required")
			childPath := childRecordPath(root, "child-1")
			requireNoRecordFile(t, childPath)
		})
	}
}

func TestReadwriteTx_UpdateMissingForeignKeyParentFailsWithoutChangingChild(t *testing.T) {
	t.Parallel()
	db, _ := setupForeignKeyDB(t, "parents")
	parentData := map[string]any{"name": "Parent"}
	if err := insertForeignKeyRecord(t, db, "parents", "parent-1", parentData); err != nil {
		t.Fatalf("insert parent: %v", err)
	}
	childData := map[string]any{"name": "Child", "parent_id": "parent-1"}
	if err := insertForeignKeyRecord(t, db, "children", "child-1", childData); err != nil {
		t.Fatalf("insert child: %v", err)
	}

	updates := []update.Update{update.ByFieldName("parent_id", "missing-parent")}
	err := updateForeignKeyRecord(t, db, "children", "child-1", updates)
	requireErrorContainsAll(t, err, "Update", "children", "parent_id", "parents", "missing-parent")

	data := readForeignKeyRecordData(t, db, "children", "child-1")
	if data["parent_id"] != "parent-1" {
		t.Fatalf("child parent_id = %v, want parent-1", data["parent_id"])
	}
}

func TestReadwriteTx_SetMissingForeignKeyParentFailsWithoutWritingChild(t *testing.T) {
	t.Parallel()
	db, root := setupForeignKeyDB(t, "parents")
	childData := map[string]any{"name": "Child", "parent_id": "missing-parent"}

	err := setForeignKeyRecord(t, db, "children", "child-1", childData)
	requireErrorContainsAll(t, err, "Set", "children", "parent_id", "parents", "missing-parent")

	childPath := childRecordPath(root, "child-1")
	requireNoRecordFile(t, childPath)
}

func TestReadwriteTx_DeleteReferencedParentFailsWithoutDeletingParent(t *testing.T) {
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

	err := deleteForeignKeyRecord(t, db, "parents", "parent-1")
	requireErrorContainsAll(t, err, "Delete", "parents", "parent-1", "children", "child-1", "parent_id")

	parentPath := collectionRecordPath(root, "parents", "parent-1")
	if _, statErr := os.Stat(parentPath); statErr != nil {
		t.Fatalf("parent record file: stat: %v", statErr)
	}
}

func TestReadwriteTx_DeleteUnreferencedParentSucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupForeignKeyDB(t, "parents")
	parentData := map[string]any{"name": "Referenced parent"}
	if err := insertForeignKeyRecord(t, db, "parents", "parent-1", parentData); err != nil {
		t.Fatalf("insert parent-1: %v", err)
	}
	unreferencedParentData := map[string]any{"name": "Unreferenced parent"}
	if err := insertForeignKeyRecord(t, db, "parents", "parent-2", unreferencedParentData); err != nil {
		t.Fatalf("insert parent-2: %v", err)
	}
	childData := map[string]any{"name": "Child", "parent_id": "parent-1"}
	if err := insertForeignKeyRecord(t, db, "children", "child-1", childData); err != nil {
		t.Fatalf("insert child: %v", err)
	}

	err := deleteForeignKeyRecord(t, db, "parents", "parent-2")
	if err != nil {
		t.Fatalf("delete unreferenced parent: %v", err)
	}

	parentPath := collectionRecordPath(root, "parents", "parent-2")
	requireNoRecordFile(t, parentPath)
}
