package dalgo2ghingitdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type githubFileFixture struct {
	path     string
	content  string
	isDir    bool
	dirItems []string
}

func TestGitHubDB_GetSingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "demo-dbs/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Active" {
		t.Fatalf("expected title=Active, got %v", data["title"])
	}
}

func TestGitHubDB_GetMapOfIDRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "demo-dbs/todo/tags/tags.json",
		content: `{"active": {"titles": {"en": "Active", "ru": "Активно"}}}`,
	}}
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
					"title": {Type: ingitdb.ColumnTypeString, Locale: "en"},
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
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Active" {
		t.Fatalf("expected title=Active, got %v", data["title"])
	}
}

func TestGitHubDB_GetNotFound(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("todo.tags", "missing")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist")
	}
}

func TestGitHubDB_SetSingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "demo-dbs/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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

func TestGitHubDB_InsertSingleRecord(t *testing.T) {
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

func TestGitHubDB_InsertSingleRecord_AlreadyExists(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "demo-dbs/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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
	data := map[string]any{"title": "Active"}
	record := dal.NewRecordWithData(key, data)
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for existing record")
	}
}

func TestGitHubDB_DeleteSingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "demo-dbs/todo/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags": {
				ID:      "todo.tags",
				DirPath: "demo-dbs/todo/tags",
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
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestGitHubDB_DeleteSingleRecord_NotFound(t *testing.T) {
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

	key := dal.NewKeyWithID("todo.tags", "missing")
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != dal.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestGitHubDB_ListDirectory(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:     "demo-dbs/todo/tags",
		isDir:    true,
		dirItems: []string{"active.yaml", "archived.yaml"},
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	entries, err := reader.ListDirectory(ctx, "demo-dbs/todo/tags")
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0] != "active.yaml" || entries[1] != "archived.yaml" {
		t.Fatalf("unexpected entries: %v", entries)
	}
}

func newGitHubContentsServer(t *testing.T, fixtures []githubFileFixture) *httptest.Server {
	t.Helper()
	fixtureByPath := make(map[string]githubFileFixture, len(fixtures))
	for _, fixture := range fixtures {
		fixtureByPath[fixture.path] = fixture
	}
	contents := make(map[string][]byte)
	for _, fixture := range fixtures {
		if !fixture.isDir {
			contents[fixture.path] = []byte(fixture.content)
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathPrefix := "/repos/ingitdb/ingitdb-cli/contents/"
		if !strings.HasPrefix(r.URL.Path, pathPrefix) {
			http.NotFound(w, r)
			return
		}
		requestedPath := strings.TrimPrefix(r.URL.Path, pathPrefix)

		if r.Method == http.MethodPut {
			var reqBody map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&reqBody)
			if decodeErr != nil {
				http.Error(w, decodeErr.Error(), http.StatusBadRequest)
				return
			}
			contentB64, ok := reqBody["content"].(string)
			if !ok {
				http.Error(w, "missing content", http.StatusBadRequest)
				return
			}
			decodedContent, decodeB64Err := base64.StdEncoding.DecodeString(contentB64)
			if decodeB64Err != nil {
				http.Error(w, decodeB64Err.Error(), http.StatusBadRequest)
				return
			}
			contents[requestedPath] = decodedContent
			response := map[string]any{
				"content": map[string]any{
					"name": path.Base(requestedPath),
					"path": requestedPath,
					"sha":  "abc123def456",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			encodeErr := json.NewEncoder(w).Encode(response)
			if encodeErr != nil {
				http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		if r.Method == http.MethodDelete {
			delete(contents, requestedPath)
			w.Header().Set("Content-Type", "application/json")
			deleteResp := map[string]any{"content": nil}
			encodeErr := json.NewEncoder(w).Encode(deleteResp)
			if encodeErr != nil {
				http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		fixture, ok := fixtureByPath[requestedPath]
		if !ok {
			http.NotFound(w, r)
			return
		}

		if fixture.isDir {
			dirResponse := make([]map[string]any, 0, len(fixture.dirItems))
			for _, item := range fixture.dirItems {
				dirResponse = append(dirResponse, map[string]any{
					"name": item,
					"path": path.Join(requestedPath, item),
					"type": "file",
				})
			}
			w.Header().Set("Content-Type", "application/json")
			encodeErr := json.NewEncoder(w).Encode(dirResponse)
			if encodeErr != nil {
				http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
			}
			return
		}

		content, hasContent := contents[requestedPath]
		if !hasContent {
			http.NotFound(w, r)
			return
		}
		encoded := base64.StdEncoding.EncodeToString(content)
		response := map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  encoded,
			"sha":      "abc123def456",
			"name":     path.Base(requestedPath),
			"path":     requestedPath,
		}
		w.Header().Set("Content-Type", "application/json")
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	return server
}
