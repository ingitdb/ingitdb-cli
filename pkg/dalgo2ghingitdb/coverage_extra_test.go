package dalgo2ghingitdb

// This file adds tests specifically to fill the coverage gaps identified in the
// following functions (all were < 100% after the initial test suite was written):
//
//   db_github.go       – NewGitHubDB, NewGitHubDBWithDef
//   file_reader.go     – readFileWithSHA (non-404 API error branch)
//   tx_readonly.go     – readRecordFromMap, resolveCollection
//   tx_readwrite.go    – Set, Insert, Delete, encodeRecordContent
//
// Branches containing "internal error: expected *githubFileReader" are marked
// unreachable: NewGitHubFileReader always returns *githubFileReader, so the
// type assertion can never fail.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// errYAMLMarshaler implements yaml.Marshaler and returns an error, allowing
// tests to trigger the yaml marshal-error branch in encodeRecordContent without
// causing yaml.v3 to panic (which it does for unsupported native types like chan).
type errYAMLMarshaler struct{}

func (e errYAMLMarshaler) MarshalYAML() (any, error) {
	return nil, fmt.Errorf("yaml marshal test error")
}

// ---------------------------------------------------------------------------
// Shared test helpers
// ---------------------------------------------------------------------------

// newReadWriteFailServer returns a test server that:
//   - GET  → returns fixture file (or 404 if path not in fixtures)
//   - PUT  → returns 403 Forbidden
//   - DELETE → returns 403 Forbidden
//
// Use this when you need readFileWithSHA to succeed but writeFile/deleteFile to fail.
func newReadWriteFailServer(t *testing.T, fixtures []githubFileFixture) *httptest.Server {
	t.Helper()
	fixtureByPath := make(map[string]githubFileFixture, len(fixtures))
	for _, f := range fixtures {
		fixtureByPath[f.path] = f
	}
	contents := make(map[string][]byte, len(fixtures))
	for _, f := range fixtures {
		if !f.isDir {
			contents[f.path] = []byte(f.content)
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathPrefix := "/repos/ingitdb/ingitdb-cli/contents/"
		if !strings.HasPrefix(r.URL.Path, pathPrefix) {
			http.NotFound(w, r)
			return
		}
		requestedPath := strings.TrimPrefix(r.URL.Path, pathPrefix)

		if r.Method == http.MethodPut || r.Method == http.MethodDelete {
			// Simulate a permission error on all write operations.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			body := `{"message":"Resource not accessible by integration","documentation_url":"https://docs.github.com"}`
			_, _ = w.Write([]byte(body))
			return
		}

		fixture, ok := fixtureByPath[requestedPath]
		if !ok {
			http.NotFound(w, r)
			return
		}
		content, hasContent := contents[fixture.path]
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
	return httptest.NewServer(handler)
}

// newAPIErrorServer returns a test server that always responds with 500 Internal
// Server Error (not 404), so the client gets a non-NotFound wrapped error.
func newAPIErrorServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal server error"}`))
	}))
}

