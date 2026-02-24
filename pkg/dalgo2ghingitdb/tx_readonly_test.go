package dalgo2ghingitdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestReadonlyTx_Options(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		opts := tx.Options()
		if opts != nil {
			t.Errorf("Options() = %v, want nil", opts)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_Exists(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		exists, existsErr := tx.Exists(ctx, key)
		if existsErr == nil {
			t.Fatal("Exists() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("exists is not implemented by %s", DatabaseID)
		if existsErr.Error() != expectedMsg {
			t.Errorf("Exists() error = %q, want %q", existsErr.Error(), expectedMsg)
		}
		if exists {
			t.Error("Exists() = true, want false")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_GetMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		records := []dal.Record{}
		getMultiErr := tx.GetMulti(ctx, records)
		if getMultiErr == nil {
			t.Fatal("GetMulti() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("getmulti is not implemented by %s", DatabaseID)
		if getMultiErr.Error() != expectedMsg {
			t.Errorf("GetMulti() error = %q, want %q", getMultiErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_ExecuteQueryToRecordsReader(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, queryErr := tx.ExecuteQueryToRecordsReader(ctx, nil)
		if queryErr == nil {
			t.Fatal("ExecuteQueryToRecordsReader() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("query is not implemented by %s", DatabaseID)
		if queryErr.Error() != expectedMsg {
			t.Errorf("ExecuteQueryToRecordsReader() error = %q, want %q", queryErr.Error(), expectedMsg)
		}
		if reader != nil {
			t.Error("ExecuteQueryToRecordsReader() reader should be nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_ExecuteQueryToRecordsetReader(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, queryErr := tx.ExecuteQueryToRecordsetReader(ctx, nil)
		if queryErr == nil {
			t.Fatal("ExecuteQueryToRecordsetReader() expected error, got nil")
		}
		expectedMsg := fmt.Sprintf("query is not implemented by %s", DatabaseID)
		if queryErr.Error() != expectedMsg {
			t.Errorf("ExecuteQueryToRecordsetReader() error = %q, want %q", queryErr.Error(), expectedMsg)
		}
		if reader != nil {
			t.Error("ExecuteQueryToRecordsetReader() reader should be nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_Get_NoDefinition(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDB(cfg)
	if err != nil {
		t.Fatalf("NewGitHubDB: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		record := dal.NewRecordWithData(key, map[string]any{})
		getErr := tx.Get(ctx, record)
		if getErr == nil {
			t.Fatal("Get() expected error for missing definition, got nil")
		}
		expectedMsg := "definition is required"
		if getErr.Error() != expectedMsg {
			t.Errorf("Get() error = %q, want %q", getErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_Get_CollectionNotFound(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		key := dal.NewKeyWithID("nonexistent", "test")
		record := dal.NewRecordWithData(key, map[string]any{})
		getErr := tx.Get(ctx, record)
		if getErr == nil {
			t.Fatal("Get() expected error for missing collection, got nil")
		}
		expectedMsg := `collection "nonexistent" not found in definition`
		if getErr.Error() != expectedMsg {
			t.Errorf("Get() error = %q, want %q", getErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_Get_UnsupportedRecordType(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/data.yaml",
		content: "value: test\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "data.yaml",
					Format:     "yaml",
					RecordType: "unsupported",
				},
			},
		},
	}
	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		record := dal.NewRecordWithData(key, map[string]any{})
		getErr := tx.Get(ctx, record)
		if getErr == nil {
			t.Fatal("Get() expected error for unsupported record type, got nil")
		}
		expectedMsg := `record type "unsupported" is not supported`
		if getErr.Error() != expectedMsg {
			t.Errorf("Get() error = %q, want %q", getErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestReadonlyTx_ResolveCollection_NoRecordFile(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:         "test",
				DirPath:    "test-ingitdb/test",
				RecordFile: nil,
			},
		},
	}
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("test", "test")
		deleteErr := tx.Delete(ctx, key)
		if deleteErr == nil {
			t.Fatal("Delete() expected error for missing record file, got nil")
		}
		expectedMsg := `collection "test" has no record file`
		if deleteErr.Error() != expectedMsg {
			t.Errorf("Delete() error = %q, want %q", deleteErr.Error(), expectedMsg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

func TestReadonlyTx_GetMapOfIDRecords_RecordNotInMap(t *testing.T) {
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
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist when not in map")
	}
}
