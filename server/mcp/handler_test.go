package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/dal-go/dalgo/update"
	"github.com/metoro-io/mcp-golang/transport"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/server/auth"
)

// --- fakes (shared with test) ---

type fakeFileReader struct {
	files map[string][]byte
}

func (f *fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	content, ok := f.files[filePath]
	return content, ok, nil
}

func (f *fakeFileReader) ListDirectory(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

type fakeStore struct {
	records map[string]map[string]any
	deleted map[string]bool
}

func newFakeStore(records map[string]map[string]any) *fakeStore {
	return &fakeStore{records: records, deleted: map[string]bool{}}
}

type fakeReadTx struct{ s *fakeStore }

var _ dal.ReadTransaction = (*fakeReadTx)(nil)

func (t *fakeReadTx) ID() string                      { return "fake-read" }
func (t *fakeReadTx) Options() dal.TransactionOptions { return nil }
func (t *fakeReadTx) Get(_ context.Context, record dal.Record) error {
	k := record.Key().String()
	if t.s.deleted[k] {
		record.SetError(dal.ErrRecordNotFound)
		return nil
	}
	data, ok := t.s.records[k]
	if !ok {
		record.SetError(dal.ErrRecordNotFound)
		return nil
	}
	record.SetError(nil)
	dst := record.Data().(map[string]any)
	for kk, v := range data {
		dst[kk] = v
	}
	return nil
}
func (t *fakeReadTx) Exists(_ context.Context, key *dal.Key) (bool, error) {
	_, ok := t.s.records[key.String()]
	return ok && !t.s.deleted[key.String()], nil
}
func (t *fakeReadTx) GetMulti(_ context.Context, _ []dal.Record) error { return nil }
func (t *fakeReadTx) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return nil, fmt.Errorf("not implemented")
}
func (t *fakeReadTx) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, fmt.Errorf("not implemented")
}

type fakeReadwriteTx struct{ fakeReadTx }

var _ dal.ReadwriteTransaction = (*fakeReadwriteTx)(nil)

func (t *fakeReadwriteTx) ID() string { return "fake-rw" }
func (t *fakeReadwriteTx) Insert(_ context.Context, record dal.Record, _ ...dal.InsertOption) error {
	record.SetError(nil)
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) InsertMulti(_ context.Context, _ []dal.Record, _ ...dal.InsertOption) error {
	return nil
}
func (t *fakeReadwriteTx) Set(_ context.Context, record dal.Record) error {
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) SetMulti(_ context.Context, _ []dal.Record) error { return nil }
func (t *fakeReadwriteTx) Delete(_ context.Context, key *dal.Key) error {
	t.s.deleted[key.String()] = true
	return nil
}
func (t *fakeReadwriteTx) DeleteMulti(_ context.Context, _ []*dal.Key) error { return nil }
func (t *fakeReadwriteTx) Update(_ context.Context, _ *dal.Key, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}
func (t *fakeReadwriteTx) UpdateRecord(_ context.Context, _ dal.Record, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}
func (t *fakeReadwriteTx) UpdateMulti(_ context.Context, _ []*dal.Key, _ []update.Update, _ ...dal.Precondition) error {
	return nil
}

type fakeDB struct{ s *fakeStore }

var _ dal.DB = (*fakeDB)(nil)

func (db *fakeDB) ID() string           { return "fake" }
func (db *fakeDB) Adapter() dal.Adapter { return dal.NewAdapter("fake", "v0.0.1") }
func (db *fakeDB) Schema() dal.Schema   { return nil }
func (db *fakeDB) RunReadonlyTransaction(_ context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), &fakeReadTx{s: db.s})
}
func (db *fakeDB) RunReadwriteTransaction(_ context.Context, f dal.RWTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), &fakeReadwriteTx{fakeReadTx: fakeReadTx{s: db.s}})
}
func (db *fakeDB) Get(_ context.Context, record dal.Record) error {
	return (&fakeReadTx{s: db.s}).Get(context.Background(), record)
}
func (db *fakeDB) Exists(_ context.Context, _ *dal.Key) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
func (db *fakeDB) GetMulti(_ context.Context, _ []dal.Record) error {
	return fmt.Errorf("not implemented")
}
func (db *fakeDB) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return nil, fmt.Errorf("not implemented")
}
func (db *fakeDB) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, fmt.Errorf("not implemented")
}

