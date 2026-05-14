package dalgo2ingitdb

// coverage_gaps_test.go covers branches that were below 90% after the initial
// test suite. All tests are in the same package (white-box) so they can reach
// unexported helpers directly.
//
// Note: newReader(), tagsCollectionDef(), and writeCollectionDef() are
// declared in database_test.go, schema_modifier_test.go, and schema_reader_test.go
// respectively; they are used here without re-declaration.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"github.com/dal-go/dalgo/update"
	"github.com/ingr-io/ingr-go/ingr"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// database.go — ID, Adapter, Schema, ExecuteQueryToRecordsetReader,
//               loadDefinition with nil reader
// ---------------------------------------------------------------------------

func TestDatabase_ID(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if db.(*Database).ID() != DatabaseID {
		t.Errorf("ID: got %q, want %q", db.(*Database).ID(), DatabaseID)
	}
}

func TestDatabase_Adapter(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	a := db.(*Database).Adapter()
	if a.Name() != DatabaseID {
		t.Errorf("Adapter.Name: got %q, want %q", a.Name(), DatabaseID)
	}
}

func TestDatabase_Schema_ReturnsNil(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if db.(*Database).Schema() != nil {
		t.Error("Schema: want nil")
	}
}

func TestDatabase_ExecuteQueryToRecordsetReader_NotSupported(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	_, gotErr := db.(*Database).ExecuteQueryToRecordsetReader(context.Background(), nil)
	if gotErr == nil {
		t.Fatal("want error from ExecuteQueryToRecordsetReader")
	}
}

func TestDatabase_loadDefinition_NilReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Construct without the exported constructor so we can pass nil reader.
	db := &Database{projectPath: root, reader: nil}
	_, err := db.loadDefinition()
	if err == nil {
		t.Fatal("want error when reader is nil")
	}
	if !strings.Contains(err.Error(), "CollectionsReader") {
		t.Errorf("error should mention CollectionsReader, got: %v", err)
	}
}

// RunReadonlyTransaction with a broken reader returns the reader's error.
func TestDatabase_RunReadonlyTransaction_LoadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db := &Database{projectPath: root, reader: nil}
	err := db.RunReadonlyTransaction(context.Background(), func(_ context.Context, _ dal.ReadTransaction) error {
		return nil
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDatabase_RunReadwriteTransaction_LoadError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db := &Database{projectPath: root, reader: nil}
	err := db.RunReadwriteTransaction(context.Background(), func(_ context.Context, _ dal.ReadwriteTransaction) error {
		return nil
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDatabase_Get_LoadError(t *testing.T) {
	t.Parallel()
	db := &Database{projectPath: t.TempDir(), reader: nil}
	err := db.Get(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDatabase_Exists_LoadError(t *testing.T) {
	t.Parallel()
	db := &Database{projectPath: t.TempDir(), reader: nil}
	_, err := db.Exists(context.Background(), dal.NewKeyWithID("c", "k"))
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDatabase_GetMulti_LoadError(t *testing.T) {
	t.Parallel()
	db := &Database{projectPath: t.TempDir(), reader: nil}
	err := db.GetMulti(context.Background(), []dal.Record{
		dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}),
	})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestDatabase_ExecuteQueryToRecordsReader_LoadError(t *testing.T) {
	t.Parallel()
	db := &Database{projectPath: t.TempDir(), reader: nil}
	q := dal.From(dal.NewRootCollectionRef("c", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("c", ""), map[string]any{})
		})
	_, err := db.ExecuteQueryToRecordsReader(context.Background(), q)
	if err == nil {
		t.Fatal("want error")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go — Options, ExecuteQueryToRecordsetReader, resolveCollection
//                  error branches, MapOfRecords Get / Exists
// ---------------------------------------------------------------------------

func TestReadonlyTx_Options(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{}
	_ = tx.Options() // just confirm it doesn't panic
}

func TestReadonlyTx_ExecuteQueryToRecordsetReader_NotSupported(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{}
	_, err := tx.ExecuteQueryToRecordsetReader(context.Background(), nil)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestReadonlyTx_Get_NilKey(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	// resolveCollection returns an error when key is nil; test via a real key
	// that passes dal.NewRecordWithData but has nil ID path covered by resolveCollection.
	err := tx.resolveNilKey()
	if err == nil {
		t.Fatal("want error for nil key")
	}
}

// resolveNilKey exercises the nil-key branch in resolveCollection without
// going through dal.NewRecordWithData (which panics on nil key).
func (r readonlyTx) resolveNilKey() error {
	_, _, err := r.resolveCollection(nil)
	return err
}

func TestReadonlyTx_Get_NilDefinition(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{def: nil}
	err := tx.Get(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error when def is nil")
	}
}

func TestReadonlyTx_Get_CollectionNotFound(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	err := tx.Get(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("missing", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error for missing collection")
	}
}

func TestReadonlyTx_Get_NoRecordFile(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {ID: "c", RecordFile: nil},
		}},
	}
	err := tx.Get(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error for missing RecordFile")
	}
}

func TestReadonlyTx_Get_UnknownRecordType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: root,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.RecordType("unknown"),
				},
			},
		}},
	}
	err := tx.Get(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error for unknown record type")
	}
}

func TestReadonlyTx_Exists_UnknownRecordType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: root,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.RecordType("unknown"),
				},
			},
		}},
	}
	_, err := tx.Exists(context.Background(), dal.NewKeyWithID("c", "k"))
	if err == nil {
		t.Fatal("want error for unknown record type")
	}
}

// makeMapOfRecordsTx builds a readonlyTx for a MapOfRecords collection.
func makeMapOfRecordsTx(t *testing.T, root string) (readonlyTx, *ingitdb.CollectionDef) {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: filepath.Join(root, "scores"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	return readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"scores": colDef,
		}},
	}, colDef
}

func TestReadonlyTx_Get_MapOfRecords_NotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, _ := makeMapOfRecordsTx(t, root)
	// No file on disk → ErrRecordNotFound via SetError.
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	if err := tx.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if rec.Exists() {
		t.Error("rec.Exists: want false for missing map-of-records entry")
	}
}

func TestReadonlyTx_Get_MapOfRecords_KeyMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeMapOfRecordsTx(t, root)
	// Write a file with one entry.
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("bob:\n  score: 42\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	if err := tx.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if rec.Exists() {
		t.Error("rec.Exists: want false for absent key in map-of-records file")
	}
}

func TestReadonlyTx_Get_MapOfRecords_Found(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeMapOfRecordsTx(t, root)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("alice:\n  score: 99\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	if err := tx.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !rec.Exists() {
		t.Error("rec.Exists: want true")
	}
}

func TestReadonlyTx_Exists_MapOfRecords_FileMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, _ := makeMapOfRecordsTx(t, root)
	exists, err := tx.Exists(context.Background(), dal.NewKeyWithID("scores", "alice"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("Exists: want false when file missing")
	}
}

func TestReadonlyTx_Exists_MapOfRecords_KeyPresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeMapOfRecordsTx(t, root)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("alice:\n  score: 7\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	exists, err := tx.Exists(context.Background(), dal.NewKeyWithID("scores", "alice"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("Exists: want true")
	}
}

func TestReadonlyTx_Exists_MapOfRecords_KeyAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeMapOfRecordsTx(t, root)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("bob:\n  score: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	exists, err := tx.Exists(context.Background(), dal.NewKeyWithID("scores", "alice"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("Exists: want false for absent key")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go — ID, SetMulti, InsertMulti, DeleteMulti, UpdateRecord,
//                   UpdateMulti, MapOfRecords Set/Insert/Delete,
//                   applyUpdates with FieldPath
// ---------------------------------------------------------------------------

func TestReadwriteTx_ID(t *testing.T) {
	t.Parallel()
	tx := readwriteTx{}
	if tx.ID() != "" {
		t.Errorf("ID: got %q, want empty string", tx.ID())
	}
}

// makeReadwriteTx builds a readwriteTx backed by a real temp DB with a
// single YAML SingleRecord collection.
func makeReadwriteTx(t *testing.T) (readwriteTx, string) {
	t.Helper()
	root := t.TempDir()
	db := &Database{projectPath: root, reader: newReader()}
	colDef := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(root, "items"),
		ColumnsOrder: []string{"name"},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: db,
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"items": colDef},
		},
	}}
	return tx, root
}

