package dalgo2ghingitdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestReadwriteTx_ID(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		id := tx.ID()
		if id != "" {
			t.Errorf("ID() = %q, want empty string", id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_SetMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		records := []dal.Record{}
		setMultiErr := tx.SetMulti(ctx, records)
		if setMultiErr == nil {
			t.Fatal("SetMulti() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if setMultiErr.Error() != expectedMsg {
			t.Errorf("SetMulti() error = %q, want %q", setMultiErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_DeleteMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		keys := []*dal.Key{}
		deleteMultiErr := tx.DeleteMulti(ctx, keys)
		if deleteMultiErr == nil {
			t.Fatal("DeleteMulti() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if deleteMultiErr.Error() != expectedMsg {
			t.Errorf("DeleteMulti() error = %q, want %q", deleteMultiErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_Update(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		updateErr := tx.Update(ctx, key, nil)
		if updateErr == nil {
			t.Fatal("Update() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if updateErr.Error() != expectedMsg {
			t.Errorf("Update() error = %q, want %q", updateErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_UpdateRecord(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		record := dal.NewRecordWithData(key, map[string]any{})
		updateRecordErr := tx.UpdateRecord(ctx, record, nil)
		if updateRecordErr == nil {
			t.Fatal("UpdateRecord() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if updateRecordErr.Error() != expectedMsg {
			t.Errorf("UpdateRecord() error = %q, want %q", updateRecordErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_UpdateMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		keys := []*dal.Key{}
		updateMultiErr := tx.UpdateMulti(ctx, keys, nil)
		if updateMultiErr == nil {
			t.Fatal("UpdateMulti() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if updateMultiErr.Error() != expectedMsg {
			t.Errorf("UpdateMulti() error = %q, want %q", updateMultiErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_InsertMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		records := []dal.Record{}
		insertMultiErr := tx.InsertMulti(ctx, records)
		if insertMultiErr == nil {
			t.Fatal("InsertMulti() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("not implemented by %s", DatabaseID)
		if insertMultiErr.Error() != expectedMsg {
			t.Errorf("InsertMulti() error = %q, want %q", insertMultiErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadwriteTx_SetMapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	data := map[string]any{"title": "Updated"}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
}

func TestReadwriteTx_SetMapOfIDRecords_NewFile(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "new")
	data := map[string]any{"title": "New"}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
}

func TestReadwriteTx_InsertMapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "new")
	data := map[string]any{"title": "New"}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
}

func TestReadwriteTx_InsertMapOfIDRecords_AlreadyExists(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	data := map[string]any{"title": "Active"}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error for existing record, got nil")
	}
	expectedMsg := "record already exists: todo.tags/active"
	if err.Error() != expectedMsg {
		t.Errorf("Insert() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestReadwriteTx_DeleteMapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestReadwriteTx_DeleteMapOfIDRecords_FileNotFound(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "active")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != dal.ErrRecordNotFound {
		t.Fatalf("Delete() expected ErrRecordNotFound, got %v", err)
	}
}

func TestReadwriteTx_DeleteMapOfIDRecords_RecordNotInMap(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "nonexistent")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != dal.ErrRecordNotFound {
		t.Fatalf("Delete() expected ErrRecordNotFound, got %v", err)
	}
}

func TestEncodeRecordContent_YAML(t *testing.T) {
	t.Parallel()
	data := map[string]any{"title": "Test", "value": 123}
	encoded, err := encodeRecordContent(data, "yaml")
	if err != nil {
		t.Fatalf("encodeRecordContent(yaml): %v", err)
	}
	if len(encoded) == 0 {
		t.Error("encodeRecordContent(yaml) returned empty result")
	}
}

func TestEncodeRecordContent_JSON(t *testing.T) {
	t.Parallel()
	data := map[string]any{"title": "Test", "value": 123}
	encoded, err := encodeRecordContent(data, "json")
	if err != nil {
		t.Fatalf("encodeRecordContent(json): %v", err)
	}
	if len(encoded) == 0 {
		t.Error("encodeRecordContent(json) returned empty result")
	}
}

func TestEncodeRecordContent_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	data := map[string]any{"title": "Test"}
	_, err := encodeRecordContent(data, "xml")
	if err == nil {
		t.Fatal("encodeRecordContent() expected error for unsupported format, got nil")
	}
	expectedMsg := `unsupported record format "xml"`
	if err.Error() != expectedMsg {
		t.Errorf("encodeRecordContent() error = %q, want %q", err.Error(), expectedMsg)
	}
}
