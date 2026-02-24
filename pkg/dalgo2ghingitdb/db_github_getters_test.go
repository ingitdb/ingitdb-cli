package dalgo2ghingitdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestGitHubDB_ID(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	id := db.ID()
	if id != DatabaseID {
		t.Errorf("ID() = %q, want %q", id, DatabaseID)
	}
}

func TestGitHubDB_Adapter(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	adapter := db.Adapter()
	if adapter == nil {
		t.Fatal("Adapter() returned nil")
	}
	if adapter.Name() != DatabaseID {
		t.Errorf("Adapter().Name() = %q, want %q", adapter.Name(), DatabaseID)
	}
	if adapter.Version() != "v0.0.1" {
		t.Errorf("Adapter().Version() = %q, want %q", adapter.Version(), "v0.0.1")
	}
}

func TestGitHubDB_Schema(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	schema := db.Schema()
	if schema != nil {
		t.Errorf("Schema() = %v, want nil", schema)
	}
}

func TestNewGitHubDB(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDB(cfg)
	if err != nil {
		t.Fatalf("NewGitHubDB: %v", err)
	}
	if db == nil {
		t.Fatal("NewGitHubDB returned nil DB")
	}
	if db.ID() != DatabaseID {
		t.Errorf("DB.ID() = %q, want %q", db.ID(), DatabaseID)
	}
}

func TestNewGitHubDB_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cfg       Config
		wantError string
	}{
		{
			name:      "missing owner",
			cfg:       Config{Repo: "test"},
			wantError: "owner is required",
		},
		{
			name:      "missing repo",
			cfg:       Config{Owner: "test"},
			wantError: "repo is required",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewGitHubDB(tc.cfg)
			if err == nil {
				t.Fatal("NewGitHubDB() expected error, got nil")
			}
			if tc.wantError != "" && err.Error() != tc.wantError {
				t.Errorf("NewGitHubDB() error = %q, want %q", err.Error(), tc.wantError)
			}
		})
	}
}

func TestNewGitHubDBWithDef_NilDefinition(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	_, err := NewGitHubDBWithDef(cfg, nil)
	if err == nil {
		t.Fatal("NewGitHubDBWithDef() expected error for nil definition, got nil")
	}
	expectedMsg := "definition is required"
	if err.Error() != expectedMsg {
		t.Errorf("NewGitHubDBWithDef() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestGitHubDB_Get(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "docs/demo-apps/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "docs/demo-apps/todo/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
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
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.Get(ctx, record)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Active" {
		t.Errorf("expected title=Active, got %v", data["title"])
	}
}

func TestGitHubDB_Exists(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	key := dal.NewKeyWithID("test", "test")
	exists, err := db.Exists(ctx, key)
	if err == nil {
		t.Fatal("Exists() expected error, got nil")
	}
	expectedMsg := fmt.Sprintf("exists is not implemented by %s", DatabaseID)
	if err.Error() != expectedMsg {
		t.Errorf("Exists() error = %q, want %q", err.Error(), expectedMsg)
	}
	if exists {
		t.Error("Exists() = true, want false")
	}
}

func TestGitHubDB_GetMulti(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	records := []dal.Record{}
	err = db.GetMulti(ctx, records)
	if err == nil {
		t.Fatal("GetMulti() expected error, got nil")
	}
	expectedMsg := fmt.Sprintf("getmulti is not implemented by %s", DatabaseID)
	if err.Error() != expectedMsg {
		t.Errorf("GetMulti() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestGitHubDB_ExecuteQueryToRecordsReader(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	reader, err := db.ExecuteQueryToRecordsReader(ctx, nil)
	if err == nil {
		t.Fatal("ExecuteQueryToRecordsReader() expected error, got nil")
	}
	expectedMsg := fmt.Sprintf("query is not implemented by %s", DatabaseID)
	if err.Error() != expectedMsg {
		t.Errorf("ExecuteQueryToRecordsReader() error = %q, want %q", err.Error(), expectedMsg)
	}
	if reader != nil {
		t.Error("ExecuteQueryToRecordsReader() reader should be nil")
	}
}

func TestGitHubDB_ExecuteQueryToRecordsetReader(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}
	ctx := context.Background()
	reader, err := db.ExecuteQueryToRecordsetReader(ctx, nil)
	if err == nil {
		t.Fatal("ExecuteQueryToRecordsetReader() expected error, got nil")
	}
	expectedMsg := fmt.Sprintf("query is not implemented by %s", DatabaseID)
	if err.Error() != expectedMsg {
		t.Errorf("ExecuteQueryToRecordsetReader() error = %q, want %q", err.Error(), expectedMsg)
	}
	if reader != nil {
		t.Error("ExecuteQueryToRecordsetReader() reader should be nil")
	}
}
