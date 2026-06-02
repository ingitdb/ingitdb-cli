package dalgo2ghingitdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// newCommitMessageCapturingServer returns an httptest server implementing the
// minimal Git Data API surface used by a batching commit, plus a pointer that
// receives the commit message sent to POST /git/commits.
func newCommitMessageCapturingServer(t *testing.T, captured *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case r.Method == "GET" && strings.Contains(path, "/git/ref/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "head-commit-sha", "type": "commit"},
			})
		case r.Method == "GET" && strings.Contains(path, "/git/commits/head-commit-sha"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":  "head-commit-sha",
				"tree": map[string]any{"sha": "base-tree-sha"},
			})
		case r.Method == "POST" && strings.HasSuffix(path, "/git/trees"):
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-tree-sha"})
		case r.Method == "POST" && strings.HasSuffix(path, "/git/commits"):
			var body struct {
				Message string `json:"message"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			*captured = body.Message
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "new-commit-sha"})
		case r.Method == "PATCH" && strings.Contains(path, "/git/refs/heads/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ref":    "refs/heads/main",
				"object": map[string]any{"sha": "new-commit-sha"},
			})
		default:
			http.Error(w, "unexpected request: "+r.Method+" "+path, http.StatusNotImplemented)
		}
	}))
}

func commitMessageTestDef() *ingitdb.Definition {
	return &ingitdb.Definition{
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
}

func runOneSetTx(t *testing.T, bdb *BatchingGitHubDB, options ...dal.TransactionOption) {
	t.Helper()
	err := bdb.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, dal.NewRecordWithData(dal.NewKeyWithID("cities", "ie"), map[string]any{"region": "eu"}))
	}, options...)
	if err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}
}

// TestBatchingGitHubDB_TxMessageOverridesDefault verifies that a transaction
// message (dal.TxWithMessage) is used as the commit message in place of the
// construction-time default.
func TestBatchingGitHubDB_TxMessageOverridesDefault(t *testing.T) {
	var captured string
	srv := newCommitMessageCapturingServer(t, &captured)
	defer srv.Close()

	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, commitMessageTestDef(), "default construction message")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	runOneSetTx(t, bdb, dal.TxWithMessage("tx-level commit message"))

	if captured != "tx-level commit message" {
		t.Errorf("commit message: got %q, want %q", captured, "tx-level commit message")
	}
}

// TestBatchingGitHubDB_NoTxMessageUsesDefault verifies the construction-time
// default is used when the transaction sets no message.
func TestBatchingGitHubDB_NoTxMessageUsesDefault(t *testing.T) {
	var captured string
	srv := newCommitMessageCapturingServer(t, &captured)
	defer srv.Close()

	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main", APIBaseURL: srv.URL + "/"}
	bdb, err := NewBatchingGitHubDB(cfg, commitMessageTestDef(), "default construction message")
	if err != nil {
		t.Fatalf("NewBatchingGitHubDB: %v", err)
	}

	runOneSetTx(t, bdb)

	if captured != "default construction message" {
		t.Errorf("commit message: got %q, want %q", captured, "default construction message")
	}
}