func TestReadwriteTx_SetMulti(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Join(root, "items", "$records"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r1 := dal.NewRecordWithData(dal.NewKeyWithID("items", "a"), map[string]any{"name": "A"})
	r2 := dal.NewRecordWithData(dal.NewKeyWithID("items", "b"), map[string]any{"name": "B"})
	if err := tx.SetMulti(context.Background(), []dal.Record{r1, r2}); err != nil {
		t.Fatalf("SetMulti: %v", err)
	}
	p := filepath.Join(root, "items", "$records", "a.yaml")
	if _, err := os.Stat(p); err != nil {
		t.Errorf("after SetMulti: a.yaml missing: %v", err)
	}
}

func TestReadwriteTx_InsertMulti(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	if err := os.MkdirAll(filepath.Join(root, "items", "$records"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	r1 := dal.NewRecordWithData(dal.NewKeyWithID("items", "x"), map[string]any{"name": "X"})
	r2 := dal.NewRecordWithData(dal.NewKeyWithID("items", "y"), map[string]any{"name": "Y"})
	if err := tx.InsertMulti(context.Background(), []dal.Record{r1, r2}); err != nil {
		t.Fatalf("InsertMulti: %v", err)
	}
	for _, key := range []string{"x", "y"} {
		p := filepath.Join(root, "items", "$records", key+".yaml")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("after InsertMulti: %s.yaml missing: %v", key, err)
		}
	}
}

func TestReadwriteTx_DeleteMulti(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	dir := filepath.Join(root, "items", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Seed two records.
	for _, key := range []string{"m", "n"} {
		p := filepath.Join(dir, key+".yaml")
		if err := os.WriteFile(p, []byte("name: "+key+"\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	keys := []*dal.Key{dal.NewKeyWithID("items", "m"), dal.NewKeyWithID("items", "n")}
	if err := tx.DeleteMulti(context.Background(), keys); err != nil {
		t.Fatalf("DeleteMulti: %v", err)
	}
	for _, key := range []string{"m", "n"} {
		p := filepath.Join(dir, key+".yaml")
		if _, err := os.Stat(p); err == nil {
			t.Errorf("after DeleteMulti: %s.yaml should be gone", key)
		}
	}
}

func TestReadwriteTx_UpdateRecord(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	dir := filepath.Join(root, "items", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, "z.yaml")
	if err := os.WriteFile(p, []byte("name: old\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("items", "z"), map[string]any{"name": "old"})
	// SetError(nil) must be called before UpdateRecord can call rec.Data().
	rec.SetError(nil)
	ups := []update.Update{update.ByFieldName("name", "new")}
	if err := tx.UpdateRecord(context.Background(), rec, ups); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["name"] != "new" {
		t.Errorf("after UpdateRecord: name = %v, want new", got["name"])
	}
}

func TestReadwriteTx_UpdateRecord_PreconditionsRejected(t *testing.T) {
	t.Parallel()
	tx, _ := makeReadwriteTx(t)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("items", "z"), map[string]any{})
	err := tx.UpdateRecord(context.Background(), rec, nil, dal.WithExistsPrecondition())
	if err == nil {
		t.Fatal("want error for preconditions in UpdateRecord")
	}
}

func TestReadwriteTx_UpdateMulti(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	dir := filepath.Join(root, "items", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, key := range []string{"p", "q"} {
		p := filepath.Join(dir, key+".yaml")
		if err := os.WriteFile(p, []byte("name: old\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	keys := []*dal.Key{dal.NewKeyWithID("items", "p"), dal.NewKeyWithID("items", "q")}
	ups := []update.Update{update.ByFieldName("name", "updated")}
	if err := tx.UpdateMulti(context.Background(), keys, ups); err != nil {
		t.Fatalf("UpdateMulti: %v", err)
	}
	for _, key := range []string{"p", "q"} {
		p := filepath.Join(dir, key+".yaml")
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		var got map[string]any
		if err := yaml.Unmarshal(content, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got["name"] != "updated" {
			t.Errorf("%s: name = %v, want updated", key, got["name"])
		}
	}
}

func TestApplyUpdates_FieldPath(t *testing.T) {
	t.Parallel()
	data := map[string]any{"x": 1}
	ups := []update.Update{update.ByFieldPath(update.FieldPath{"score"}, 42)}
	if err := applyUpdates(data, ups); err != nil {
		t.Fatalf("applyUpdates with FieldPath: %v", err)
	}
	if data["score"] != 42 {
		t.Errorf("data[score]: got %v, want 42", data["score"])
	}
}

func TestApplyUpdates_NestedFieldPathRejected(t *testing.T) {
	t.Parallel()
	data := map[string]any{}
	ups := []update.Update{update.ByFieldPath(update.FieldPath{"a", "b"}, 1)}
	if err := applyUpdates(data, ups); err == nil {
		t.Fatal("want error for nested field path")
	}
}

// makeMapOfRecordsRWTx builds a readwriteTx for a MapOfRecords collection.
func makeMapOfRecordsRWTx(t *testing.T) (readwriteTx, *ingitdb.CollectionDef, string) {
	t.Helper()
	root := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: filepath.Join(root, "scores"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"scores": colDef},
		},
	}}
	return tx, colDef, root
}

func TestReadwriteTx_Set_MapOfRecords(t *testing.T) {
	t.Parallel()
	tx, colDef, _ := makeMapOfRecordsRWTx(t)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{"score": 100})
	if err := tx.Set(context.Background(), rec); err != nil {
		t.Fatalf("Set MapOfRecords: %v", err)
	}
	// Read back to verify.
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]map[string]any
	if err := yaml.Unmarshal(content, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["alice"]["score"] != 100 {
		t.Errorf("alice.score: got %v, want 100", m["alice"]["score"])
	}
}

func TestReadwriteTx_Insert_MapOfRecords(t *testing.T) {
	t.Parallel()
	tx, colDef, _ := makeMapOfRecordsRWTx(t)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "bob"), map[string]any{"score": 55})
	if err := tx.Insert(context.Background(), rec); err != nil {
		t.Fatalf("Insert MapOfRecords: %v", err)
	}
	// Duplicate insert must fail.
	rec2 := dal.NewRecordWithData(dal.NewKeyWithID("scores", "bob"), map[string]any{"score": 66})
	if err := tx.Insert(context.Background(), rec2); err == nil {
		t.Fatal("duplicate Insert MapOfRecords: want error")
	}
}

func TestReadwriteTx_Delete_MapOfRecords(t *testing.T) {
	t.Parallel()
	tx, colDef, _ := makeMapOfRecordsRWTx(t)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("eve:\n  score: 77\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Delete present key.
	if err := tx.Delete(context.Background(), dal.NewKeyWithID("scores", "eve")); err != nil {
		t.Fatalf("Delete present key: %v", err)
	}
	// Delete absent key — file still exists but key gone.
	if err := tx.Delete(context.Background(), dal.NewKeyWithID("scores", "eve")); err == nil {
		t.Fatal("delete absent key: want error")
	}
}

func TestReadwriteTx_Delete_MapOfRecords_FileMissing(t *testing.T) {
	t.Parallel()
	tx, colDef, _ := makeMapOfRecordsRWTx(t)
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No file → ErrRecordNotFound.
	err := tx.Delete(context.Background(), dal.NewKeyWithID("scores", "ghost"))
	if err == nil {
		t.Fatal("want ErrRecordNotFound when file missing")
	}
}

func TestReadwriteTx_Set_UnknownRecordType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: root,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.RecordType("unknown"),
				},
			},
		}},
	}}
	err := tx.Set(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error for unknown record type in Set")
	}
}

func TestReadwriteTx_Insert_UnknownRecordType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: root,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.RecordType("unknown"),
				},
			},
		}},
	}}
	err := tx.Insert(context.Background(), dal.NewRecordWithData(dal.NewKeyWithID("c", "k"), map[string]any{}))
	if err == nil {
		t.Fatal("want error for unknown record type in Insert")
	}
}

func TestReadwriteTx_Delete_UnknownRecordType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx := readwriteTx{readonlyTx: readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: root,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.RecordType("unknown"),
				},
			},
		}},
	}}
	err := tx.Delete(context.Background(), dal.NewKeyWithID("c", "k"))
	if err == nil {
		t.Fatal("want error for unknown record type in Delete")
	}
}

func TestReadwriteTx_Update_WithPreconditions(t *testing.T) {
	t.Parallel()
	tx, _ := makeReadwriteTx(t)
	err := tx.Update(context.Background(), dal.NewKeyWithID("items", "k"), nil, dal.WithExistsPrecondition())
	if err == nil {
		t.Fatal("want error: preconditions not supported")
	}
}

// ---------------------------------------------------------------------------
// record_io.go — readMapOfRecordsFile, writeMapOfRecordsFile
// ---------------------------------------------------------------------------

func TestReadMapOfRecordsFile_Missing(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "missing.yaml")
	result, found, err := readMapOfRecordsFile(p, ingitdb.RecordFormatYAML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("found: want false for missing file")
	}
	if result != nil {
		t.Errorf("result: want nil, got %v", result)
	}
}

