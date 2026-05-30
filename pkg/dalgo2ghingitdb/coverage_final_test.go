package dalgo2ghingitdb

// coverage_final_test.go fills the remaining coverage gaps identified after
// the previous test-writing rounds. Each section is labelled with the file and
// line range it targets.
//
// Untestable branches (production code already carries "// untestable" comments):
//
//   batching.go:50-52   – !ok guard after inner.(*githubDB) — NewGitHubDBWithDef always returns *githubDB
//   db_github.go:25-27  – !ok guard after reader.(*githubFileReader) in NewGitHubDB
//   db_github.go:44-46  – !ok guard after reader.(*githubFileReader) in NewGitHubDBWithDef
//   tx_readwrite.go:182-185 – encodeErr after deleting from MapOfRecords map —
//                             ParseMapOfRecordsContent already rejects unsupported formats,
//                             and for supported formats (json/yaml) the parsed data can always be re-encoded.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// batching.go:54-56 — NewBatchingGitHubDB: NewTreeWriter fails (invalid APIBaseURL)
// ---------------------------------------------------------------------------

// TestNewBatchingGitHubDB_TreeWriterError triggers the NewTreeWriter error path
// inside newBatchingGitHubDB by injecting a factory that always returns an error.
// NewGitHubDBWithDef must succeed first (valid config), then the injected factory
// fails, exercising the `if err != nil { return nil, err }` guard after NewTreeWriter.
func TestNewBatchingGitHubDB_TreeWriterError(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	cfg := Config{Owner: "o", Repo: "r"}
	failFactory := func(Config) (*TreeWriter, error) {
		return nil, fmt.Errorf("injected tree writer error")
	}
	_, err := newBatchingGitHubDB(cfg, def, "msg", failFactory)
	if err == nil {
		t.Fatal("newBatchingGitHubDB: expected error from tree writer factory, got nil")
	}
	if err.Error() != "injected tree writer error" {
		t.Errorf("error = %q, want 'injected tree writer error'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// batching.go:81-83 — RunReadwriteTransaction: flushChanges error propagation
//
// The easiest way to make flushChanges fail without modifying production code
// is to create a batchingTx where workingMaps contains a MapOfRecords with
// an unsupported format. We exercise this via a direct batchingTx, using
// the same approach already used in TestBatchingTx_FlushChanges_EncodeError
// but exercising the RunReadwriteTransaction wrapper by setting up the tx
// state inside the worker via Set/Delete (which populates workingMaps).
//
// The simplest end-to-end approach: use a BatchingGitHubDB whose collection
// uses XML format (unsupported) so that when the worker calls Set for a
// MapOfRecords entry, ensureMapLoaded is called (returns the empty map from
// a 404), the entry is stored in workingMaps, and then flushChanges fails to
// encode it.
// ---------------------------------------------------------------------------

// TestBatchingGitHubDB_RunRWTx_FlushChangesError verifies that an error from
// flushChanges is propagated to the RunReadwriteTransaction caller.
func TestBatchingGitHubDB_RunRWTx_FlushChangesError(t *testing.T) {
	t.Parallel()

	// Server returns 404 for GET /contents/... so ensureMapLoaded gets
	// not-found and initialises an empty working map (no parse needed).
	// Git Data API endpoints must not be called because flushChanges should
	// fail before CommitChanges is invoked.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/contents/") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "unexpected request: "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
	}))
	defer srv.Close()

	// XML is unsupported by EncodeMapOfRecordsContent — flushing will fail.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"tags": {
				ID:      "tags",
				DirPath: "data/tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.xml",
					Format:     "xml",
					RecordType: ingitdb.MapOfRecords,
				},
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		// Set on a MapOfRecords collection: ensureMapLoaded fires (empty map from
		// 404), then the entry is stored in workingMaps.  The worker returns nil
		// so flushChanges is called, where EncodeMapOfRecordsContent fails on "xml".
		rec := readyRecord(dal.NewKeyWithID("tags", "k"), map[string]any{"v": 1})
		return tx.Set(ctx, rec)
	})
	if err == nil {
		t.Fatal("RunReadwriteTransaction: expected flushChanges error, got nil")
	}
	if !strings.Contains(err.Error(), "encode map for") {
		t.Errorf("RunReadwriteTransaction error = %q, want 'encode map for'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// batching.go:125-127 — batchingTx.Set: non-map[string]any record data
// ---------------------------------------------------------------------------

// TestBatchingTx_Set_InvalidRecordData verifies that Set returns an error when
// the record's data is not map[string]any.
func TestBatchingTx_Set_InvalidRecordData(t *testing.T) {
	t.Parallel()
	srv := makeBatchServer(t, nil)
	defer srv.Close()

	def := singleRecordDef("tags", "data/tags", "{key}.yaml")
	bdb := newBatchingDB(t, srv, def)

	ctx := context.Background()
	err := bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "k")
		// Directly construct a record whose Data() returns a string, not map[string]any.
		rec := dal.NewRecordWithData(key, "not-a-map")
		rec.SetError(nil) // clear the "no data" sentinel so Data() returns the string
		return tx.Set(ctx, rec)
	})
	if err == nil {
		t.Fatal("Set: expected error for non-map data, got nil")
	}
	if err.Error() != "record data is not map[string]any" {
		t.Errorf("Set error = %q, want 'record data is not map[string]any'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// batching.go:153-155 — batchingTx.Insert: non-map[string]any record data
// ---------------------------------------------------------------------------

// TestBatchingTx_Insert_InvalidRecordData verifies that Insert returns an error when
// the record's data is not map[string]any.
func TestBatchingTx_Insert_InvalidRecordData(t *testing.T) {
	t.Parallel()
	srv := makeBatchServer(t, nil)
	defer srv.Close()

	def := singleRecordDef("tags", "data/tags", "{key}.yaml")
	bdb := newBatchingDB(t, srv, def)

	ctx := context.Background()
	err := bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		key := dal.NewKeyWithID("tags", "k")
		rec := dal.NewRecordWithData(key, "not-a-map")
		rec.SetError(nil)
		return tx.Insert(ctx, rec)
	})
	if err == nil {
		t.Fatal("Insert: expected error for non-map data, got nil")
	}
	if err.Error() != "record data is not map[string]any" {
		t.Errorf("Insert error = %q, want 'record data is not map[string]any'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// batching.go:158-160 — batchingTx.Insert MapOfRecords: ensureMapLoaded error
// ---------------------------------------------------------------------------

// TestBatchingTx_Insert_MapOfRecords_EnsureLoadError verifies that Insert for a
// MapOfRecords collection propagates ensureMapLoaded errors.
func TestBatchingTx_Insert_MapOfRecords_EnsureLoadError(t *testing.T) {
	t.Parallel()
	// Server always returns 500 → readFileWithSHA error → ensureMapLoaded fails.
	srv := newAPIErrorServer(t)
	defer srv.Close()

	def := mapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		rec := readyRecord(dal.NewKeyWithID("tags", "new"), map[string]any{"title": "New"})
		return tx.Insert(ctx, rec)
	})
	if err == nil {
		t.Fatal("Insert MapOfRecords load error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// batching.go:181-183 — batchingTx.Insert single-record: readErr when checking
// remote existence (not buffered, server returns API error)
// ---------------------------------------------------------------------------

// TestBatchingTx_Insert_SingleRecord_ReadError verifies that Insert propagates
// the API error returned by readFileWithSHA when the record is not buffered.
func TestBatchingTx_Insert_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()
	// Server returns 500 for content requests → readFileWithSHA error.
	srv := newAPIErrorServer(t)
	defer srv.Close()

	def := singleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		rec := readyRecord(dal.NewKeyWithID("tags", "k"), map[string]any{"v": 1})
		return tx.Insert(ctx, rec)
	})
	if err == nil {
		t.Fatal("Insert SingleRecord API error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// batching.go:190-192 — batchingTx.Insert single-record: encodeErr
// ---------------------------------------------------------------------------

// TestBatchingTx_Insert_SingleRecord_EncodeError_MapOfRecordsFormat verifies
// that Insert propagates an encode error when the collection uses an unsupported
// format and the record is not already buffered or found on the remote.
func TestBatchingTx_Insert_SingleRecord_EncodeError_AfterNotFound(t *testing.T) {
	t.Parallel()
	// Server returns 404 for content GETs so "not found" passes the remote check,
	// then encoding fails because of the unsupported format.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer srv.Close()

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
			},
		},
	}
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		rec := readyRecord(dal.NewKeyWithID("tags", "k"), map[string]any{"v": 1})
		return tx.Insert(ctx, rec)
	})
	if err == nil {
		t.Fatal("Insert SingleRecord encode error: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported record format") {
		t.Errorf("Insert error = %q, want 'unsupported record format'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// batching.go:206-208 — batchingTx.Delete MapOfRecords: ensureMapLoaded error
// ---------------------------------------------------------------------------

// TestBatchingTx_Delete_MapOfRecords_EnsureLoadError verifies that Delete for a
// MapOfRecords collection propagates ensureMapLoaded errors.
func TestBatchingTx_Delete_MapOfRecords_EnsureLoadError(t *testing.T) {
	t.Parallel()
	srv := newAPIErrorServer(t)
	defer srv.Close()

	def := mapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID("tags", "active"))
	})
	if err == nil {
		t.Fatal("Delete MapOfRecords load error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// batching.go:224-226 — batchingTx.Delete single-record: readErr when checking
// remote existence (no buffered entry, server returns API error)
// ---------------------------------------------------------------------------

// TestBatchingTx_Delete_SingleRecord_ReadError verifies that Delete propagates
// the API error returned by readFileWithSHA when there is no buffered entry.
func TestBatchingTx_Delete_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()
	srv := newAPIErrorServer(t)
	defer srv.Close()

	def := singleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID("tags", "k"))
	})
	if err == nil {
		t.Fatal("Delete SingleRecord API error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// batching.go:239-241 — ensureMapLoaded: early return when already loaded
// ---------------------------------------------------------------------------

// TestEnsureMapLoaded_AlreadyLoaded verifies the early-return path: calling Set
// twice on the same MapOfRecords key should only fire one remote read (the map
// is loaded on the first call and reused on the second).
func TestEnsureMapLoaded_AlreadyLoaded(t *testing.T) {
	t.Parallel()

	var getContentsCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path
		switch {
		case r.Method == "GET" && strings.Contains(p, "/contents/"):
			getContentsCalls++
			// Return an empty map so ensureMapLoaded succeeds.
			http.NotFound(w, r)
		case r.Method == "GET" && strings.Contains(p, "/git/ref/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "head-sha", "type": "commit"},
			})
		case r.Method == "GET" && strings.Contains(p, "/git/commits/head-sha"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":  "head-sha",
				"tree": map[string]any{"sha": "tree-sha"},
			})
		case r.Method == "POST" && strings.HasSuffix(p, "/git/trees"):
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-tree"})
		case r.Method == "POST" && strings.HasSuffix(p, "/git/commits"):
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-commit"})
		case r.Method == "PATCH" && strings.Contains(p, "/git/refs/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "new-commit"},
			})
		default:
			http.Error(w, "unexpected: "+r.Method+" "+p, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	def := mapRecordDef("tags", "data/tags", "tags.json")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	ctx := context.Background()
	err = bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		// First Set: ensureMapLoaded fires the GET /contents/ call.
		rec1 := readyRecord(dal.NewKeyWithID("tags", "a"), map[string]any{"v": 1})
		if setErr := tx.Set(ctx, rec1); setErr != nil {
			return fmt.Errorf("first Set: %w", setErr)
		}
		// Second Set for a different key in the same map file: ensureMapLoaded
		// must take the early-return path (map already loaded).
		rec2 := readyRecord(dal.NewKeyWithID("tags", "b"), map[string]any{"v": 2})
		if setErr := tx.Set(ctx, rec2); setErr != nil {
			return fmt.Errorf("second Set: %w", setErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
	if getContentsCalls != 1 {
		t.Errorf("GET /contents/ called %d times, want 1 (early-return should suppress second load)", getContentsCalls)
	}
}

// ---------------------------------------------------------------------------
// batching.go:251-253 — ensureMapLoaded: parseErr from malformed file
// ---------------------------------------------------------------------------

// TestEnsureMapLoaded_ParseError verifies that ensureMapLoaded returns an error
// when the fetched file content cannot be parsed as MapOfRecords.
func TestEnsureMapLoaded_ParseError(t *testing.T) {
	t.Parallel()
	fixtures := map[string]string{
		"data/tags/tags.json": `{"key": "not-a-map-of-maps"}`, // wrong shape
	}
	srv := makeBatchServer(t, fixtures)
	defer srv.Close()

	def := mapRecordDef("tags", "data/tags", "tags.json")
	bdb := newBatchingDB(t, srv, def)

	ctx := context.Background()
	err := bdb.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		rec := readyRecord(dal.NewKeyWithID("tags", "new"), map[string]any{"v": 1})
		return tx.Set(ctx, rec) // triggers ensureMapLoaded → parseErr
	})
	if err == nil {
		t.Fatal("Set with malformed map file: expected parse error, got nil")
	}
}

// ---------------------------------------------------------------------------
// tree_writer.go:44-46 — NewTreeWriter: Token branch (adds auth header option)
// ---------------------------------------------------------------------------

// TestNewTreeWriter_WithToken verifies that a non-empty Token is accepted and
// results in a valid TreeWriter (the token option is exercised, covering the
// cfg.Token != "" branch in NewTreeWriter).
func TestNewTreeWriter_WithToken(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "o", Repo: "r", Token: "ghp_testtoken"}
	w, err := NewTreeWriter(cfg)
	if err != nil {
		t.Fatalf("NewTreeWriter with token: %v", err)
	}
	if w == nil {
		t.Fatal("NewTreeWriter returned nil")
	}
}

