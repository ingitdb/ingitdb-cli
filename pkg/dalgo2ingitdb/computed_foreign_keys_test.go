package dalgo2ingitdb_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// setupComputedForeignKeyDB creates a "users" collection and a "things"
// collection whose computed "owner_key" column derives a foreign key into
// "users" from the stored "owner_input" field via a formula.
func setupComputedForeignKeyDB(t *testing.T) (dal.DB, string) {
	t.Helper()
	return setupComputedForeignKeyDBWith(t, "users", `"user-" + owner_input`, dbschema.String)
}

// setupComputedForeignKeyDBWith builds the users/things schema with a
// configurable foreign-key target collection, formula, and owner_input type so
// individual tests can exercise the missing-collection, lookup-error, and
// non-string-key derivation paths.
func setupComputedForeignKeyDBWith(t *testing.T, fkTarget, formula string, inputType dbschema.Type) (dal.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)
	users := dbschema.CollectionDef{
		Name: "users",
		Fields: []dbschema.FieldDef{
			{Name: "name", Type: dbschema.String, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), users); err != nil {
		t.Fatalf("CreateCollection users: %v", err)
	}
	things := dbschema.CollectionDef{
		Name: "things",
		Fields: []dbschema.FieldDef{
			{Name: "owner_input", Type: inputType, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), things); err != nil {
		t.Fatalf("CreateCollection things: %v", err)
	}
	inputTypeName := "string"
	if inputType == dbschema.Int {
		inputTypeName = "int"
	}
	definition := `record_file:
    name: "{key}.yaml"
    format: yaml
    type: map[string]any
columns:
    owner_input:
        type: ` + inputTypeName + `
        required: true
    owner_key:
        type: string
        formula: '` + formula + `'
        foreign_key: ` + fkTarget + `
columns_order:
    - owner_input
    - owner_key
`
	schemaDir := filepath.Join(root, "things", ".collection")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("mkdir schema dir: %v", err)
	}
	definitionPath := filepath.Join(schemaDir, "definition.yaml")
	if err := os.WriteFile(definitionPath, []byte(definition), 0o644); err != nil {
		t.Fatalf("write things definition: %v", err)
	}
	return db, root
}

func thingRecordPath(root, key string) string {
	return filepath.Join(root, "things", "$records", key+".yaml")
}

// AC: foreign-key-on-insert-violation
func TestReadwriteTx_InsertComputedForeignKeyMissingParentFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	data := map[string]any{"owner_input": "absent"}
	err := insertRecord(t, db, "things", "thing-1", data)
	requireErrorContainsAll(t, err, "Insert", "things", "thing-1", "owner_key", "users", "user-absent", "parent record not found")
	requireNoRecordFile(t, thingRecordPath(root, "thing-1"))
}

func TestReadwriteTx_InsertComputedForeignKeyExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	data := map[string]any{"owner_input": "1"}
	if err := insertRecord(t, db, "things", "thing-1", data); err != nil {
		t.Fatalf("insert thing: %v", err)
	}
	if _, err := os.Stat(thingRecordPath(root, "thing-1")); err != nil {
		t.Fatalf("thing record file: stat: %v", err)
	}
}

// AC: foreign-key-revalidates-on-input-change
// Set backs update: changing the formula input field re-evaluates owner_key and
// re-validates the derived foreign key, even though owner_key is never written.
func TestReadwriteTx_UpdateComputedForeignKeyInputToMissingParentFails(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	// Set with the changed input field; owner_key (the FK column) is not written.
	err := setRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "absent"})
	requireErrorContainsAll(t, err, "Set", "things", "thing-1", "owner_key", "users", "user-absent", "parent record not found")

	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "1" {
		t.Fatalf("things owner_input = %v, want 1", data["owner_input"])
	}
}

func TestReadwriteTx_UpdateComputedForeignKeyInputToExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	if err := setRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "2"}); err != nil {
		t.Fatalf("update thing: %v", err)
	}
	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "2" {
		t.Fatalf("things owner_input = %v, want 2", data["owner_input"])
	}
}

