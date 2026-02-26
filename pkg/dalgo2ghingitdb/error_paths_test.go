package dalgo2ghingitdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestReadwriteTx_Set_InvalidRecordData(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "test")
	record := dal.NewRecordWithData(key, "invalid-not-a-map")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error for invalid record data, got nil")
	}
	expectedMsg := "record data is not map[string]any"
	if err.Error() != expectedMsg {
		t.Errorf("Set() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestReadwriteTx_Set_MapOfIDRecords_InvalidRecordData(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/data.json",
		content: `{"existing": {"value": "test"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "data.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "new")
	record := dal.NewRecordWithData(key, "invalid-not-a-map")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error for invalid record data, got nil")
	}
	expectedMsg := "record data is not map[string]any"
	if err.Error() != expectedMsg {
		t.Errorf("Set() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestReadwriteTx_Insert_InvalidRecordData(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "test")
	record := dal.NewRecordWithData(key, "invalid-not-a-map")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error for invalid record data, got nil")
	}
	expectedMsg := "record data is not map[string]any"
	if err.Error() != expectedMsg {
		t.Errorf("Insert() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestReadwriteTx_Insert_MapOfIDRecords_InvalidRecordData(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/data.json",
		content: `{"existing": {"value": "test"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "data.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "new")
	record := dal.NewRecordWithData(key, "invalid-not-a-map")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error for invalid record data, got nil")
	}
	expectedMsg := "record data is not map[string]any"
	if err.Error() != expectedMsg {
		t.Errorf("Insert() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestReadonlyTx_Get_ReadError(t *testing.T) {
	t.Parallel()
	// Create server that returns error for file read
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
	server := newTestServer(t, handler)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "test")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("Get() expected error for read failure, got nil")
	}
}

func TestReadonlyTx_Get_ParseError_SingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/bad.yaml",
		content: "!!invalid yaml: [unclosed",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "bad")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("Get() expected error for parse failure, got nil")
	}
}

func TestReadonlyTx_Get_ParseError_MapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/bad.json",
		content: `{"key": "not-a-map-of-maps"}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test": {
				ID:      "test",
				DirPath: "test-ingitdb/test",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "bad.json",
					Format:     "json",
					RecordType: ingitdb.MapOfIDRecords,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("test", "key")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("Get() expected error for parse failure, got nil")
	}
}

func TestEncodeRecordContent_YML(t *testing.T) {
	t.Parallel()
	data := map[string]any{"title": "Test", "value": 456}
	encoded, err := encodeRecordContent(data, "yml")
	if err != nil {
		t.Fatalf("encodeRecordContent(yml): %v", err)
	}
	if len(encoded) == 0 {
		t.Error("encodeRecordContent(yml) returned empty result")
	}
}

func TestReadwriteTx_InsertMapOfIDRecords_NewFile(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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