// buildSingleRecordDef is a convenience builder for single-record YAML collections.
func buildSingleRecordDef(collectionID, dirPath, fileName string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			collectionID: {
				ID:      collectionID,
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       fileName,
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
}

// buildMapRecordDef is a convenience builder for map-of-id JSON collections.
func buildMapRecordDef(collectionID, dirPath, fileName string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			collectionID: {
				ID:      collectionID,
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       fileName,
					Format:     "json",
					RecordType: ingitdb.MapOfRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// db_github.go – NewGitHubDBWithDef: error from NewGitHubFileReader
// ---------------------------------------------------------------------------

func TestNewGitHubDBWithDef_InvalidConfig(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	_, err := NewGitHubDBWithDef(Config{}, def) // empty Owner/Repo causes validate() to fail
	if err == nil {
		t.Fatal("NewGitHubDBWithDef() expected error for invalid config, got nil")
	}
}

// ---------------------------------------------------------------------------
// file_reader.go – readFileWithSHA: non-404 API error branch
// ---------------------------------------------------------------------------

func TestReadFileWithSHA_NonNotFoundAPIError(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}
	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	_, _, _, err = concrete.readFileWithSHA(ctx, "some/path.yaml")
	if err == nil {
		t.Fatal("readFileWithSHA() expected error for 500 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go – readRecordFromMap: ReadFile error branch
// ---------------------------------------------------------------------------

func TestReadRecordFromMap_ReadFileError(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("tags", "active")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err == nil {
		t.Fatal("Get() expected error when ReadFile returns API error, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go – readRecordFromMap: file not found → record not found
// ---------------------------------------------------------------------------

func TestReadRecordFromMap_FileNotFound(t *testing.T) {
	t.Parallel()
	// server with no fixtures → every GET returns 404
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	key := dal.NewKeyWithID("tags", "active")
	record := dal.NewRecordWithData(key, map[string]any{})
	ctx := context.Background()
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if record.Exists() {
		t.Fatal("Get() expected record to not exist when map-file not found")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go – resolveCollection: nil definition via readwrite tx
// ---------------------------------------------------------------------------

func TestResolveCollection_NilDef_ViaSet(t *testing.T) {
	t.Parallel()
	// NewGitHubDB creates a db with nil def; resolveCollection checks this first.
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDB(cfg)
	if err != nil {
		t.Fatalf("NewGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error for nil definition, got nil")
	}
	if err.Error() != "definition is required" {
		t.Errorf("Set() error = %q, want %q", err.Error(), "definition is required")
	}
}

func TestResolveCollection_NilDef_ViaInsert(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDB(cfg)
	if err != nil {
		t.Fatalf("NewGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error for nil definition, got nil")
	}
	if err.Error() != "definition is required" {
		t.Errorf("Insert() error = %q, want %q", err.Error(), "definition is required")
	}
}

// ---------------------------------------------------------------------------
// tx_readonly.go – resolveCollection: collection not found via readwrite tx
// ---------------------------------------------------------------------------

func TestResolveCollection_CollectionNotFound_ViaSet(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}} // empty
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("unknown", "active")
		record := dal.NewRecordWithData(key, map[string]any{})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error for unknown collection, got nil")
	}
	if !strings.Contains(err.Error(), "not found in definition") {
		t.Errorf("Set() error = %q, want to contain 'not found in definition'", err.Error())
	}
}

func TestResolveCollection_CollectionNotFound_ViaInsert(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	cfg := Config{Owner: "test", Repo: "test"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("unknown", "active")
		record := dal.NewRecordWithData(key, map[string]any{})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error for unknown collection, got nil")
	}
	if !strings.Contains(err.Error(), "not found in definition") {
		t.Errorf("Insert() error = %q, want to contain 'not found in definition'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Set: readFileWithSHA API error branches
// ---------------------------------------------------------------------------

func TestSet_ReadFileWithSHA_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Active"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error when readFileWithSHA fails, got nil")
	}
}

func TestSet_ReadFileWithSHA_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Active"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error when readFileWithSHA fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Set: parseErr when existing map-file is malformed
// ---------------------------------------------------------------------------

func TestSet_ParseError_MapOfRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": "not-a-map"}`, // valid JSON but wrong shape → parse error
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected parse error for malformed map file, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Set: writeFile error branches
// ---------------------------------------------------------------------------

func TestSet_WriteFile_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newReadWriteFailServer(t, fixtures)
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Updated"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error when writeFile fails, got nil")
	}
}

func TestSet_WriteFile_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": {"title": "Active"}}`,
	}}
	server := newReadWriteFailServer(t, fixtures)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Updated"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected error when writeFile fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Set: encodeRecordContent error branches
// ---------------------------------------------------------------------------

func TestSet_EncodeError_SingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/active.xml",
		content: "<title>Active</title>",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	// Use xml format (unsupported by encodeRecordContent) → triggers encodeErr path in Set default.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.xml",
					Format:     "xml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Active"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected encode error for unsupported format, got nil")
	}
}

func TestSet_EncodeError_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil) // 404 → new file
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		ch := make(chan int)
		record := dal.NewRecordWithData(key, map[string]any{"ch": ch})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected encode error for un-marshallable map data, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Insert: readFileWithSHA API error branches
// ---------------------------------------------------------------------------

func TestInsert_ReadFileWithSHA_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error when readFileWithSHA fails, got nil")
	}
}

func TestInsert_ReadFileWithSHA_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error when readFileWithSHA fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Insert: parseErr when existing map-file is malformed
// ---------------------------------------------------------------------------

func TestInsert_ParseError_MapOfRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": "not-a-map"}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected parse error for malformed map file, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Insert: writeFile error branches
// ---------------------------------------------------------------------------

func TestInsert_WriteFile_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newReadWriteFailServer(t, nil) // GET → 404 (not found) then PUT → 403
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error when writeFile fails, got nil")
	}
}

func TestInsert_WriteFile_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newReadWriteFailServer(t, nil) // GET → 404 (new file) then PUT → 403
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected error when writeFile fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Insert: encodeRecordContent error branches
// ---------------------------------------------------------------------------

func TestInsert_EncodeError_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil) // GET → 404 (not found)
	defer server.Close()

	// Use xml format (unsupported) → triggers encodeErr path in Insert default.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.xml",
					Format:     "xml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected encode error for unsupported format, got nil")
	}
}

func TestInsert_EncodeError_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil) // GET → 404 (new file)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		ch := make(chan int)
		record := dal.NewRecordWithData(key, map[string]any{"ch": ch})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected encode error for un-marshallable map data, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: readFileWithSHA API error branches
// ---------------------------------------------------------------------------

func TestDelete_ReadFileWithSHA_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected error when readFileWithSHA fails, got nil")
	}
}