func TestReadwriteTx_InsertComputedForeignKeyEvalErrorFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	// owner_input is an int64 here; the formula concatenates a string with an
	// int, which is a Starlark type error, exercising the eval-error path.
	data := map[string]any{"owner_input": int64(5)}
	err := insertRecord(t, db, "things", "thing-1", data)
	requireErrorContainsAll(t, err, "Insert", "things", "thing-1", "owner_key", "users", "evaluation failed")
	requireNoRecordFile(t, thingRecordPath(root, "thing-1"))
}

func TestReadwriteTx_InsertComputedForeignKeyMissingTargetCollectionFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDBWith(t, "ghosts", `"user-" + owner_input`, dbschema.String)
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"})
	requireErrorContainsAll(t, err, "Insert", "things", "thing-1", "owner_key", "ghosts", "configuration error")
	requireNoRecordFile(t, thingRecordPath(root, "thing-1"))
}

// Non-string derived key: the formula yields an int64, which is coerced to its
// decimal string form before the parent lookup, matching stored-FK handling.
func TestReadwriteTx_InsertComputedForeignKeyIntKeySucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDBWith(t, "users", `owner_input + 1`, dbschema.Int)
	if err := insertRecord(t, db, "users", "6", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": int64(5)}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}
	if _, err := os.Stat(thingRecordPath(root, "thing-1")); err != nil {
		t.Fatalf("thing record file: stat: %v", err)
	}
}

// Nil derived key: a formula yielding None coerces to an empty key, which never
// resolves to a parent, so the write fails with the referential-integrity error.
func TestReadwriteTx_InsertComputedForeignKeyNilKeyFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDBWith(t, "users", "None", dbschema.String)
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"})
	requireErrorContainsAll(t, err, "Insert", "things", "thing-1", "owner_key", "users", "parent record not found")
	requireNoRecordFile(t, thingRecordPath(root, "thing-1"))
}

// Lookup error: the resolved parent record file is corrupt, so the existence
// check returns an error rather than a clean found/not-found result.
func TestReadwriteTx_InsertComputedForeignKeyLookupErrorFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	corruptPath := collectionRecordPath(root, "users", "user-bad")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0o755); err != nil {
		t.Fatalf("mkdir users records: %v", err)
	}
	if err := os.WriteFile(corruptPath, []byte("\t: : not yaml :"), 0o644); err != nil {
		t.Fatalf("write corrupt parent: %v", err)
	}
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "bad"})
	requireErrorContainsAll(t, err, "Insert", "things", "thing-1", "owner_key", "users", "user-bad", "lookup failed")
	requireNoRecordFile(t, thingRecordPath(root, "thing-1"))
}

// A corrupt child records file makes readAllRecordsFromDisk fail during the
// parent-side computed-FK scan, so the delete returns the wrapped scan error.
func TestReadwriteTx_DeleteComputedForeignKeyScanErrorFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	// Corrupt the things records file so the scan's read fails.
	badPath := thingRecordPath(root, "thing-bad")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("mkdir things records: %v", err)
	}
	if err := os.WriteFile(badPath, []byte("\t: : not yaml :"), 0o644); err != nil {
		t.Fatalf("write corrupt thing: %v", err)
	}

	err := deleteComputedForeignKeyRecord(t, db, "users", "user-1")
	requireErrorContainsAll(t, err, "Delete computed foreign key scan failed", "users", "user-1", "things")

	// The blocked delete is a no-op: the parent must remain on disk.
	if _, statErr := os.Stat(collectionRecordPath(root, "users", "user-1")); statErr != nil {
		t.Fatalf("user record file: stat: %v", statErr)
	}
}

