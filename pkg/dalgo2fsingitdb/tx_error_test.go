package dalgo2fsingitdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestGet_NilDefinition(t *testing.T) {
	t.Parallel()

	// Create DB without definition
	db, err := NewLocalDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDB: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for nil definition, got nil")
	}
	expected := "definition is required: use NewLocalDBWithDef"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestGet_CollectionNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
	db, err := NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("nonexistent.collection", "abc")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for nonexistent collection, got nil")
	}
	expected := `collection "nonexistent.collection" not found in definition`
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestGet_UnsupportedRecordType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: "unsupported_type", // Invalid record type
				},
			},
		},
	}
	db, err := NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for unsupported record type, got nil")
	}
	expected := `not yet implemented for record type "unsupported_type"`
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestResolveCollection_NilDefinition(t *testing.T) {
	t.Parallel()

	tx := readwriteTx{
		readonlyTx: readonlyTx{
			db: localDB{
				rootDirPath: t.TempDir(),
				def:         nil,
			},
		},
	}

	key := dal.NewKeyWithID("test.items", "abc")
	_, _, err := tx.resolveCollection(key)
	if err == nil {
		t.Fatal("expected error for nil definition, got nil")
	}
	expected := "definition is required: use NewLocalDBWithDef"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestResolveCollection_CollectionNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
		},
	}
	tx := readwriteTx{
		readonlyTx: readonlyTx{
			db: localDB{
				rootDirPath: dir,
				def:         def,
			},
		},
	}

	key := dal.NewKeyWithID("nonexistent.collection", "abc")
	_, _, err := tx.resolveCollection(key)
	if err == nil {
		t.Fatal("expected error for nonexistent collection, got nil")
	}
	expected := `collection "nonexistent.collection" not found in definition`
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestResolveCollection_NilRecordFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:         "test.items",
				DirPath:    dir,
				RecordFile: nil, // Missing RecordFile definition
			},
		},
	}
	tx := readwriteTx{
		readonlyTx: readonlyTx{
			db: localDB{
				rootDirPath: dir,
				def:         def,
			},
		},
	}

	key := dal.NewKeyWithID("test.items", "abc")
	_, _, err := tx.resolveCollection(key)
	if err == nil {
		t.Fatal("expected error for nil RecordFile, got nil")
	}
	expected := `collection "test.items" has no record_file definition`
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestSet_WithResolveError(t *testing.T) {
	t.Parallel()

	// Use DB without definition to trigger resolve error
	db, err := NewLocalDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDB: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{"name": "Test"}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("expected resolve error, got nil")
	}
	expected := "definition is required: use NewLocalDBWithDef"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestDelete_WithResolveError(t *testing.T) {
	t.Parallel()

	// Use DB without definition to trigger resolve error
	db, err := NewLocalDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDB: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("expected resolve error, got nil")
	}
	expected := "definition is required: use NewLocalDBWithDef"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestInsert_WithResolveError(t *testing.T) {
	t.Parallel()

	// Use DB without definition to trigger resolve error
	db, err := NewLocalDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDB: %v", err)
	}

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{"name": "Test"}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected resolve error, got nil")
	}
	expected := "definition is required: use NewLocalDBWithDef"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestInsert_SingleRecord_StatError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)

	// Create a directory where we want to create a file (naming collision).
	// The template is {key}.yaml with $records/ prefix, so if key is "blocking",
	// the file will be at $records/blocking.yaml.
	recordsDir := filepath.Join(dir, "$records")
	err := os.Mkdir(recordsDir, 0o755)
	if err != nil {
		t.Fatalf("setup: mkdir $records: %v", err)
	}
	blockingDir := filepath.Join(recordsDir, "blocking.yaml")
	err = os.Mkdir(blockingDir, 0o755)
	if err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "blocking") // key "blocking" -> file "blocking.yaml"
	data := map[string]any{"name": "Test"}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error (file/directory already exists), got nil")
	}
	expected := "record already exists:"
	if len(err.Error()) < len(expected) || err.Error()[:len(expected)] != expected {
		t.Errorf("error should start with %q, got: %v", expected, err)
	}
}

func TestSet_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)

	// Create an invalid YAML file (JSON parser will fail)
	path := filepath.Join(dir, "tags.json")
	err := os.WriteFile(path, []byte("invalid json content"), 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{"title": "Work"}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("expected read or parse error, got nil")
	}
}

func TestSet_MapOfRecords_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{"title": "Work"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set on non-existent file: %v", err)
	}

	// Verify the file was created
	path := filepath.Join(dir, "tags.json")
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected file to be created: %v", statErr)
	}
}

