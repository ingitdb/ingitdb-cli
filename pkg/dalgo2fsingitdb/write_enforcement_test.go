package dalgo2fsingitdb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeFKTestDB builds a filesystem DB with a parent "users" collection and a
// child "things" collection whose columns are supplied by the caller, so each
// test can exercise stored foreign keys, computed foreign keys, or computed
// columns in isolation.
func makeFKTestDB(t *testing.T, childColumns map[string]*ingitdb.ColumnDef) dal.DB {
	t.Helper()
	dir := t.TempDir()
	singleYAML := func() *ingitdb.RecordFileDef {
		return &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord}
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				DirPath:    filepath.Join(dir, "users"),
				RecordFile: singleYAML(),
				Columns:    map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
			},
			"things": {
				ID:         "things",
				DirPath:    filepath.Join(dir, "things"),
				RecordFile: singleYAML(),
				Columns:    childColumns,
			},
		},
	}
	db, err := NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}
	return db
}

func insertRecord(t *testing.T, db dal.DB, collection, key string, data map[string]any) error {
	t.Helper()
	return db.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(dal.NewKeyWithID(collection, key), data))
	})
}

func deleteRecord(t *testing.T, db dal.DB, collection, key string) error {
	t.Helper()
	return db.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID(collection, key))
	})
}

func requireErrContains(t *testing.T, err error, subs ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %v, got nil", subs)
	}
	for _, sub := range subs {
		if !strings.Contains(err.Error(), sub) {
			t.Fatalf("error %q does not contain %q", err.Error(), sub)
		}
	}
}

var storedFKColumns = map[string]*ingitdb.ColumnDef{
	"name":  {Type: ingitdb.ColumnTypeString},
	"owner": {Type: ingitdb.ColumnTypeString, ForeignKey: "users"},
}

var computedFKColumns = map[string]*ingitdb.ColumnDef{
	"owner_input": {Type: ingitdb.ColumnTypeString},
	"owner_key":   {Type: ingitdb.ColumnTypeString, Formula: `"user-" + owner_input`, ForeignKey: "users"},
}

// --- Insert: stored foreign key -------------------------------------------------

func TestFSInsert_StoredForeignKeyMissingParentFails(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, storedFKColumns)
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"name": "T", "owner": "user-absent"})
	requireErrContains(t, err, "things", "owner", "users")
}

func TestFSInsert_StoredForeignKeyExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, storedFKColumns)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"name": "T", "owner": "user-1"}); err != nil {
		t.Fatalf("insert thing with valid FK should succeed: %v", err)
	}
}

// --- Insert: computed foreign key ----------------------------------------------

func TestFSInsert_ComputedForeignKeyMissingParentFails(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, computedFKColumns)
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "absent"})
	requireErrContains(t, err, "things", "owner_key", "users")
}

func TestFSInsert_ComputedForeignKeyExistingParentSucceeds(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, computedFKColumns)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing whose computed owner_key resolves should succeed: %v", err)
	}
}

// --- Insert: reject a supplied computed value -----------------------------------

func TestFSInsert_RejectsStoredComputedValue(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, map[string]*ingitdb.ColumnDef{
		"owner_input": {Type: ingitdb.ColumnTypeString},
		"display":     {Type: ingitdb.ColumnTypeString, Formula: "owner_input"},
	})
	err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "x", "display": "hand-written"})
	requireErrContains(t, err, "things", "display", "computed")
}

// --- Set: enforcement also applies ---------------------------------------------

func TestFSSet_StoredForeignKeyMissingParentFails(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, storedFKColumns)
	err := db.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, dal.NewRecordWithData(dal.NewKeyWithID("things", "thing-1"), map[string]any{"name": "T", "owner": "user-absent"}))
	})
	requireErrContains(t, err, "things", "owner", "users")
}

// --- Delete: parent-side enforcement -------------------------------------------

func TestFSDelete_StoredForeignKeyReferencedParentFails(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, storedFKColumns)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"name": "T", "owner": "user-1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}
	err := deleteRecord(t, db, "users", "user-1")
	requireErrContains(t, err, "things", "thing-1", "user-1")
}

func TestFSDelete_ComputedForeignKeyReferencedParentFails(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, computedFKColumns)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := insertRecord(t, db, "things", "thing-1", map[string]any{"owner_input": "1"}); err != nil {
		t.Fatalf("insert thing: %v", err)
	}
	err := deleteRecord(t, db, "users", "user-1")
	requireErrContains(t, err, "things", "thing-1", "owner_key", "user-1")
}

func TestFSDelete_UnreferencedParentSucceeds(t *testing.T) {
	t.Parallel()
	db := makeFKTestDB(t, storedFKColumns)
	if err := insertRecord(t, db, "users", "user-1", map[string]any{"name": "Owner"}); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := deleteRecord(t, db, "users", "user-1"); err != nil {
		t.Fatalf("deleting an unreferenced parent should succeed: %v", err)
	}
}