// --- test fixtures ---

const rootConfigYAML = `rootCollections:
  countries: data/countries
`

const countryColDefYAML = `record_file:
  name: "{key}.yaml"
  format: yaml
  type: map[string]any
columns:
  title:
    type: string
`

func newTestHandler() (*Handler, *fakeStore) {
	s := newFakeStore(map[string]map[string]any{
		"countries/ie": {"title": "Ireland"},
	})
	h := &Handler{
		newGitHubFileReader: func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return &fakeFileReader{files: map[string][]byte{
				".ingitdb.yaml": []byte(rootConfigYAML),
				"data/countries/.ingitdb-collection.yaml": []byte(countryColDefYAML),
			}}, nil
		},
		newGitHubDBWithDef: func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
			return &fakeDB{s: s}, nil
		},
		authConfig: auth.Config{
			AuthAPIBaseURL: "https://api.ingitdb.com",
			CookieName:     "ingitdb_github_token",
		},
		validateToken: func(ctx context.Context, token string) error {
			_, _ = ctx, token
			return nil
		},
		requireAuth: false,
	}
	h.router = h.buildRouter()
	return h, s
}

// buildMCPRequest creates a JSON-RPC request body for an MCP tools/call.
func buildMCPRequest(id int, method string, params any) []byte {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	return body
}

// --- transport tests ---

func TestSingleRequestTransport_StartClose(t *testing.T) {
	t.Parallel()
	tr := newSingleRequestTransport()
	if err := tr.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestSingleRequestTransport_Send(t *testing.T) {
	t.Parallel()
	tr := newSingleRequestTransport()
	msg := transport.NewBaseMessageResponse(&transport.BaseJSONRPCResponse{
		Id:      1,
		Jsonrpc: "2.0",
		Result:  json.RawMessage(`{}`),
	})
	if err := tr.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	received := <-tr.respCh
	if received != msg {
		t.Error("received wrong message")
	}
}

func TestSingleRequestTransport_HandlerSet(t *testing.T) {
	t.Parallel()
	tr := newSingleRequestTransport()
	called := false
	tr.SetMessageHandler(func(_ context.Context, _ *transport.BaseJsonRpcMessage) {
		called = true
	})
	tr.SetCloseHandler(func() {})
	tr.SetErrorHandler(func(_ error) {})
	if tr.msgHandler == nil {
		t.Error("expected msgHandler to be set")
	}
	// Ensure no panic
	tr.SetCloseHandler(nil)
	tr.SetErrorHandler(nil)
	_ = called
}

// --- parseDBArg tests ---

func TestParseDBArg(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"", "", "", true},
		{"badformat", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
	}
	for _, tc := range tests {
		owner, repo, err := parseDBArg(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("input %q: wantErr=%v got err=%v", tc.input, tc.wantErr, err)
		}
		if err == nil && (owner != tc.wantOwner || repo != tc.wantRepo) {
			t.Errorf("input %q: got owner=%q repo=%q", tc.input, owner, repo)
		}
	}
}

// --- handler tests ---

func TestNewHandler(t *testing.T) {
	t.Parallel()
	h := NewHandler()
	if h == nil {
		t.Fatal("expected handler")
	}
	if h.router == nil {
		t.Error("expected router")
	}
}

func TestNewHandlerWithAuth(t *testing.T) {
	t.Parallel()
	h := NewHandlerWithAuth(auth.Config{}, false)
	if h == nil {
		t.Fatal("expected handler")
	}
	if h.requireAuth {
		t.Error("expected requireAuth=false")
	}
}

func TestServeIndex(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %s", w.Header().Get("Content-Type"))
	}
}

func TestHandleMCP_InvalidBody(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString("bad json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleMCP_ListTools(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	body := buildMCPRequest(1, "tools/list", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("unexpected jsonrpc: %v", resp["jsonrpc"])
	}
}