// A child whose computed foreign key resolves to empty is skipped by the
// parent-side scan, so deleting an unreferenced parent succeeds while a separate
// child that resolves to the parent still blocks its delete.
func TestReadwriteTx_DeleteComputedForeignKeyEmptyChildSkipped(t *testing.T) {
	t.Parallel()
	// owner_key derives "" when owner_input is "none" and "user-..." otherwise.
	formula := `("user-" + owner_input) if owner_input != "none" else ""`
	db, root := setupComputedForeignKeyDBWith(t, "users", formula, dbschema.String)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	// thing-ref's owner_key resolves to user-1.
	if err := insertRecord(t, db, "things", "thing-ref", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing-ref: %v", err)
	}
	// thing-empty's owner_key resolves to "" and so cannot be inserted through the
	// write path (write validation rejects an empty computed FK); write it to disk
	// directly so the delete-side scan recomputes its empty key and skips it.
	if err := os.WriteFile(thingRecordPath(root, "thing-empty"), []byte("owner_input: none\n"), 0o644); err != nil {
		t.Fatalf("write thing-empty: %v", err)
	}

	// user-2 is referenced by no thing (thing-empty's FK is skipped), so deleting
	// it succeeds.
	if err := deleteComputedForeignKeyRecord(t, db, "users", "user-2"); err != nil {
		t.Fatalf("delete unreferenced user: %v", err)
	}
	requireNoRecordFile(t, collectionRecordPath(root, "users", "user-2"))

	// user-1 is still referenced by thing-ref, so its delete is blocked.
	err := deleteComputedForeignKeyRecord(t, db, "users", "user-1")
	requireErrorContainsAll(t, err, "things", "thing-ref", "owner_key", "users", "user-1")
}

func deleteComputedForeignKeyRecord(t *testing.T, db dal.DB, collection, key string) error {
	t.Helper()
	ctx := context.Background()
	return db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		return tx.Delete(ctx, recordKey)
	})
}

func updateComputedForeignKeyRecord(t *testing.T, db dal.DB, collection, key string, updates []update.Update) error {
	t.Helper()
	ctx := context.Background()
	return db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		return tx.Update(ctx, recordKey, updates)
	})
}

func updateRecordComputedForeignKey(t *testing.T, db dal.DB, collection, key string, current map[string]any, updates []update.Update) error {
	t.Helper()
	ctx := context.Background()
	return db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		recordKey := dal.NewKeyWithID(collection, key)
		record := dal.NewRecordWithData(recordKey, current)
		// SetError(nil) marks the record loaded so UpdateRecord can read its Data.
		record.SetError(nil)
		return tx.UpdateRecord(ctx, record, updates)
	})
}

// AC: foreign-key-parent-delete-detected
// Deleting a users record still referenced by a things record's computed
// owner_key is blocked with the reference-error shape: referencing collection
// things, referencing record key, the owner_key column, and referenced
// collection users.
func TestReadwriteTx_DeleteReferencedByComputedForeignKeyFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	err := deleteComputedForeignKeyRecord(t, db, "users", "user-1")
	requireErrorContainsAll(t, err, "things", "thing-1", "owner_key", "users", "user-1")

	// The referenced parent must remain on disk: the blocked delete is a no-op.
	if _, statErr := os.Stat(collectionRecordPath(root, "users", "user-1")); statErr != nil {
		t.Fatalf("user record file: stat: %v", statErr)
	}
}