func TestReadMapOfRecordsFile_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(p, []byte("{broken yaml"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := readMapOfRecordsFile(p, ingitdb.RecordFormatYAML)
	if err == nil {
		t.Fatal("want error for invalid YAML")
	}
}

func TestReadWriteMapOfRecordsFile_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "data.yaml")
	colDef := &ingitdb.CollectionDef{
		ID: "test",
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatYAML,
		},
	}
	original := map[string]map[string]any{
		"alice": {"score": 10},
		"bob":   {"score": 20},
	}
	if err := writeMapOfRecordsFile(p, colDef, original); err != nil {
		t.Fatalf("writeMapOfRecordsFile: %v", err)
	}
	result, found, err := readMapOfRecordsFile(p, ingitdb.RecordFormatYAML)
	if err != nil {
		t.Fatalf("readMapOfRecordsFile: %v", err)
	}
	if !found {
		t.Fatal("found: want true")
	}
	if len(result) != 2 {
		t.Errorf("len: got %d, want 2", len(result))
	}
}

// ---------------------------------------------------------------------------
// record_io.go — deleteSingleRecordFile, writeSingleRecordFile,
//                writeMapOfRecordsFile directory creation
// ---------------------------------------------------------------------------

func TestDeleteSingleRecordFile_MissingAndPresent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.yaml")

	// File missing before stat → ErrRecordNotFound.
	err := deleteSingleRecordFile(p)
	if err == nil {
		t.Fatal("want error for missing file")
	}

	// File exists → deleted successfully.
	if err := os.WriteFile(p, []byte("x: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := deleteSingleRecordFile(p); err != nil {
		t.Fatalf("deleteSingleRecordFile: %v", err)
	}
	if _, err := os.Stat(p); err == nil {
		t.Error("file should be gone after deleteSingleRecordFile")
	}
}

func TestWriteSingleRecordFile_CreatesAndWrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "rec.yaml")
	colDef := &ingitdb.CollectionDef{
		ID:      "c",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	data := map[string]any{"name": "test"}
	if err := writeSingleRecordFile(p, colDef, data); err != nil {
		t.Fatalf("writeSingleRecordFile: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file should exist after write: %v", err)
	}
}

func TestWriteMapOfRecordsFile_CreatesDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Path with a new subdirectory.
	p := filepath.Join(root, "newdir", "records.yaml")
	colDef := &ingitdb.CollectionDef{
		ID: "c",
		RecordFile: &ingitdb.RecordFileDef{
			Format: ingitdb.RecordFormatYAML,
		},
	}
	data := map[string]map[string]any{"key1": {"val": 1}}
	if err := writeMapOfRecordsFile(p, colDef, data); err != nil {
		t.Fatalf("writeMapOfRecordsFile: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

// ---------------------------------------------------------------------------
// query.go — readAllMapOfRecords, evaluateGroupCondition, toFloat64,
//            evaluateComparison operators, resolveExpression branches
// ---------------------------------------------------------------------------

func makeMapColDef(t *testing.T, root string) *ingitdb.CollectionDef {
	t.Helper()
	colDir := filepath.Join(root, "scores")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: colDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
}

func TestReadAllMapOfRecords_Empty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := makeMapColDef(t, root)
	// No file on disk.
	recs, err := readAllMapOfRecords(colDef)
	if err != nil {
		t.Fatalf("readAllMapOfRecords: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("want 0 records, got %d", len(recs))
	}
}

func TestReadAllMapOfRecords_WithData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDef := makeMapColDef(t, root)
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte("alice:\n  score: 5\nbob:\n  score: 3\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	recs, err := readAllMapOfRecords(colDef)
	if err != nil {
		t.Fatalf("readAllMapOfRecords: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("want 2 records, got %d", len(recs))
	}
}

func TestReadAllRecordsFromDisk_UnsupportedType(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: t.TempDir(),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.RecordType("unsupported"),
		},
	}
	_, err := readAllRecordsFromDisk(colDef)
	if err == nil {
		t.Fatal("want error for unsupported record type")
	}
}

func TestReadAllRecordsFromDisk_MapOfRecordsPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	colDir := filepath.Join(root, "scores")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDir, "scores.yaml")
	if err := os.WriteFile(p, []byte("a:\n  val: 1\nb:\n  val: 2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: colDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
	recs, err := readAllRecordsFromDisk(colDef)
	if err != nil {
		t.Fatalf("readAllRecordsFromDisk: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("want 2 records, got %d", len(recs))
	}
}

func TestBuildKeyExtractor_NoTemplate(t *testing.T) {
	t.Parallel()
	// nameTemplate has no {key} placeholder — falls back to stripping extension.
	fn, err := buildKeyExtractor("record.yaml")
	if err != nil {
		t.Fatalf("buildKeyExtractor: %v", err)
	}
	got := fn("record.yaml")
	if got != "record" {
		t.Errorf("got %q, want record", got)
	}
}

func TestBuildKeyExtractor_WithTemplate(t *testing.T) {
	t.Parallel()
	fn, err := buildKeyExtractor("{key}.yaml")
	if err != nil {
		t.Fatalf("buildKeyExtractor: %v", err)
	}
	got := fn("mykey.yaml")
	if got != "mykey" {
		t.Errorf("got %q, want mykey", got)
	}
	// Non-matching path returns empty string.
	got2 := fn("mykey.json")
	if got2 != "" {
		t.Errorf("non-matching: got %q, want empty", got2)
	}
}

func TestExecuteQueryToRecordsReader_NilDefinition(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{def: nil}
	q := dal.From(dal.NewRootCollectionRef("items", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("items", ""), map[string]any{})
		})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("want error for nil definition")
	}
}

func TestExecuteQueryToRecordsReader_CollectionNotFound(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	q := dal.From(dal.NewRootCollectionRef("noexist", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("noexist", ""), map[string]any{})
		})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("want error for missing collection")
	}
}

func TestExecuteQueryToRecordsReader_CollectionNoRecordFile(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items", RecordFile: nil},
		}},
	}
	q := dal.From(dal.NewRootCollectionRef("items", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("items", ""), map[string]any{})
		})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("want error for nil RecordFile")
	}
}

