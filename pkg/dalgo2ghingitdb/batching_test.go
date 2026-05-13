package dalgo2ghingitdb

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// TestBatchingTx_FlushChanges_SingleRecordBuffersAndDeletions verifies the
// flush output for a SingleRecord-only batch: every buffered write becomes a
// TreeChange with its content, every buffered delete becomes a TreeChange
// with nil Content.
func TestBatchingTx_FlushChanges_SingleRecordBuffersAndDeletions(t *testing.T) {
	t.Parallel()
	tx := &batchingTx{
		bufferedFiles: map[string]TreeChange{
			"data/cities/ie.yaml": {Path: "data/cities/ie.yaml", Content: []byte("name: Ireland\n")},
			"data/cities/us.yaml": {Path: "data/cities/us.yaml", Content: nil}, // deletion
		},
		workingMaps: map[string]map[string]map[string]any{},
		mapColDefs:  map[string]*ingitdb.CollectionDef{},
		mapLoaded:   map[string]bool{},
	}
	changes, err := tx.flushChanges()
	if err != nil {
		t.Fatalf("flushChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}
	byPath := make(map[string]TreeChange, len(changes))
	for _, ch := range changes {
		byPath[ch.Path] = ch
	}
	if string(byPath["data/cities/ie.yaml"].Content) != "name: Ireland\n" {
		t.Errorf("ie content = %q, want %q", byPath["data/cities/ie.yaml"].Content, "name: Ireland\n")
	}
	if byPath["data/cities/us.yaml"].Content != nil {
		t.Errorf("us content should be nil (deletion), got %q", byPath["data/cities/us.yaml"].Content)
	}
}

// TestBatchingTx_FlushChanges_EmptyMapBecomesDeletion verifies that a
// MapOfRecords working copy that has been emptied flushes as a file
// deletion — preserving the "leave no trace" property.
func TestBatchingTx_FlushChanges_EmptyMapBecomesDeletion(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "test.tags",
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: ingitdb.MapOfRecords,
			Format:     ingitdb.RecordFormatYAML,
		},
	}
	tx := &batchingTx{
		bufferedFiles: map[string]TreeChange{},
		workingMaps: map[string]map[string]map[string]any{
			"data/tags/all.yaml": {}, // emptied
		},
		mapColDefs: map[string]*ingitdb.CollectionDef{
			"data/tags/all.yaml": colDef,
		},
		mapLoaded: map[string]bool{
			"data/tags/all.yaml": true,
		},
	}
	changes, err := tx.flushChanges()
	if err != nil {
		t.Fatalf("flushChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Content != nil {
		t.Errorf("emptied map should flush as deletion (nil Content), got %q", changes[0].Content)
	}
}

// TestBatchingGitHubDB_OneCommitPerBatch is the spec-compliance test:
// a worker that calls tx.Set N times must produce exactly one Git Data API
// commit, not N commits via the Contents API.
//
// Modifies package-level state? No — uses a self-contained httptest server.
func TestBatchingGitHubDB_OneCommitPerBatch(t *testing.T) {
	t.Parallel()

	var (
		createTreeCalls   atomic.Int32
		createCommitCalls atomic.Int32
		updateRefCalls    atomic.Int32
		putContentsCalls  atomic.Int32 // The per-file Contents API — MUST be 0.
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		// HEAD ref for the default branch.
		case r.Method == "GET" && strings.Contains(path, "/git/ref/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "head-commit-sha", "type": "commit"},
			})
		// The commit object → tree SHA.
		case r.Method == "GET" && strings.Contains(path, "/git/commits/head-commit-sha"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":  "head-commit-sha",
				"tree": map[string]any{"sha": "base-tree-sha"},
			})
		// Create tree.
		case r.Method == "POST" && strings.HasSuffix(path, "/git/trees"):
			createTreeCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-tree-sha"})
		// Create commit.
		case r.Method == "POST" && strings.HasSuffix(path, "/git/commits"):
			createCommitCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-commit-sha"})
		// Update ref.
		case r.Method == "PATCH" && strings.Contains(path, "/git/refs/heads/main"):
			updateRefCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "new-commit-sha"},
			})
		// The per-file Contents API must NEVER fire during batching.
		case (r.Method == "PUT" || r.Method == "DELETE") && strings.Contains(path, "/contents/"):
			putContentsCalls.Add(1)
			http.Error(w, "batching tx must not call the Contents API", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+path, http.StatusNotImplemented)
		}
	}))
	defer srv.Close()

	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main", APIBaseURL: srv.URL + "/"}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"cities": {
				ID:      "cities",
				DirPath: "data/cities",
				RecordFile: &ingitdb.RecordFileDef{
					RecordType: ingitdb.SingleRecord,
					Format:     ingitdb.RecordFormatYAML,
					Name:       "{key}.yaml",
				},
			},
		},
	}
	bdb, err := NewBatchingGitHubDB(cfg, def, "ingitdb: update cities (batch)")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	// Worker writes three records — should produce ONE commit, not three.
	err = bdb.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, name := range []string{"ie", "gb", "us"} {
			rec := dal.NewRecordWithData(dal.NewKeyWithID("cities", name), map[string]any{"region": "world"})
			if setErr := tx.Set(ctx, rec); setErr != nil {
				return setErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}

	if got := createTreeCalls.Load(); got != 1 {
		t.Errorf("CreateTree calls = %d, want 1", got)
	}
	if got := createCommitCalls.Load(); got != 1 {
		t.Errorf("CreateCommit calls = %d, want 1", got)
	}
	if got := updateRefCalls.Load(); got != 1 {
		t.Errorf("UpdateRef calls = %d, want 1", got)
	}
	if got := putContentsCalls.Load(); got != 0 {
		t.Errorf("PUT/DELETE /contents calls = %d, want 0 (batching MUST use Git Data API)", got)
	}
}