// Multiple child records force the deterministic sort comparator to run and the
// scan to skip non-matching children before reaching the one that resolves to the
// deleted parent key.
func TestReadwriteTx_DeleteReferencedByComputedForeignKeyMultipleChildrenFails(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	// thing-a -> user-2, thing-b -> user-1: two records so the comparator runs and
	// the first scanned record does not match the deleted key.
	if err := insertRecord(t, db, "things", "thing-a", map[string]any{"owner_input": "2"}); err != nil {
		t.Fatalf("insert thing-a: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-b", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing-b: %v", err)
	}

	err := deleteComputedForeignKeyRecord(t, db, "users", "user-1")
	requireErrorContainsAll(t, err, "things", "thing-b", "owner_key", "users", "user-1")
}

// Deleting a users record that no things record's computed owner_key resolves to
// succeeds: the parent-side scan finds no referencing child.
func TestReadwriteTx_DeleteNotReferencedByComputedForeignKeySucceeds(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	// user-2 is referenced by no thing, so its delete succeeds.
	if err := deleteComputedForeignKeyRecord(t, db, "users", "user-2"); err != nil {
		t.Fatalf("delete unreferenced user: %v", err)
	}
	requireNoRecordFile(t, collectionRecordPath(root, "users", "user-2"))
}

// AC: foreign-key-parent-rename-detected
// A key rename has no dedicated code path in this driver: it manifests as the old
// key ceasing to exist. Removing the old referenced key while a things record
// still resolves to it triggers the same parent-side scan and is blocked with the
// reference-error shape.
func TestReadwriteTx_RenameReferencedByComputedForeignKeyFails(t *testing.T) {
	t.Parallel()
	db, root := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	// Rename user-1 -> user-renamed: the old key user-1 is removed, which the
	// thing's computed owner_key still resolves to, so the removal is blocked.
	err := deleteComputedForeignKeyRecord(t, db, "users", "user-1")
	requireErrorContainsAll(t, err, "things", "thing-1", "owner_key", "users", "user-1")
	if _, statErr := os.Stat(collectionRecordPath(root, "users", "user-1")); statErr != nil {
		t.Fatalf("old user record file: stat: %v", statErr)
	}
}

// AC: foreign-key-revalidates-on-input-change
// Update threads the changed input field through validateComputedWriteForeignKeys
// so a field-level Update that makes owner_key resolve to a missing parent fails.
func TestReadwriteTx_UpdateMethodComputedForeignKeyInputToMissingParentFails(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	updates := []update.Update{update.ByFieldName("owner_input", "absent")}
	err := updateComputedForeignKeyRecord(t, db, "things", "thing-1", updates)
	requireErrorContainsAll(t, err, "Update", "things", "thing-1", "owner_key", "users", "user-absent", "parent record not found")

	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "1" {
		t.Fatalf("things owner_input = %v, want 1", data["owner_input"])
	}
}

func TestReadwriteTx_UpdateMethodComputedForeignKeyInputToExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	updates := []update.Update{update.ByFieldName("owner_input", "2")}
	if err := updateComputedForeignKeyRecord(t, db, "things", "thing-1", updates); err != nil {
		t.Fatalf("update thing: %v", err)
	}
	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "2" {
		t.Fatalf("things owner_input = %v, want 2", data["owner_input"])
	}
}

// AC: foreign-key-revalidates-on-input-change
// UpdateRecord also threads the changed input field through
// validateComputedWriteForeignKeys, so an UpdateRecord that makes owner_key
// resolve to a missing parent fails.
func TestReadwriteTx_UpdateRecordComputedForeignKeyInputToMissingParentFails(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	current := map[string]any{"owner_input": "1"}
	updates := []update.Update{update.ByFieldName("owner_input", "absent")}
	err := updateRecordComputedForeignKey(t, db, "things", "thing-1", current, updates)
	requireErrorContainsAll(t, err, "UpdateRecord", "things", "thing-1", "owner_key", "users", "user-absent", "parent record not found")

	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "1" {
		t.Fatalf("things owner_input = %v, want 1", data["owner_input"])
	}
}

func TestReadwriteTx_UpdateRecordComputedForeignKeyInputToExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db, _ := setupComputedForeignKeyDB(t)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner1"}); err != nil {
		t.Fatalf("insert user-1: %v", err)
	}
	if err := insertRecord(t, db, "users", "user-2", map[string]any{"name": "Owner2"}); err != nil {
		t.Fatalf("insert user-2: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}

	current := map[string]any{"owner_input": "1"}
	updates := []update.Update{update.ByFieldName("owner_input", "2")}
	if err := updateRecordComputedForeignKey(t, db, "things", "thing-1", current, updates); err != nil {
		t.Fatalf("update record thing: %v", err)
	}
	data := readForeignKeyRecordData(t, db, "things", "thing-1")
	if data["owner_input"] != "2" {
		t.Fatalf("things owner_input = %v, want 2", data["owner_input"])
	}
}