func TestExecuteQueryToRecordsReader_WithLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "items", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rec := range []struct{ key, content string }{
		{"a", "val: 1\n"},
		{"b", "val: 2\n"},
		{"c", "val: 3\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, rec.key+".yaml"), []byte(rec.content), 0o644); err != nil {
			t.Fatalf("seed %s: %v", rec.key, err)
		}
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "items",
		DirPath: filepath.Join(root, "items"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"items": colDef},
		},
	}
	q := dal.From(dal.NewRootCollectionRef("items", "")).NewQuery().
		Limit(2).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("items", ""), map[string]any{})
		})
	reader, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err != nil {
		t.Fatalf("executeQueryToRecordsReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	var count int
	for {
		_, nextErr := reader.Next()
		if nextErr == dal.ErrNoMoreRecords {
			break
		}
		if nextErr != nil {
			t.Fatalf("Next: %v", nextErr)
		}
		count++
	}
	if count != 2 {
		t.Errorf("want 2 records (limit=2), got %d", count)
	}
}

func TestApplyOrderBy_DescendingByVal(t *testing.T) {
	t.Parallel()
	makeRec := func(id string, val int) dal.Record {
		key := dal.NewKeyWithID("c", id)
		rec := dal.NewRecordWithData(key, map[string]any{"val": val})
		rec.SetError(nil)
		return rec
	}
	recs := []dal.Record{makeRec("a", 1), makeRec("b", 3), makeRec("c", 2)}
	// Build an order expression: descending by val.
	orderExprs := []dal.OrderExpression{dal.DescendingField("val")}
	applyOrderBy(recs, orderExprs)
	// After descending sort by val: b(3), c(2), a(1).
	if recs[0].Key().ID != "b" {
		t.Errorf("recs[0] = %v, want b", recs[0].Key().ID)
	}
	if recs[1].Key().ID != "c" {
		t.Errorf("recs[1] = %v, want c", recs[1].Key().ID)
	}
}

func TestApplyOrderBy_ByID(t *testing.T) {
	t.Parallel()
	makeRec := func(id string) dal.Record {
		key := dal.NewKeyWithID("c", id)
		rec := dal.NewRecordWithData(key, map[string]any{})
		rec.SetError(nil)
		return rec
	}
	recs := []dal.Record{makeRec("c"), makeRec("a"), makeRec("b")}
	// Order ascending by $id field.
	orderExprs := []dal.OrderExpression{dal.AscendingField("$id")}
	applyOrderBy(recs, orderExprs)
	if recs[0].Key().ID != "a" {
		t.Errorf("recs[0] = %v, want a", recs[0].Key().ID)
	}
}

func TestEvaluateCondition_UnsupportedType(t *testing.T) {
	t.Parallel()
	// unsupportedCond satisfies dal.Condition but is not Comparison or GroupCondition.
	_, err := evaluateCondition(unsupportedCond{}, map[string]any{}, "k")
	if err == nil {
		t.Fatal("want error for unsupported condition type")
	}
}

// unsupportedCond satisfies dal.Condition but is not Comparison or GroupCondition.
type unsupportedCond struct{}

func (unsupportedCond) String() string { return "unsupported" }

// evaluateGroupCondition is tested via the query builder path because
// dal.GroupCondition has unexported fields that can only be set inside the dal
// package; we cannot construct one with a known operator from outside.
// TestEvaluateGroupCondition_ViaQuery exercises all AND/OR code paths.

func TestEvaluateGroupCondition_ViaQuery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Set up a collection with two records, use a WhereField AND query.
	db := &Database{projectPath: root, reader: newReader()}
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	dir := filepath.Join(root, "things", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, rec := range []struct{ key, content string }{
		{"a", "val: 5\n"},
		{"b", "val: 15\n"},
		{"c", "val: 25\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, rec.key+".yaml"), []byte(rec.content), 0o644); err != nil {
			t.Fatalf("seed %s: %v", rec.key, err)
		}
	}
	tx := readonlyTx{
		db: db,
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"things": colDef},
		},
	}
	// Build a query: val > 3 AND val < 20
	q := dal.From(dal.NewRootCollectionRef("things", "")).NewQuery().
		WhereField("val", dal.GreaterThen, 3).
		WhereField("val", dal.LessThen, 20).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("things", ""), map[string]any{})
		})
	reader, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err != nil {
		t.Fatalf("executeQueryToRecordsReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	var keys []string
	for {
		rec, nextErr := reader.Next()
		if nextErr == dal.ErrNoMoreRecords {
			break
		}
		if nextErr != nil {
			t.Fatalf("Next: %v", nextErr)
		}
		keys = append(keys, rec.Key().ID.(string))
	}
	if len(keys) != 2 {
		t.Errorf("want 2 matching records, got %d: %v", len(keys), keys)
	}
}

func TestToFloat64_AllIntTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    any
		want float64
	}{
		{int(1), 1},
		{int8(2), 2},
		{int16(3), 3},
		{int32(4), 4},
		{int64(5), 5},
		{uint(6), 6},
		{uint8(7), 7},
		{uint16(8), 8},
		{uint32(9), 9},
		{uint64(10), 10},
		{float32(1.5), float64(float32(1.5))},
		{float64(2.5), 2.5},
	}
	for _, tc := range tests {
		got, ok := toFloat64(tc.v)
		if !ok {
			t.Errorf("toFloat64(%T=%v): want ok=true", tc.v, tc.v)
			continue
		}
		if got != tc.want {
			t.Errorf("toFloat64(%T=%v): got %v, want %v", tc.v, tc.v, got, tc.want)
		}
	}
	// Non-numeric types.
	_, ok := toFloat64("hello")
	if ok {
		t.Error("toFloat64(string): want ok=false")
	}
	_, ok = toFloat64(nil)
	if ok {
		t.Error("toFloat64(nil): want ok=false")
	}
}

func TestEvaluateComparison_AllOperators(t *testing.T) {
	t.Parallel()
	data := map[string]any{"v": 5}
	tests := []struct {
		op   dal.Operator
		rhs  any
		want bool
	}{
		{dal.Equal, 5, true},
		{dal.Equal, 6, false},
		{dal.GreaterThen, 4, true},
		{dal.GreaterThen, 5, false},
		{dal.GreaterOrEqual, 5, true},
		{dal.GreaterOrEqual, 6, false},
		{dal.LessThen, 6, true},
		{dal.LessThen, 5, false},
		{dal.LessOrEqual, 5, true},
		{dal.LessOrEqual, 4, false},
	}
	for _, tc := range tests {
		cmp := dal.Comparison{
			Left:     dal.NewFieldRef("v"),
			Operator: tc.op,
			Right:    dal.Constant{Value: tc.rhs},
		}
		got, err := evaluateComparison(cmp, data, "k")
		if err != nil {
			t.Errorf("op %q rhs %v: unexpected error: %v", tc.op, tc.rhs, err)
			continue
		}
		if got != tc.want {
			t.Errorf("op %q rhs %v: got %v, want %v", tc.op, tc.rhs, got, tc.want)
		}
	}
}

func TestEvaluateComparison_UnsupportedOperator(t *testing.T) {
	t.Parallel()
	data := map[string]any{"v": 1}
	cmp := dal.Comparison{
		Left:     dal.NewFieldRef("v"),
		Operator: dal.Operator("UNKNOWN"),
		Right:    dal.Constant{Value: 1},
	}
	_, err := evaluateComparison(cmp, data, "k")
	if err == nil {
		t.Fatal("want error for unsupported operator")
	}
}

func TestResolveExpression_ID(t *testing.T) {
	t.Parallel()
	ref := dal.NewFieldRef("$id")
	got, err := resolveExpression(ref, map[string]any{}, "my-key")
	if err != nil {
		t.Fatalf("resolveExpression $id: %v", err)
	}
	if got != "my-key" {
		t.Errorf("got %v, want my-key", got)
	}
}

// ---------------------------------------------------------------------------
// parse.go — EncodeMapOfRecordsContent, marshalForFormat,
//             encodeINGRFromMap, resolveINGRColumns, parseINGRAsMap
// ---------------------------------------------------------------------------

func TestEncodeMapOfRecordsContent_YAML(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"a": {"x": 1},
	}
	out, err := EncodeMapOfRecordsContent(data, ingitdb.RecordFormatYAML, "test", nil)
	if err != nil {
		t.Fatalf("EncodeMapOfRecordsContent YAML: %v", err)
	}
	if len(out) == 0 {
		t.Error("EncodeMapOfRecordsContent YAML: got empty output")
	}
}

func TestEncodeMapOfRecordsContent_JSON(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"a": {"x": 1},
	}
	out, err := EncodeMapOfRecordsContent(data, ingitdb.RecordFormatJSON, "test", nil)
	if err != nil {
		t.Fatalf("EncodeMapOfRecordsContent JSON: %v", err)
	}
	if !strings.Contains(string(out), `"a"`) {
		t.Errorf("EncodeMapOfRecordsContent JSON: output missing key 'a': %s", out)
	}
}

func TestEncodeMapOfRecordsContent_TOML(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"a": {"x": "hello"},
	}
	out, err := EncodeMapOfRecordsContent(data, ingitdb.RecordFormatTOML, "test", nil)
	if err != nil {
		t.Fatalf("EncodeMapOfRecordsContent TOML: %v", err)
	}
	if len(out) == 0 {
		t.Error("EncodeMapOfRecordsContent TOML: empty output")
	}
}

func TestMarshalForFormat_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	_, err := marshalForFormat(map[string]any{"x": 1}, ingitdb.RecordFormat("xml"))
	if err == nil {
		t.Fatal("want error for unsupported format")
	}
}

func TestEncodeMapOfRecordsContent_INGR_RoundTrip(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"alice": {"score": "99"},
		"bob":   {"score": "55"},
	}
	out, err := EncodeMapOfRecordsContent(data, ingitdb.RecordFormatINGR, "players", []string{"score"})
	if err != nil {
		t.Fatalf("EncodeMapOfRecordsContent INGR: %v", err)
	}
	// Decode back.
	parsed, err := ParseMapOfRecordsContent(out, ingitdb.RecordFormatINGR)
	if err != nil {
		t.Fatalf("ParseMapOfRecordsContent INGR: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("round-trip: got %d records, want 2", len(parsed))
	}
}

func TestResolveINGRColumns_EmptyColumnsOrder(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"a": {"z": 1, "a": 2},
	}
	cols := resolveINGRColumns(data, nil)
	if cols[0] != "$ID" {
		t.Errorf("first column: got %q, want $ID", cols[0])
	}
	// Remaining must be sorted.
	if len(cols) < 3 {
		t.Fatalf("want at least 3 columns, got %v", cols)
	}
	if cols[1] > cols[2] {
		t.Errorf("remaining columns not sorted: %v", cols[1:])
	}
}

func TestResolveINGRColumns_WithColumnsOrder(t *testing.T) {
	t.Parallel()
	data := map[string]map[string]any{
		"a": {"b": 1, "c": 2, "d": 3},
	}
	cols := resolveINGRColumns(data, []string{"c", "b"})
	if cols[0] != "$ID" {
		t.Errorf("first: want $ID, got %q", cols[0])
	}
	if cols[1] != "c" {
		t.Errorf("second: want c, got %q", cols[1])
	}
	if cols[2] != "b" {
		t.Errorf("third: want b, got %q", cols[2])
	}
}