func TestDelete_ReadFileWithSHA_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	server := newAPIErrorServer(t)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected error when readFileWithSHA fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: parseErr when map-file is malformed
// ---------------------------------------------------------------------------

func TestDelete_ParseError_MapOfRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": "not-a-map"}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected parse error for malformed map file, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: for-range loop body in map path (need 2+ entries)
// The existing test deletes the only entry, so the post-deletion loop over
// remaining entries is never executed. This test deletes one of two entries.
// ---------------------------------------------------------------------------

func TestDelete_MapOfRecords_LoopBodyCovered(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": {"title": "Active"}, "archived": {"title": "Archived"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	// Delete "active" while "archived" remains → the for-range body executes for "archived".
	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: encodeErr after removing entry from map
// ---------------------------------------------------------------------------

func TestDelete_EncodeError_MapOfRecords(t *testing.T) {
	t.Parallel()
	// File has two entries. The one that remains after deletion has un-serialisable data.
	// We store the raw JSON to fake it: we need the server to return something that parses
	// as map[string]map[string]any but contains a value that can't be re-encoded.
	// Since we can only inject chan/func at the Go level (not via JSON), we use a workaround:
	// store valid JSON and rely on encodeRecordContent being called with a collection that
	// uses an unsupported format.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.json",
					Format:     "json",
					RecordType: ingitdb.MapOfRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
	// We'll patch the format after creating the def to be unsupported so that
	// encodeRecordContent returns an error.
	def.Collections["tags"].RecordFile.Format = "xml" // unsupported → encodeErr

	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": {"title": "Active"}, "archived": {"title": "Archived"}}`,
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected encode error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported record format") {
		t.Errorf("Delete() error = %q, want 'unsupported record format'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: writeFile error after removing entry from map
// ---------------------------------------------------------------------------

func TestDelete_WriteFile_Error_MapOfRecords(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/tags.json",
		content: `{"active": {"title": "Active"}, "archived": {"title": "Archived"}}`,
	}}
	server := newReadWriteFailServer(t, fixtures) // GET succeeds, PUT fails
	defer server.Close()

	def := buildMapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected error when writeFile fails for map record, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Delete: deleteFile error in single-record path
// ---------------------------------------------------------------------------

func TestDelete_DeleteFile_Error_SingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/active.yaml",
		content: "title: Active\n",
	}}
	server := newReadWriteFailServer(t, fixtures) // GET succeeds, DELETE fails
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		return tx.Delete(ctx, key)
	})
	if err == nil {
		t.Fatal("Delete() expected error when deleteFile fails, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – encodeRecordContent: yaml and json marshal error paths
// ---------------------------------------------------------------------------

func TestEncodeRecordContent_YAMLMarshalError(t *testing.T) {
	t.Parallel()
	// errYAMLMarshaler.MarshalYAML() returns an error, which yaml.v3 propagates as
	// an error return (rather than panicking, which it does for unsupported native types).
	data := map[string]any{"key": errYAMLMarshaler{}}
	_, err := encodeRecordContent(data, "yaml")
	if err == nil {
		t.Fatal("encodeRecordContent(yaml) expected error for failing marshaler, got nil")
	}
	if !strings.Contains(err.Error(), "failed to encode YAML record") {
		t.Errorf("encodeRecordContent(yaml) error = %q, want 'failed to encode YAML record'", err.Error())
	}
}

func TestEncodeRecordContent_JSONMarshalError(t *testing.T) {
	t.Parallel()
	// json.MarshalIndent returns an error for channel values.
	ch := make(chan int)
	data := map[string]any{"invalid": ch}
	_, err := encodeRecordContent(data, "json")
	if err == nil {
		t.Fatal("encodeRecordContent(json) expected error for channel value, got nil")
	}
	if !strings.Contains(err.Error(), "failed to encode JSON record") {
		t.Errorf("encodeRecordContent(json) error = %q, want 'failed to encode JSON record'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go – Set: encodeErr path via unsupported format
// (supplements the channel-based tests above with an alternative trigger)
// ---------------------------------------------------------------------------

func TestSet_EncodeError_UnsupportedFormat_SingleRecord(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "data/tags/active.xml",
		content: "<title>Active</title>",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.xml",
					Format:     "xml", // unsupported → encodeErr
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "active")
		record := dal.NewRecordWithData(key, map[string]any{"title": "Active"})
		return tx.Set(ctx, record)
	})
	if err == nil {
		t.Fatal("Set() expected encode error for unsupported format, got nil")
	}
}

func TestInsert_EncodeError_UnsupportedFormat_SingleRecord(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil) // GET → 404
	defer server.Close()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.xml",
					Format:     "xml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "new")
		record := dal.NewRecordWithData(key, map[string]any{"title": "New"})
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("Insert() expected encode error for unsupported format, got nil")
	}
}