func TestGet_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)

	// Create an invalid YAML file under $records/ (where {key}.yaml records are stored).
	recordsDir := filepath.Join(dir, "$records")
	if mkErr := os.MkdirAll(recordsDir, 0o755); mkErr != nil {
		t.Fatalf("setup: MkdirAll: %v", mkErr)
	}
	path := filepath.Join(recordsDir, "bad.yaml")
	err := os.WriteFile(path, []byte("key: [unclosed"), 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "bad")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
	if err.Error()[:25] != "failed to parse YAML file" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGet_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)

	// Create an invalid JSON file
	path := filepath.Join(dir, "tags.json")
	err := os.WriteFile(path, []byte("{invalid json}"), 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected JSON parse or validation error, got nil")
	}
}

func TestGet_MapOfRecords_InvalidRecordInFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)

	// Create a JSON file with invalid record structure (non-map value)
	path := filepath.Join(dir, "tags.json")
	content := []byte(`{"work": "not_a_map", "home": {"title": "Home"}}`)
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for invalid record structure, got nil")
	}
}

func TestInsert_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)

	// Create an invalid JSON file
	path := filepath.Join(dir, "tags.json")
	err := os.WriteFile(path, []byte("{invalid json}"), 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{"title": "Work"}
	record := dal.NewRecordWithData(key, data)

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected JSON parse or validation error, got nil")
	}
}

func TestDelete_MapOfRecords_ReadError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)

	// Create an invalid JSON file
	path := filepath.Join(dir, "tags.json")
	err := os.WriteFile(path, []byte("{invalid json}"), 0o644)
	if err != nil {
		t.Fatalf("setup: write invalid file: %v", err)
	}

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("expected JSON parse or validation error, got nil")
	}
}

func TestResolveRecordPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		template  string
		recordKey string
		want      string
	}{
		{
			name:      "simple template",
			template:  "{key}.yaml",
			recordKey: "abc",
			want:      "$records/abc.yaml",
		},
		{
			name:      "subdirectory template",
			template:  "{key}/{key}.yaml",
			recordKey: "de",
			want:      "$records/de/de.yaml",
		},
		{
			name:      "multiple key occurrences",
			template:  "{key}/data/{key}.json",
			recordKey: "test",
			want:      "$records/test/data/test.json",
		},
		{
			name:      "no key placeholder",
			template:  "fixed.yaml",
			recordKey: "xyz",
			want:      "fixed.yaml",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			colDef := &ingitdb.CollectionDef{
				DirPath: "/test/dir",
				RecordFile: &ingitdb.RecordFileDef{
					Name: tc.template,
				},
			}

			got := resolveRecordPath(colDef, tc.recordKey)
			want := filepath.Join("/test/dir", tc.want)
			if got != want {
				t.Errorf("resolveRecordPath() = %q, want %q", got, want)
			}
		})
	}
}

func TestDeleteRecordFile_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	err := os.WriteFile(path, []byte("content"), 0o644)
	if err != nil {
		t.Fatalf("setup: write file: %v", err)
	}

	err = deleteRecordFile(path)
	if err != nil {
		t.Fatalf("deleteRecordFile: %v", err)
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("expected file to be deleted")
	}
}

func TestWriteMapOfRecordsFile_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := map[string]map[string]any{
		"id1": {"field": "value1"},
		"id2": {"field": "value2"},
	}

	err := writeMapOfRecordsFile(path, "yaml", data)
	if err != nil {
		t.Fatalf("writeMapOfRecordsFile: %v", err)
	}

	// Read back and verify
	var result map[string]any
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if err = yaml.Unmarshal(content, &result); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 records, got %d", len(result))
	}
}

// TestInsert_SingleRecord_StatPermissionError covers the
// `return fmt.Errorf("failed to check file %s: %w", path, statErr)` branch
// in Insert (default/SingleRecord path).
// When the parent directory is non-executable, os.Stat of a file inside it
// returns EACCES — an error that is not os.ErrNotExist — triggering this branch.
func TestInsert_SingleRecord_StatPermissionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Use a subdirectory as the collection root so we can restrict permissions
	// without affecting t.TempDir() cleanup.
	collDir := filepath.Join(dir, "coll")
	mkdirErr := os.Mkdir(collDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir collDir: %v", mkdirErr)
	}

	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: collDir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.items": colDef},
	}

	// Remove all permissions from the collection directory so that
	// os.Stat of any file inside returns EACCES (not ErrNotExist).
	chmodErr := os.Chmod(collDir, 0o000)
	if chmodErr != nil {
		t.Fatalf("setup: chmod collDir: %v", chmodErr)
	}
	// Restore permissions so t.TempDir() cleanup can delete the directory.
	t.Cleanup(func() { _ = os.Chmod(collDir, 0o755) })

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "test")
	data := map[string]any{"name": "Test"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected permission error from os.Stat, got nil")
	}
	const prefix = "failed to check file"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}