func TestParseINGRAsMap_MissingID(t *testing.T) {
	t.Parallel()
	// Build INGR content without $ID column.
	var buf strings.Builder
	w := ingr.NewRecordsWriter(&buf)
	_, err := w.WriteHeader("test", []ingr.ColDef{{Name: "name"}})
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	_, err = w.WriteRecords(0, ingr.NewMapRecordEntry("r1", map[string]any{"name": "foo"}))
	if err != nil {
		t.Fatalf("WriteRecords: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, parseErr := parseINGRAsMap([]byte(buf.String()))
	if parseErr == nil {
		t.Fatal("want error for INGR without $ID column")
	}
}

func TestParseINGRAsMap_DuplicateID(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	w := ingr.NewRecordsWriter(&buf)
	_, err := w.WriteHeader("test", []ingr.ColDef{{Name: "$ID"}, {Name: "name"}})
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	// Both records share the same id "dup" — the first arg to NewMapRecordEntry
	// is what gets written as the $ID column value.
	r1 := ingr.NewMapRecordEntry("dup", map[string]any{"name": "A"})
	r2 := ingr.NewMapRecordEntry("dup", map[string]any{"name": "B"})
	_, err = w.WriteRecords(0, r1, r2)
	if err != nil {
		t.Fatalf("WriteRecords: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, parseErr := parseINGRAsMap([]byte(buf.String()))
	if parseErr == nil {
		t.Fatal("want error for duplicate $ID")
	}
}

func TestParseRecordContentForCollection_NilColDef(t *testing.T) {
	t.Parallel()
	_, err := ParseRecordContentForCollection([]byte("x: 1"), nil)
	if err == nil {
		t.Fatal("want error for nil colDef")
	}
}

func TestParseRecordContentForCollection_NilRecordFile(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{ID: "c", RecordFile: nil}
	_, err := ParseRecordContentForCollection([]byte("x: 1"), colDef)
	if err == nil {
		t.Fatal("want error for nil RecordFile")
	}
}

func TestEncodeRecordContentForCollection_NilColDef(t *testing.T) {
	t.Parallel()
	_, err := EncodeRecordContentForCollection(map[string]any{"x": 1}, nil)
	if err == nil {
		t.Fatal("want error for nil colDef")
	}
}

func TestEncodeRecordContentForCollection_NilRecordFile(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{ID: "c", RecordFile: nil}
	_, err := EncodeRecordContentForCollection(map[string]any{"x": 1}, colDef)
	if err == nil {
		t.Fatal("want error for nil RecordFile")
	}
}

// ---------------------------------------------------------------------------
// slice_records_reader.go — Cursor
// ---------------------------------------------------------------------------

func TestSliceRecordsReader_Cursor(t *testing.T) {
	t.Parallel()
	r := newSliceRecordsReader(nil)
	cursor, err := r.Cursor()
	if err != nil {
		t.Fatalf("Cursor: %v", err)
	}
	if cursor != "" {
		t.Errorf("Cursor: got %q, want empty string", cursor)
	}
}

func TestSliceRecordsReader_NextAndClose(t *testing.T) {
	t.Parallel()
	key := dal.NewKeyWithID("c", "1")
	rec := dal.NewRecordWithData(key, map[string]any{})
	r := newSliceRecordsReader([]dal.Record{rec})

	got, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Key().ID != "1" {
		t.Errorf("Next: ID = %v, want 1", got.Key().ID)
	}
	_, err = r.Next()
	if err == nil {
		t.Fatal("second Next: want ErrNoMoreRecords")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// schema_modifier.go — ApplyModifyField, ApplyDropIndex, additional branches
// ---------------------------------------------------------------------------

func TestAlterCollection_ModifyField_TypeAndNullability(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String, Nullable: true}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Modify: change type to Int, make required.
	op := ddl.ModifyField("label", dbschema.FieldDef{Name: "label", Type: dbschema.Int, Nullable: false})
	if err := modifier.AlterCollection(context.Background(), "tags", op); err != nil {
		t.Fatalf("AlterCollection ModifyField: %v", err)
	}
	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if len(got.Fields) != 1 {
		t.Fatalf("want 1 field, got %d", len(got.Fields))
	}
	if got.Fields[0].Type != dbschema.Int {
		t.Errorf("field type: got %v, want Int", got.Fields[0].Type)
	}
	if got.Fields[0].Nullable {
		t.Error("field Nullable: want false after ModifyField")
	}
}

func TestAlterCollection_ModifyField_RenameViaSameOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String, Nullable: true}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Seed a record with "label" key.
	recDir := filepath.Join(root, "tags", "$records")
	if err := os.MkdirAll(recDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	recPath := filepath.Join(recDir, "rust.yaml")
	if err := os.WriteFile(recPath, []byte("label: Rust\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// ModifyField with a different new name — renames the column.
	op := ddl.ModifyField("label", dbschema.FieldDef{Name: "name", Type: dbschema.String, Nullable: true})
	if err := modifier.AlterCollection(context.Background(), "tags", op); err != nil {
		t.Fatalf("AlterCollection ModifyField (rename): %v", err)
	}

	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if len(got.Fields) != 1 || got.Fields[0].Name != "name" {
		t.Errorf("after ModifyField rename: got fields %v, want [{name}]", got.Fields)
	}
	// Record file should have the key renamed.
	content, _ := os.ReadFile(recPath)
	var parsed map[string]any
	_ = yaml.Unmarshal(content, &parsed)
	if _, ok := parsed["label"]; ok {
		t.Errorf("record still has old 'label' key: %v", parsed)
	}
	if _, ok := parsed["name"]; !ok {
		t.Errorf("record missing new 'name' key: %v", parsed)
	}
}

func TestAlterCollection_ModifyField_ColumnNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	op := ddl.ModifyField("nonexistent", dbschema.FieldDef{Name: "nonexistent", Type: dbschema.String})
	if err := modifier.AlterCollection(context.Background(), "tags", op); err == nil {
		t.Fatal("want error for ModifyField on nonexistent column")
	}
}

func TestAlterCollection_ModifyField_TargetNameConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name: "tags",
		Fields: []dbschema.FieldDef{
			{Name: "label", Type: dbschema.String},
			{Name: "color", Type: dbschema.String},
		},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Try to rename "label" to "color" which already exists.
	op := ddl.ModifyField("label", dbschema.FieldDef{Name: "color", Type: dbschema.String})
	if err := modifier.AlterCollection(context.Background(), "tags", op); err == nil {
		t.Fatal("want error: target name already in use")
	}
}

func TestAlterCollection_DropIndex_IsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// DropIndex is a no-op (log only).
	if err := modifier.AlterCollection(context.Background(), "tags", ddl.DropIndex("some_idx")); err != nil {
		t.Errorf("AlterCollection DropIndex: want nil, got %v", err)
	}
}

func TestAlterCollection_AddField_ConflictAndIfNotExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Add label (already exists) without IfNotExists → error.
	opConflict := ddl.AddField(dbschema.FieldDef{Name: "label", Type: dbschema.String})
	if err := modifier.AlterCollection(context.Background(), "tags", opConflict); err == nil {
		t.Fatal("want error when adding existing column without IfNotExists")
	}

	// Add label with IfNotExists → no-op.
	opSafe := ddl.AddField(dbschema.FieldDef{Name: "label", Type: dbschema.String}, ddl.IfNotExists())
	if err := modifier.AlterCollection(context.Background(), "tags", opSafe); err != nil {
		t.Fatalf("AddField IfNotExists: got error: %v", err)
	}
	ref := dal.NewRootCollectionRef("tags", "")
	got, _ := reader.DescribeCollection(context.Background(), &ref)
	if len(got.Fields) != 1 {
		t.Errorf("after IfNotExists no-op: want 1 field, got %d", len(got.Fields))
	}
}

func TestAlterCollection_DropField_IfExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Drop nonexistent column without IfExists → error.
	opBad := ddl.DropField("nonexistent")
	if err := modifier.AlterCollection(context.Background(), "tags", opBad); err == nil {
		t.Fatal("want error: drop nonexistent column without IfExists")
	}

	// Drop nonexistent column with IfExists → no-op.
	opSafe := ddl.DropField("nonexistent", ddl.IfExists())
	if err := modifier.AlterCollection(context.Background(), "tags", opSafe); err != nil {
		t.Errorf("DropField IfExists: %v", err)
	}
}

func TestAlterCollection_RenameField_Conflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name: "tags",
		Fields: []dbschema.FieldDef{
			{Name: "label", Type: dbschema.String},
			{Name: "color", Type: dbschema.String},
		},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Rename label → color should fail (conflict).
	if err := modifier.AlterCollection(context.Background(), "tags", ddl.RenameField("label", "color")); err == nil {
		t.Fatal("want error: rename to already-existing column name")
	}
}

func TestAlterCollection_RenameField_NotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := modifier.AlterCollection(context.Background(), "tags", ddl.RenameField("nofield", "newname")); err == nil {
		t.Fatal("want error: rename nonexistent field")
	}
}