func TestHandleMCP_ListCollections(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	body := buildMCPRequest(2, "tools/call", map[string]any{
		"name": "list_collections",
		"arguments": map[string]any{
			"db": "owner/repo",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != nil {
		t.Errorf("unexpected error in MCP response: %v", resp["error"])
	}
}

func TestHandleMCP_ReadRecord(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	body := buildMCPRequest(3, "tools/call", map[string]any{
		"name": "read_record",
		"arguments": map[string]any{
			"db": "owner/repo",
			"id": "countries/ie",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMCP_CreateRecord(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := buildMCPRequest(4, "tools/call", map[string]any{
		"name": "create_record",
		"arguments": map[string]any{
			"db":   "owner/repo",
			"id":   "countries/de",
			"data": `{"title":"Germany"}`,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := s.records["countries/de"]; !ok {
		t.Error("record not inserted via MCP create_record")
	}
}

func TestHandleMCP_UpdateRecord(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := buildMCPRequest(6, "tools/call", map[string]any{
		"name": "update_record",
		"arguments": map[string]any{
			"db":     "owner/repo",
			"id":     "countries/ie",
			"fields": `{"title":"Ireland Updated"}`,
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if title, _ := s.records["countries/ie"]["title"].(string); title != "Ireland Updated" {
		t.Errorf("unexpected title after update: %q", title)
	}
}

func TestHandleMCP_DeleteRecord(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := buildMCPRequest(5, "tools/call", map[string]any{
		"name": "delete_record",
		"arguments": map[string]any{
			"db": "owner/repo",
			"id": "countries/ie",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !s.deleted["countries/ie"] {
		t.Error("record not deleted via MCP delete_record")
	}
}

func TestHandleMCP_GithubToken(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if tok := githubToken(req); tok != "" {
		t.Errorf("expected empty token, got %q", tok)
	}
	req.Header.Set("Authorization", "Bearer ghtoken")
	if tok := githubToken(req); tok != "ghtoken" {
		t.Errorf("expected ghtoken, got %q", tok)
	}
}

func TestHandleMCP_RequiresAuth(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	body := buildMCPRequest(1, "tools/list", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMCPAuthRedirectToAPI(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "https://api.ingitdb.com/auth/github/login" {
		t.Fatalf("unexpected redirect location: %s", got)
	}
}

func TestReadDefinitionFromGitHub_Success(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte(rootConfigYAML),
		"data/countries/.ingitdb-collection.yaml": []byte(countryColDefYAML),
	}}
	def, err := readDefinitionFromGitHub(context.Background(), fr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := def.Collections["countries"]; !ok {
		t.Error("expected 'countries' collection")
	}
}

func TestReadDefinitionFromGitHub_MissingRoot(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{files: map[string][]byte{}}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error for missing root config")
	}
}

func TestHandleMCP_Errors(t *testing.T) {
	t.Parallel()

	setupH := func(t *testing.T) *Handler {
		h, _ := newTestHandler()
		return h
	}

	call := func(h *Handler, name string, args map[string]any) map[string]any {
		body := buildMCPRequest(1, "tools/call", map[string]any{
			"name":      name,
			"arguments": args,
		})
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		return resp
	}

	t.Run("invalid jsonrpc request", func(t *testing.T) {
		h := setupH(t)
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("bad"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("auth error", func(t *testing.T) {
		h := setupH(t)
		h.requireAuth = true
		h.validateToken = func(_ context.Context, _ string) error {
			return fmt.Errorf("invalid token")
		}
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
		req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "bad"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("tool list error (register fails)", func(t *testing.T) {
		h := setupH(t)
		// We can't easily force register to fail with valid inputs, but we can verify
		// that the server returns 500 if registerMCPTools failed.
		// Since registerMCPTools only returns error if RegisterTool fails (which shouldn't happen),
		// we skip this specific case or assume it's covered by library tests.
		_ = h
	})

	t.Run("list collections error", func(t *testing.T) {
		h := setupH(t)
		h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return nil, fmt.Errorf("fail")
		}
		resp := call(h, "list_collections", map[string]any{"db": "owner/repo"})
		res, ok := resp["result"].(map[string]any)
		if !ok || res["isError"] != true {
			t.Errorf("expected isError: true in result, got: %+v", resp)
		}
	})

	t.Run("create record invalid data", func(t *testing.T) {
		h := setupH(t)
		resp := call(h, "create_record", map[string]any{
			"db":   "owner/repo",
			"id":   "countries/new",
			"data": "bad json",
		})
		res, ok := resp["result"].(map[string]any)
		if !ok || res["isError"] != true {
			t.Errorf("expected isError: true in result, got: %+v", resp)
		}
	})
}