// ---------------------------------------------------------------------------
// tree_writer.go:55-57 — NewTreeWriter: github.NewClient error (invalid URL)
// ---------------------------------------------------------------------------

// TestNewTreeWriter_InvalidAPIBaseURL verifies that an invalid APIBaseURL causes
// NewTreeWriter to return an error (github.WithEnterpriseURLs → github.NewClient fails).
func TestNewTreeWriter_InvalidAPIBaseURL(t *testing.T) {
	t.Parallel()
	// ":%bad/" is syntactically invalid — url.Parse fails, so github.NewClient
	// returns the error from WithEnterpriseURLs.
	cfg := Config{Owner: "o", Repo: "r", APIBaseURL: ":%bad/"}
	_, err := NewTreeWriter(cfg)
	if err == nil {
		t.Fatal("NewTreeWriter: expected error for invalid APIBaseURL, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create github client") {
		t.Errorf("NewTreeWriter error = %q, want 'failed to create github client'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// tree_writer.go:70-72 — ListFilesUnder: headTree error propagation
// ---------------------------------------------------------------------------

// TestListFilesUnder_HeadTreeError verifies that ListFilesUnder propagates
// errors returned by headTree (e.g. GetRef fails).
func TestListFilesUnder_HeadTreeError(t *testing.T) {
	t.Parallel()
	// Server fails on GetRef → headTree fails → ListFilesUnder must return the error.
	srv := newAPIErrorServer(t)
	defer srv.Close()

	w := newTestTreeWriter(t, srv, "main")
	_, err := w.ListFilesUnder(context.Background(), "data/cities")
	if err == nil {
		t.Fatal("ListFilesUnder headTree error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// tree_writer.go:111-113 — CommitChanges: resolveBranch error propagation
//
// CommitChanges calls resolveBranch before headTreeForBranch. To hit the
// resolveBranch error branch we need cfg.Ref == "" so that resolveBranch must
// call Repositories.Get, and the server must return an error for that call.
// ---------------------------------------------------------------------------

// TestCommitChanges_ResolveBranchError verifies that CommitChanges propagates
// an error from resolveBranch when the default branch cannot be resolved.
func TestCommitChanges_ResolveBranchError(t *testing.T) {
	t.Parallel()
	// Server returns 500 for all requests — Repositories.Get fails.
	srv := newAPIErrorServer(t)
	defer srv.Close()

	// Empty Ref: resolveBranch must call Repositories.Get to discover the branch.
	cfg := Config{Owner: "owner", Repo: "repo", APIBaseURL: srv.URL + "/"}
	w, err := NewTreeWriter(cfg)
	if err != nil {
		t.Fatalf("NewTreeWriter: %v", err)
	}
	_, err = w.CommitChanges(context.Background(), "msg", []TreeChange{{Path: "x", Content: []byte("y")}})
	if err == nil {
		t.Fatal("CommitChanges resolveBranch error: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go:200-202 — readwriteTx.Delete single-record: deleteFile error
// ---------------------------------------------------------------------------

// TestReadwriteTx_Delete_SingleRecord_DeleteFileError verifies that Delete
// propagates the error returned by deleteFile (e.g. 403 Forbidden).
func TestReadwriteTx_Delete_SingleRecord_DeleteFileError(t *testing.T) {
	t.Parallel()
	// The fixture's path must match what resolveRecordPath produces for a
	// single-record collection. Name="{key}.yaml" → base="$records" →
	// path = "data/tags/$records/active.yaml".
	fixtures := []githubFileFixture{{
		path:    "data/tags/$records/active.yaml",
		content: "title: Active\n",
	}}
	server := newReadWriteFailServer(t, fixtures) // GET succeeds, DELETE → 403
	defer server.Close()

	def := buildSingleRecordDef("tags", "data/tags", "{key}.yaml")
	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	db, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		t.Fatalf("NewGitHubDBWithDef: %v", err)
	}

	ctx := context.Background()
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID("tags", "active"))
	})
	if err == nil {
		t.Fatal("Delete: expected error from deleteFile, got nil")
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go — readwriteTx.Delete MapOfRecords: encodeErr via injected encodeFn
//
// The encodeErr guard after deleting an entry from a MapOfRecords is normally
// unreachable because ParseMapOfRecordsContent already rejects unsupported
// formats, and for supported formats parsed data can always be re-encoded.
// We inject a failing encodeRecordFn via the readwriteTx.encodeRecordFn seam
// to cover the branch.
// ---------------------------------------------------------------------------

// TestReadwriteTx_Delete_MapOfRecords_EncodeError verifies that Delete
// propagates an error returned by the encode step via the injected encodeFn.
func TestReadwriteTx_Delete_MapOfRecords_EncodeError(t *testing.T) {
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
	concreteDB := db.(*githubDB)

	ctx := context.Background()
	tx := readwriteTx{
		readonlyTx: readonlyTx{db: concreteDB},
		encodeRecordFn: func(_ map[string]any, _ ingitdb.RecordFormat) ([]byte, error) {
			return nil, fmt.Errorf("injected encode error")
		},
	}
	err = tx.Delete(ctx, dal.NewKeyWithID("tags", "active"))
	if err == nil {
		t.Fatal("Delete: expected encode error, got nil")
	}
	if err.Error() != "injected encode error" {
		t.Errorf("Delete error = %q, want 'injected encode error'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// tx_readwrite.go:257-259 — encodeRecordContent: TOML marshal error
// ---------------------------------------------------------------------------

// TestEncodeRecordContent_TOMLMarshalError verifies that encodeRecordContent
// returns a wrapped error when toml.Marshal fails.
// The go-toml library returns an error for channel values (which cannot be serialised).
func TestEncodeRecordContent_TOMLMarshalError(t *testing.T) {
	t.Parallel()
	ch := make(chan int)
	data := map[string]any{"invalid": ch}
	_, err := encodeRecordContent(data, ingitdb.RecordFormatTOML)
	if err == nil {
		t.Fatal("encodeRecordContent(toml) expected error for channel value, got nil")
	}
	if !strings.Contains(err.Error(), "failed to encode TOML record") {
		t.Errorf("encodeRecordContent(toml) error = %q, want 'failed to encode TOML record'", err.Error())
	}
}