func TestAlterCollection_BadCollectionName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	// Empty name is invalid.
	err := modifier.AlterCollection(context.Background(), "")
	if err == nil {
		t.Fatal("want error for empty collection name")
	}
}

func TestAlterCollection_CollectionNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	// Collection "noexist" has no definition.yaml.
	err := modifier.AlterCollection(context.Background(), "noexist",
		ddl.AddField(dbschema.FieldDef{Name: "x", Type: dbschema.String}))
	if err == nil {
		t.Fatal("want error when collection not found")
	}
}

func TestAlterCollection_AddField_NilColumns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	// Create a minimal collection (no fields).
	if err := modifier.CreateCollection(context.Background(), dbschema.CollectionDef{Name: "empty"}); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	op := ddl.AddField(dbschema.FieldDef{Name: "score", Type: dbschema.Int, Nullable: true})
	if err := modifier.AlterCollection(context.Background(), "empty", op); err != nil {
		t.Fatalf("AlterCollection AddField: %v", err)
	}
}

func TestAlterCollection_ApplyAddIndex_IsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// AddIndex is a no-op (log only).
	if err := modifier.AlterCollection(context.Background(), "tags",
		ddl.AddIndex(dbschema.IndexDef{Name: "idx_label", Fields: []dal.FieldName{"label"}})); err != nil {
		t.Errorf("AlterCollection AddIndex: want nil, got %v", err)
	}
}

func TestReadCollectionDefYAML_ReadError(t *testing.T) {
	t.Parallel()
	// Pass a path to a non-existent file.
	_, err := readCollectionDefYAML(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestReadCollectionDefYAML_ParseError(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(p, []byte("{broken: yaml: badly"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := readCollectionDefYAML(p)
	if err == nil {
		t.Fatal("want error for invalid YAML")
	}
}

func TestWriteCollectionDefYAML_WriteError(t *testing.T) {
	t.Parallel()
	// Try to write to a path where the directory doesn't exist.
	p := filepath.Join(t.TempDir(), "nonexistent_dir", "definition.yaml")
	colDef := &ingitdb.CollectionDef{ID: "c"}
	err := writeCollectionDefYAML(p, colDef)
	if err == nil {
		t.Fatal("want error writing to nonexistent directory")
	}
}

func TestRewriteRecordFiles_NonYAMLFormat_NoOp(t *testing.T) {
	t.Parallel()
	// Non-YAML format → function returns nil without doing anything.
	err := rewriteRecordFiles(t.TempDir(), ingitdb.RecordFormatJSON, func(_ map[string]any) {})
	if err != nil {
		t.Fatalf("rewriteRecordFiles non-YAML: %v", err)
	}
}

func TestRewriteRecordFiles_MissingDir_NoOp(t *testing.T) {
	t.Parallel()
	// YAML format but directory doesn't exist → returns nil.
	nonexistent := filepath.Join(t.TempDir(), "nodir")
	err := rewriteRecordFiles(nonexistent, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err != nil {
		t.Fatalf("rewriteRecordFiles missing dir: %v", err)
	}
}

func TestRewriteRecordFiles_MutatesFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "rec.yaml")
	if err := os.WriteFile(p, []byte("a: 1\nb: 2\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := rewriteRecordFiles(dir, ingitdb.RecordFormatYAML, func(rec map[string]any) {
		delete(rec, "a")
	})
	if err != nil {
		t.Fatalf("rewriteRecordFiles: %v", err)
	}
	var got map[string]any
	content, _ := os.ReadFile(p)
	if err := yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, hasA := got["a"]; hasA {
		t.Error("field 'a' should have been deleted by rewriteRecordFiles")
	}
	if got["b"] == nil {
		t.Error("field 'b' should still be present")
	}
}

// ---------------------------------------------------------------------------
// schema_reader.go — DescribeCollection nil ref, unknown column type,
//                    ColumnsOrder referencing unknown column
// ---------------------------------------------------------------------------

func TestDescribeCollection_NilRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	_, err := reader.DescribeCollection(context.Background(), nil)
	if err == nil {
		t.Fatal("want error for nil ref")
	}
}

func TestDescribeCollection_ColumnsOrderUnknownColumn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defYAML := `record_file:
  name: "{key}.yaml"
  format: yaml
  type: single_record
columns_order: [name, ghost]
columns:
  name:
    type: string
`
	writeCollectionDef(t, root, "items", defYAML)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("items", "")
	_, err := reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("want error: columns_order references unknown column")
	}
}

func TestDescribeCollection_NoColumnsOrder_Sorted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defYAML := `record_file:
  name: "{key}.yaml"
  format: yaml
  type: single_record
columns:
  z_field:
    type: string
  a_field:
    type: int
`
	writeCollectionDef(t, root, "items", defYAML)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("items", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	// Without columns_order, fields are sorted alphabetically.
	if len(got.Fields) != 2 {
		t.Fatalf("want 2 fields, got %d", len(got.Fields))
	}
	if got.Fields[0].Name >= got.Fields[1].Name {
		t.Errorf("fields not sorted: %v", got.Fields)
	}
}

// ---------------------------------------------------------------------------
// csv.go — coerceToRowList branches, csvCellString branches
// ---------------------------------------------------------------------------

func TestCoerceToRowList_SliceMap(t *testing.T) {
	t.Parallel()
	input := []map[string]any{{"a": 1}}
	got, err := coerceToRowList(input)
	if err != nil {
		t.Fatalf("coerceToRowList []map: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len: got %d, want 1", len(got))
	}
}

func TestCoerceToRowList_MapStringMap(t *testing.T) {
	t.Parallel()
	input := map[string]map[string]any{"a": {"x": 1}}
	_, err := coerceToRowList(input)
	if err == nil {
		t.Fatal("want error for map[string]map[string]any input")
	}
}

func TestCoerceToRowList_SliceAny_Valid(t *testing.T) {
	t.Parallel()
	input := []any{map[string]any{"a": 1}, map[string]any{"b": 2}}
	got, err := coerceToRowList(input)
	if err != nil {
		t.Fatalf("coerceToRowList []any: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len: got %d, want 2", len(got))
	}
}

func TestCoerceToRowList_SliceAny_InvalidItem(t *testing.T) {
	t.Parallel()
	input := []any{map[string]any{"a": 1}, "not a map"}
	_, err := coerceToRowList(input)
	if err == nil {
		t.Fatal("want error for non-map item in []any")
	}
}

func TestCoerceToRowList_UnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := coerceToRowList("just a string")
	if err == nil {
		t.Fatal("want error for unsupported type")
	}
}