// TestBatchingGitHubDB_WorkerErrorSkipsFlush verifies that when the worker
// returns an error, no commit is attempted. The remote is untouched.
func TestBatchingGitHubDB_WorkerErrorSkipsFlush(t *testing.T) {
	t.Parallel()

	var gitDataCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/") {
			gitDataCalls.Add(1)
		}
		http.Error(w, "no requests expected on worker error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main", APIBaseURL: srv.URL + "/"}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"cities": {
				ID:      "cities",
				DirPath: "data/cities",
				RecordFile: &ingitdb.RecordFileDef{
					RecordType: ingitdb.SingleRecord,
					Format:     ingitdb.RecordFormatYAML,
					Name:       "{key}.yaml",
				},
			},
		},
	}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	wantErr := errors.New("worker decided to bail")
	err = bdb.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		_ = tx.Set(ctx, dal.NewRecordWithData(dal.NewKeyWithID("cities", "ie"), map[string]any{"x": 1}))
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wantErr propagated, got: %v", err)
	}
	if got := gitDataCalls.Load(); got != 0 {
		t.Errorf("expected 0 Git Data API calls on worker error, got %d", got)
	}
}

// TestBatchingGitHubDB_EmptyWorkerNoCommit verifies that a worker that
// buffers nothing produces zero commits — no point disturbing the remote.
func TestBatchingGitHubDB_EmptyWorkerNoCommit(t *testing.T) {
	t.Parallel()

	var anyCall atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		anyCall.Add(1)
		http.Error(w, "no requests expected for empty batch", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main", APIBaseURL: srv.URL + "/"}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	bdb, err := NewBatchingGitHubDB(cfg, def, "msg")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}
	err = bdb.RunReadwriteTransaction(context.Background(), func(_ context.Context, _ dal.ReadwriteTransaction) error {
		return nil // no writes
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got := anyCall.Load(); got != 0 {
		t.Errorf("expected 0 API calls for empty batch, got %d", got)
	}
}

// TestBatchingGitHubDB_NilDefinition verifies guard: NewBatchingGitHubDB
// requires a non-nil Definition (matches NewGitHubDBWithDef behavior).
func TestBatchingGitHubDB_NilDefinition(t *testing.T) {
	t.Parallel()
	_, err := NewBatchingGitHubDB(Config{Owner: "o", Repo: "r"}, nil, "msg")
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

// TestBatchingGitHubDB_EmptyMessage verifies guard: NewBatchingGitHubDB
// requires a commit message (we have no sensible default).
func TestBatchingGitHubDB_EmptyMessage(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	_, err := NewBatchingGitHubDB(Config{Owner: "o", Repo: "r"}, def, "")
	if err == nil {
		t.Fatal("expected error for empty commit message")
	}
}