func TestCSVCellString_Nil(t *testing.T) {
	t.Parallel()
	got, err := csvCellString(nil, "f", 0)
	if err != nil {
		t.Fatalf("csvCellString nil: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestCSVCellString_String(t *testing.T) {
	t.Parallel()
	got, err := csvCellString("hello", "f", 0)
	if err != nil {
		t.Fatalf("csvCellString string: %v", err)
	}
	if got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestCSVCellString_Map(t *testing.T) {
	t.Parallel()
	_, err := csvCellString(map[string]any{"x": 1}, "f", 0)
	if err == nil {
		t.Fatal("want error for map value")
	}
}

func TestCSVCellString_Slice(t *testing.T) {
	t.Parallel()
	_, err := csvCellString([]any{1, 2}, "f", 0)
	if err == nil {
		t.Fatal("want error for slice value")
	}
}

func TestCSVCellString_DefaultFormatted(t *testing.T) {
	t.Parallel()
	got, err := csvCellString(42, "f", 0)
	if err != nil {
		t.Fatalf("csvCellString int: %v", err)
	}
	if got != "42" {
		t.Errorf("got %q, want 42", got)
	}
}

func TestValidateCSVHeader_ShortHeader(t *testing.T) {
	t.Parallel()
	err := validateCSVHeader([]string{"a"}, []string{"a", "b"})
	if err == nil {
		t.Fatal("want error for short header")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should mention 'missing', got: %v", err)
	}
}

func TestValidateCSVHeader_LongHeader(t *testing.T) {
	t.Parallel()
	err := validateCSVHeader([]string{"a", "b", "c"}, []string{"a", "b"})
	if err == nil {
		t.Fatal("want error for extra column in header")
	}
	if !strings.Contains(err.Error(), "extra") {
		t.Errorf("error should mention 'extra', got: %v", err)
	}
}

func TestValidateCSVHeader_WrongOrder(t *testing.T) {
	t.Parallel()
	err := validateCSVHeader([]string{"b", "a"}, []string{"a", "b"})
	if err == nil {
		t.Fatal("want error for mismatched column order")
	}
}

func TestParseCSVForCollection_EmptyColumnsOrder(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: nil, // empty
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	_, err := parseCSVForCollection([]byte("a,b\n1,2\n"), colDef)
	if err == nil {
		t.Fatal("want error for empty ColumnsOrder")
	}
}

func TestParseCSVForCollection_EmptyInput(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	_, err := parseCSVForCollection([]byte(""), colDef)
	if err == nil {
		t.Fatal("want error for empty input (no header)")
	}
}

func TestParseCSVForCollection_WrongRowLength(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	_, err := parseCSVForCollection([]byte("a,b\n1,2,3\n"), colDef)
	if err == nil {
		t.Fatal("want error for row with wrong column count")
	}
}

func TestEncodeCSVForCollection_EmptyColumnsOrder(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: nil,
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	_, err := encodeCSVForCollection([]map[string]any{{"a": 1}}, colDef)
	if err == nil {
		t.Fatal("want error for empty ColumnsOrder")
	}
}

func TestEncodeCSVForCollection_MissingCellValue(t *testing.T) {
	t.Parallel()
	// Row is missing a column that's in ColumnsOrder — should produce empty cell.
	colDef := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"a", "b"},
		RecordFile:   &ingitdb.RecordFileDef{Format: ingitdb.RecordFormatCSV},
	}
	rows := []map[string]any{
		{"a": "val1"}, // missing "b"
	}
	out, err := encodeCSVForCollection(rows, colDef)
	if err != nil {
		t.Fatalf("encodeCSVForCollection: %v", err)
	}
	if !strings.Contains(string(out), "val1") {
		t.Errorf("output should contain val1: %s", out)
	}
}

// ---------------------------------------------------------------------------
// batch_parsers.go — ParseBatchYAMLStream edge cases, ParseBatchINGR errors,
//                    ParseBatchCSV edge cases
// ---------------------------------------------------------------------------

func TestParseBatchYAMLStream_Empty(t *testing.T) {
	t.Parallel()
	got, err := ParseBatchYAMLStream(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseBatchYAMLStream empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 records, got %d", len(got))
	}
}

func TestParseBatchYAMLStream_SkipsEmptyDocuments(t *testing.T) {
	t.Parallel()
	input := "---\n$id: foo\nname: Foo\n---\n---\n$id: bar\nname: Bar\n"
	got, err := ParseBatchYAMLStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseBatchYAMLStream: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 records, got %d", len(got))
	}
}

func TestParseBatchYAMLStream_MissingID(t *testing.T) {
	t.Parallel()
	input := "---\nname: Foo\n"
	_, err := ParseBatchYAMLStream(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for missing $id")
	}
}

func TestParseBatchYAMLStream_InvalidKey(t *testing.T) {
	t.Parallel()
	input := "---\n$id: bad/key\nname: Foo\n"
	_, err := ParseBatchYAMLStream(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for key containing path separator")
	}
}

func TestParseBatchYAMLStream_NonStringID(t *testing.T) {
	t.Parallel()
	// $id is an integer, not a string.
	input := "---\n$id: 42\nname: Foo\n"
	_, err := ParseBatchYAMLStream(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for non-string $id in YAML stream")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error should mention 'string', got: %v", err)
	}
}

func TestParseBatchYAMLStream_EmptyStringID(t *testing.T) {
	t.Parallel()
	input := "---\n$id: \"\"\nname: Foo\n"
	_, err := ParseBatchYAMLStream(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty $id in YAML stream")
	}
}

func TestParseBatchINGR_Empty(t *testing.T) {
	t.Parallel()
	got, err := ParseBatchINGR(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseBatchINGR empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 records, got %d", len(got))
	}
}

func TestParseBatchINGR_MissingID(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	w := ingr.NewRecordsWriter(&buf)
	_, err := w.WriteHeader("test", []ingr.ColDef{{Name: "name"}})
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	_, err = w.WriteRecords(0, ingr.NewMapRecordEntry("r1", map[string]any{"name": "foo"}))
	if err != nil {
		t.Fatalf("WriteRecords: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, parseErr := ParseBatchINGR(strings.NewReader(buf.String()))
	if parseErr == nil {
		t.Fatal("want error for INGR without $ID column")
	}
}

func TestParseBatchINGR_InvalidKey(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	w := ingr.NewRecordsWriter(&buf)
	_, err := w.WriteHeader("test", []ingr.ColDef{{Name: "$ID"}, {Name: "name"}})
	if err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	// "bad/key" must be the first arg — that is what gets written as the $ID column.
	_, err = w.WriteRecords(0, ingr.NewMapRecordEntry("bad/key", map[string]any{"name": "foo"}))
	if err != nil {
		t.Fatalf("WriteRecords: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	_, parseErr := ParseBatchINGR(strings.NewReader(buf.String()))
	if parseErr == nil {
		t.Fatal("want error for invalid key in INGR")
	}
}

func TestParseBatchJSONL_NonStringID(t *testing.T) {
	t.Parallel()
	input := `{"$id": 42, "name": "Foo"}` + "\n"
	_, err := ParseBatchJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for non-string $id in JSONL")
	}
}

func TestParseBatchJSONL_EmptyID(t *testing.T) {
	t.Parallel()
	input := `{"$id": "", "name": "Foo"}` + "\n"
	_, err := ParseBatchJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatal("want error for empty $id in JSONL")
	}
}

func TestParseBatchCSV_EmptyKeyColumn(t *testing.T) {
	t.Parallel()
	input := "id,name\n,France\n"
	_, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{})
	if err == nil {
		t.Fatal("want error for empty key column value")
	}
}

func TestParseBatchCSV_WithFieldsOption(t *testing.T) {
	t.Parallel()
	// When Fields are provided, the first line is data, not a header.
	input := "france,France\njapan,Japan\n"
	got, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{
		Fields:    []string{"$id", "name"},
		KeyColumn: "$id",
	})
	if err != nil {
		t.Fatalf("ParseBatchCSV with Fields: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 records, got %d", len(got))
	}
	if got[0].Key != "france" {
		t.Errorf("record[0].Key: got %q, want france", got[0].Key)
	}
}

func TestParseBatchCSV_KeyColumnNotFound(t *testing.T) {
	t.Parallel()
	input := "name,country\nFrance,FR\n"
	_, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{KeyColumn: "$id"})
	if err == nil {
		t.Fatal("want error when key column not in header")
	}
}

func TestParseBatchCSV_NoKeyColumn(t *testing.T) {
	t.Parallel()
	// Header has neither $id nor id.
	input := "name,country\nFrance,FR\n"
	_, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{})
	if err == nil {
		t.Fatal("want error when no key column resolvable")
	}
}

func TestParseBatchCSV_InvalidKey(t *testing.T) {
	t.Parallel()
	input := "$id,name\nbad/key,France\n"
	_, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{})
	if err == nil {
		t.Fatal("want error for key with path separator")
	}
}

func TestParseBatchCSV_WrongColumnCount(t *testing.T) {
	t.Parallel()
	input := "$id,name\nfrance,France,extra\n"
	_, err := ParseBatchCSV(strings.NewReader(input), CSVParseOptions{})
	if err == nil {
		t.Fatal("want error for row with wrong column count")
	}
}

// ---------------------------------------------------------------------------
// registry.go — deregisterFromRootCollections idempotency
// ---------------------------------------------------------------------------

func TestDeregisterFromRootCollections_NoFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// No .ingitdb/root-collections.yaml file.
	if err := deregisterFromRootCollections(root, "tags"); err != nil {
		t.Fatalf("deregisterFromRootCollections with no file: %v", err)
	}
}

func TestDeregisterFromRootCollections_KeyAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	// Create one collection to ensure the file exists.
	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Deregister a name that was never registered.
	if err := deregisterFromRootCollections(root, "nope"); err != nil {
		t.Fatalf("deregisterFromRootCollections absent key: %v", err)
	}
}

// ---------------------------------------------------------------------------
// filelock.go — confirm fn errors are propagated correctly
// ---------------------------------------------------------------------------

func TestWithSharedLock_FnError(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock.dat")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := withSharedLock(p, func() error {
		return os.ErrNotExist
	})
	if err != os.ErrNotExist {
		t.Errorf("withSharedLock should propagate fn error, got: %v", err)
	}
}

func TestWithExclusiveLock_FnError(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "lock.dat")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := withExclusiveLock(p, func() error {
		return os.ErrPermission
	})
	if err != os.ErrPermission {
		t.Errorf("withExclusiveLock should propagate fn error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Targeted error-path tests added to reach ≥90% coverage.
// Each test exercises one or more uncovered branches identified via
// go tool cover -func output.
// ---------------------------------------------------------------------------

// fakeQuery implements dal.Query but NOT dal.StructuredQuery.
// It is used to exercise the "only StructuredQuery is supported" branch in
// executeQueryToRecordsReader (query.go line 25-26).
type fakeQuery struct{}

func (fakeQuery) String() string { return "fake" }
func (fakeQuery) Offset() int    { return 0 }
func (fakeQuery) Limit() int     { return 0 }
func (fakeQuery) GetRecordsReader(_ context.Context, _ dal.QueryExecutor) (dal.RecordsReader, error) {
	return nil, nil
}
func (fakeQuery) GetRecordsetReader(_ context.Context, _ dal.QueryExecutor) (dal.RecordsetReader, error) {
	return nil, nil
}

var _ dal.Query = fakeQuery{}

// TestExecuteQuery_NotStructuredQuery covers the branch where the query is not
// a dal.StructuredQuery (query.go line 25-26).
func TestExecuteQuery_NotStructuredQuery(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	_, err := executeQueryToRecordsReader(context.Background(), tx, fakeQuery{})
	if err == nil {
		t.Fatal("want error for non-StructuredQuery")
	}
}

// TestExecuteQuery_NilFrom covers the branch where sq.From() returns nil
// (query.go line 29-30). This happens when NewQueryBuilder is given a nil
// FromSource.
func TestExecuteQuery_NilFrom(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	q := dal.NewQueryBuilder(nil).SelectIntoRecord(func() dal.Record {
		return dal.NewRecordWithData(dal.NewKeyWithID("x", "k"), map[string]any{})
	})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("want error for nil FROM")
	}
}

// TestExecuteQuery_NonCollectionRef covers the branch where FROM.Base() is not
// a dal.CollectionRef (query.go line 34-35). A CollectionGroupRef satisfies
// RecordsetSource but is not a CollectionRef.
func TestExecuteQuery_NonCollectionRef(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	q := dal.From(dal.NewCollectionGroupRef("grp", "")).NewQuery().
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("grp", "k"), map[string]any{})
		})
	_, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err == nil {
		t.Fatal("want error for non-CollectionRef FROM base")
	}
}

// TestApplyOrderBy_NonFieldRefExpr covers the branch in applyOrderBy where
// the order expression wraps a non-FieldRef expression (line 188: continue).
// It also covers the cmp==0 continue (line 200) and the end-of-loop
// return false (line 208) by sorting records that are all equal.
func TestApplyOrderBy_NonFieldRefExpr(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "things", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Three records with the same val so cmp==0 for each pair.
	for _, key := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, key+".yaml"), []byte("val: 5\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	db := &Database{projectPath: root, reader: newReader()}
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: filepath.Join(root, "things"),
		Columns: map[string]*ingitdb.ColumnDef{
			"val": {Type: ingitdb.ColumnTypeInt},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	tx := readonlyTx{
		db: db,
		def: &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{"things": colDef},
		},
	}
	// Order by a constant expression (non-FieldRef) and by "val" ascending (all
	// equal → cmp==0 continue → fall through to return false at end of loop).
	q := dal.From(dal.NewRootCollectionRef("things", "")).NewQuery().
		OrderBy(dal.Ascending(dal.Constant{Value: "literal"}), dal.AscendingField("val")).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("things", ""), map[string]any{})
		})
	reader, err := executeQueryToRecordsReader(context.Background(), tx, q)
	if err != nil {
		t.Fatalf("executeQueryToRecordsReader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	var count int
	for {
		_, nextErr := reader.Next()
		if nextErr == dal.ErrNoMoreRecords {
			break
		}
		if nextErr != nil {
			t.Fatalf("Next: %v", nextErr)
		}
		count++
	}
	if count != 3 {
		t.Errorf("want 3 records, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go error-propagation paths: Set/SetMulti/Insert/InsertMulti/
// Delete/DeleteMulti/Update/UpdateMulti with unknown collection.
// ---------------------------------------------------------------------------

// makeUnknownCollectionTx returns a readwriteTx with a definition that has no
// collections, so any operation on any collection triggers resolveCollection error.
func makeUnknownCollectionTx() readwriteTx {
	return readwriteTx{readonlyTx: readonlyTx{
		db:  &Database{},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}}
}

func TestSet_UnknownCollection(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	rec := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "k"), map[string]any{})
	err := tx.Set(context.Background(), rec)
	if err == nil {
		t.Fatal("Set: want error for unknown collection")
	}
}

func TestSetMulti_PropagatesSetError(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	r1 := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "a"), map[string]any{})
	r2 := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "b"), map[string]any{})
	err := tx.SetMulti(context.Background(), []dal.Record{r1, r2})
	if err == nil {
		t.Fatal("SetMulti: want error propagated from Set")
	}
}

func TestInsert_UnknownCollection(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	rec := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "k"), map[string]any{})
	err := tx.Insert(context.Background(), rec)
	if err == nil {
		t.Fatal("Insert: want error for unknown collection")
	}
}

func TestInsertMulti_PropagatesInsertError(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	r1 := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "x"), map[string]any{})
	err := tx.InsertMulti(context.Background(), []dal.Record{r1})
	if err == nil {
		t.Fatal("InsertMulti: want error propagated from Insert")
	}
}

func TestDelete_UnknownCollection(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	err := tx.Delete(context.Background(), dal.NewKeyWithID("no_such_col", "k"))
	if err == nil {
		t.Fatal("Delete: want error for unknown collection")
	}
}

func TestDeleteMulti_PropagatesDeleteError(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	err := tx.DeleteMulti(context.Background(), []*dal.Key{dal.NewKeyWithID("no_such_col", "k")})
	if err == nil {
		t.Fatal("DeleteMulti: want error propagated from Delete")
	}
}

// TestUpdate_GetError covers tx_readwrite.go line 171-173: Get returning an
// error (e.g. resolveCollection fails for unknown collection).
func TestUpdate_GetError(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	err := tx.Update(context.Background(), dal.NewKeyWithID("no_such_col", "k"),
		[]update.Update{update.ByFieldName("x", 1)})
	if err == nil {
		t.Fatal("Update: want error from Get for unknown collection")
	}
}

// TestUpdate_RecordNotFound verifies that Update on a missing record does NOT
// return an error, because dal's record.Error() returns nil for ErrRecordNotFound
// (per the dal convention: per-record not-found is stored on the record, not
// surfaced as a method-level error). The tx_readwrite.go line 174-176 branch
// (rec.Error() != nil) is therefore unreachable via normal Get semantics.
func TestUpdate_RecordNotFound(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	// Ensure the records dir exists but the specific file does not.
	if err := os.MkdirAll(filepath.Join(root, "items", "$records"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Update on non-existent record: Get sets ErrRecordNotFound on record but
	// rec.Error() returns nil for IsNotFound errors (dal convention), so Update
	// proceeds to applyUpdates on empty data then writes a new record.
	err := tx.Update(context.Background(), dal.NewKeyWithID("items", "nonexistent"),
		[]update.Update{update.ByFieldName("name", "new")})
	// This should succeed (upsert-like behaviour via the dal not-found convention).
	if err != nil {
		t.Fatalf("Update on missing record: unexpected error: %v", err)
	}
}

// TestUpdate_ApplyUpdatesError covers tx_readwrite.go line 178-180:
// applyUpdates returning an error due to a nested field path.
func TestUpdate_ApplyUpdatesError(t *testing.T) {
	t.Parallel()
	tx, root := makeReadwriteTx(t)
	dir := filepath.Join(root, "items", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.yaml"), []byte("name: old\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A nested field path (>1 segment) is not supported and triggers an error in applyUpdates.
	err := tx.Update(context.Background(), dal.NewKeyWithID("items", "z"),
		[]update.Update{update.ByFieldPath(update.FieldPath{"nested", "key"}, "val")})
	if err == nil {
		t.Fatal("Update: want error for nested field path")
	}
}

func TestUpdateMulti_PropagatesUpdateError(t *testing.T) {
	t.Parallel()
	tx := makeUnknownCollectionTx()
	err := tx.UpdateMulti(context.Background(),
		[]*dal.Key{dal.NewKeyWithID("no_such_col", "k")},
		[]update.Update{update.ByFieldName("x", 1)})
	if err == nil {
		t.Fatal("UpdateMulti: want error propagated from Update")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go error-propagation paths.
// ---------------------------------------------------------------------------

func TestExists_UnknownCollection(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		db:  &Database{},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	_, err := tx.Exists(context.Background(), dal.NewKeyWithID("no_such_col", "k"))
	if err == nil {
		t.Fatal("Exists: want error for unknown collection")
	}
}

func TestGetMulti_PropagatesGetError(t *testing.T) {
	t.Parallel()
	tx := readonlyTx{
		db:  &Database{},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}},
	}
	rec := dal.NewRecordWithData(dal.NewKeyWithID("no_such_col", "k"), map[string]any{})
	err := tx.GetMulti(context.Background(), []dal.Record{rec})
	if err == nil {
		t.Fatal("GetMulti: want error propagated from Get")
	}
}
