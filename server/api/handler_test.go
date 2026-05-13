package api

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/server/auth"
)

// cryptoRandRead is the real rand.Read used as default in test handlers.
var cryptoRandRead = cryptorand.Read

// --- fakes ---

// fakeFileReader implements dalgo2ghingitdb.FileReader returning preset content.
// readErrors maps file paths to errors that should be returned instead of content.
type fakeFileReader struct {
	files      map[string][]byte
	readErrors map[string]error
}

func (f *fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	if f.readErrors != nil {
		if err, ok := f.readErrors[filePath]; ok {
			return nil, false, err
		}
	}
	content, ok := f.files[filePath]
	return content, ok, nil
}

func (f *fakeFileReader) ListDirectory(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// fakeStore holds mutable state shared across fakeTx instances.
type fakeStore struct {
	records map[string]map[string]any // key.String() → data
	deleted map[string]bool
}

func newFakeStore(records map[string]map[string]any) *fakeStore {
	return &fakeStore{records: records, deleted: map[string]bool{}}
}

// fakeReadTx implements dal.ReadTransaction.
type fakeReadTx struct {
	s      *fakeStore
	getErr error // if non-nil, Get() returns this error
}

var _ dal.ReadTransaction = (*fakeReadTx)(nil)

func (t *fakeReadTx) ID() string                      { return "fake-read" }
func (t *fakeReadTx) Options() dal.TransactionOptions { return nil }
func (t *fakeReadTx) Get(_ context.Context, record dal.Record) error {
	if t.getErr != nil {
		return t.getErr
	}
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

// fakeReadwriteTx implements dal.ReadwriteTransaction.
type fakeReadwriteTx struct {
	fakeReadTx
	setErr    error // if non-nil, Set() returns this error
	deleteErr error // if non-nil, Delete() returns this error
	insertErr error // if non-nil, Insert() returns this error
}

var _ dal.ReadwriteTransaction = (*fakeReadwriteTx)(nil)

func (t *fakeReadwriteTx) ID() string { return "fake-rw" }
func (t *fakeReadwriteTx) Insert(_ context.Context, record dal.Record, _ ...dal.InsertOption) error {
	if t.insertErr != nil {
		return t.insertErr
	}
	record.SetError(nil)
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) InsertMulti(_ context.Context, _ []dal.Record, _ ...dal.InsertOption) error {
	return nil
}
func (t *fakeReadwriteTx) Set(_ context.Context, record dal.Record) error {
	if t.setErr != nil {
		return t.setErr
	}
	t.s.records[record.Key().String()] = record.Data().(map[string]any)
	return nil
}
func (t *fakeReadwriteTx) SetMulti(_ context.Context, _ []dal.Record) error { return nil }
func (t *fakeReadwriteTx) Delete(_ context.Context, key *dal.Key) error {
	if t.deleteErr != nil {
		return t.deleteErr
	}
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

// fakeDB implements dal.DB with a fakeStore.
// Error fields allow injecting failures at different layers:
//   - readTxErr: returned from RunReadonlyTransaction before calling the worker
//   - writeTxErr: returned from RunReadwriteTransaction before calling the worker
//   - getTxErr: injected into the tx so that tx.Get() returns this error
//   - setErr / deleteErr / insertErr: injected into the rw-tx for op-level errors
type fakeDB struct {
	dal.NoConcurrency
	s          *fakeStore
	readTxErr  error
	writeTxErr error
	getTxErr   error
	setErr     error
	deleteErr  error
	insertErr  error
}

func (db *fakeDB) ID() string { return "fake" }
func (db *fakeDB) Adapter() dal.Adapter {
	return dal.NewAdapter("fake", "v0.0.1")
}
func (db *fakeDB) Schema() dal.Schema { return nil }
func (db *fakeDB) RunReadonlyTransaction(_ context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	if db.readTxErr != nil {
		return db.readTxErr
	}
	return f(context.Background(), &fakeReadTx{s: db.s, getErr: db.getTxErr})
}
func (db *fakeDB) RunReadwriteTransaction(_ context.Context, f dal.RWTxWorker, _ ...dal.TransactionOption) error {
	if db.writeTxErr != nil {
		return db.writeTxErr
	}
	tx := &fakeReadwriteTx{
		fakeReadTx: fakeReadTx{s: db.s, getErr: db.getTxErr},
		setErr:     db.setErr,
		deleteErr:  db.deleteErr,
		insertErr:  db.insertErr,
	}
	return f(context.Background(), tx)
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

// --- helper to build a test handler ---

const rootConfigYAML = `countries: data/countries
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
				".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
				"data/countries/.collection/countries.yaml": []byte(countryColDefYAML),
			}}, nil
		},
		newGitHubDBWithDef: func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
			return &fakeDB{s: s}, nil
		},
		authConfig: auth.Config{
			GitHubClientID:     "client-id",
			GitHubClientSecret: "client-secret",
			CallbackURL:        "https://api.ingitdb.com/auth/github/callback",
			Scopes:             []string{"public_repo", "read:user"},
			CookieDomain:       ".ingitdb.com",
			CookieName:         "ingitdb_github_token",
			CookieSecure:       true,
			AuthAPIBaseURL:     "https://api.ingitdb.com",
		},
		exchangeCodeForToken: func(ctx context.Context, code string) (string, error) {
			_, _ = ctx, code
			return "oauth-token", nil
		},
		validateToken: func(ctx context.Context, token string) error {
			_, _ = ctx, token
			return nil
		},
		randRead:    cryptoRandRead,
		requireAuth: false,
	}
	h.router = h.buildRouter()
	return h, s
}

// --- tests ---

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

func TestListCollections_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCollections_InvalidDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=badformat", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListCollections_Success(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var ids []string
	if err := json.NewDecoder(w.Body).Decode(&ids); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(ids) != 1 || ids[0] != "countries" {
		t.Errorf("unexpected collections: %v", ids)
	}
}

func TestReadRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReadRecord_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/xx", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReadRecord_Success(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var data map[string]any
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if data["title"] != "Ireland" {
		t.Errorf("unexpected data: %v", data)
	}
}

func TestCreateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de", strings.NewReader("bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := `{"title":"Germany"}`
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if _, ok := s.records["countries/de"]; !ok {
		t.Error("record not inserted into fake DB")
	}
}

func TestUpdateRecord_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	body := `{"title":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/xx", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	body := `{"title":"Ireland Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if title, _ := s.records["countries/ie"]["title"].(string); title != "Ireland Updated" {
		t.Errorf("unexpected title after update: %q", title)
	}
}

func TestDeleteRecord_Success(t *testing.T) {
	t.Parallel()
	h, s := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !s.deleted["countries/ie"] {
		t.Error("record not marked as deleted in fake DB")
	}
}

func TestDeleteRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestParseDBParam(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"db=owner/repo", "owner", "repo", false},
		{"db=", "", "", true},
		{"db=badformat", "", "", true},
		{"db=/repo", "", "", true},
		{"db=owner/", "", "", true},
		{"", "", "", true},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?"+tc.query, nil)
		owner, repo, err := parseDBParam(req)
		if (err != nil) != tc.wantErr {
			t.Errorf("query %q: wantErr=%v got err=%v", tc.query, tc.wantErr, err)
		}
		if err == nil && (owner != tc.wantOwner || repo != tc.wantRepo) {
			t.Errorf("query %q: got owner=%q repo=%q", tc.query, owner, repo)
		}
	}
}

func TestGithubToken(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if tok := githubToken(req); tok != "" {
		t.Errorf("expected empty token, got %q", tok)
	}
	req.Header.Set("Authorization", "Bearer mytoken")
	if tok := githubToken(req); tok != "mytoken" {
		t.Errorf("expected mytoken, got %q", tok)
	}
}

func TestGitHubLoginRedirect(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Fatalf("unexpected redirect: %s", location)
	}
	cookies := w.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == oauthStateCookieName && cookie.Domain != "" {
			t.Fatalf("expected host-only oauth state cookie, got domain=%q", cookie.Domain)
		}
	}
}

func TestGitHubLoginRedirect_ConfigError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.authConfig.GitHubClientID = "" // Invalid config
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGitHubCallbackSetsCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state=state123", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state123"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "ingitdb_github_token=") {
		t.Fatalf("expected auth cookie to be set, got %q", setCookie)
	}
	if !strings.Contains(w.Body.String(), "Successfully authenticated") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestGitHubCallbackAcceptsLegacyStateCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=abc&state=legacy123", nil)
	req.AddCookie(&http.Cookie{Name: legacyOAuthStateCookieName, Value: "legacy123"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGitHubCallback_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		query      string
		cookies    []*http.Cookie
		setupH     func(*Handler)
		wantCode   int
		wantBody   string
		skipConfig bool
	}{
		{
			name:     "missing code or state",
			query:    "code=abc",
			wantCode: http.StatusBadRequest,
			wantBody: "missing code or state",
		},
		{
			name:     "missing state cookie",
			query:    "code=abc&state=xyz",
			wantCode: http.StatusBadRequest,
			wantBody: "missing oauth state cookie",
		},
		{
			name:     "state mismatch",
			query:    "code=abc&state=xyz",
			cookies:  []*http.Cookie{{Name: oauthStateCookieName, Value: "abc"}},
			wantCode: http.StatusBadRequest,
			wantBody: "invalid oauth state",
		},
		{
			name:    "exchange error",
			query:   "code=fail&state=xyz",
			cookies: []*http.Cookie{{Name: oauthStateCookieName, Value: "xyz"}},
			setupH: func(h *Handler) {
				h.exchangeCodeForToken = func(_ context.Context, _ string) (string, error) {
					return "", fmt.Errorf("exchange failed")
				}
			},
			wantCode: http.StatusBadGateway,
			wantBody: "oauth token exchange failed",
		},
		{
			name:       "config validation error",
			query:      "code=abc&state=xyz",
			skipConfig: true,
			wantCode:   http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler()
			if tc.skipConfig {
				h.authConfig.GitHubClientID = ""
			}
			if tc.setupH != nil {
				tc.setupH(h)
			}
			req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?"+tc.query, nil)
			for _, c := range tc.cookies {
				req.AddCookie(c)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Fatalf("expected %d, got %d", tc.wantCode, w.Code)
			}
			if tc.wantBody != "" && !strings.Contains(w.Body.String(), tc.wantBody) {
				t.Fatalf("expected body to contain %q, got %q", tc.wantBody, w.Body.String())
			}
		})
	}
}

func TestGitHubStatusWithCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/status", nil)
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "oauth-token"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGitHubStatus_Errors(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()

	// Missing token
	req := httptest.NewRequest(http.MethodGet, "/auth/github/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Invalid token
	h.validateToken = func(_ context.Context, _ string) error {
		return fmt.Errorf("invalid token")
	}
	req = httptest.NewRequest(http.MethodGet, "/auth/github/status", nil)
	req.AddCookie(&http.Cookie{Name: "ingitdb_github_token", Value: "bad-token"})
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGitHubLogoutClearsCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/github/logout", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	var found bool
	for _, cookie := range cookies {
		if cookie.Name == "ingitdb_github_token" {
			found = true
			if cookie.MaxAge != -1 {
				t.Fatalf("expected token cookie MaxAge=-1, got %d", cookie.MaxAge)
			}
		}
	}
	if !found {
		t.Fatal("expected cleared auth cookie in response")
	}
	if !strings.Contains(w.Body.String(), "Successfully logged out") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestListCollections_RequiresAuth(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestReadDefinitionFromGitHub_Success(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{files: map[string][]byte{
		".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
		"data/countries/.collection/countries.yaml": []byte(countryColDefYAML),
	}}
	def, err := readDefinitionFromGitHub(context.Background(), fr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := def.Collections["countries"]; !ok {
		t.Error("expected 'countries' collection in definition")
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

func TestCookieNames(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := cookieNames(req); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
	req.AddCookie(&http.Cookie{Name: "c1", Value: "v1"})
	req.AddCookie(&http.Cookie{Name: "c2", Value: "v2"})
	got := cookieNames(req)
	if !strings.Contains(got, "c1") || !strings.Contains(got, "c2") {
		t.Errorf("expected c1 and c2, got %q", got)
	}
}

func TestCRUDErrors(t *testing.T) {
	t.Parallel()

	setupH := func(t *testing.T) *Handler {
		h, _ := newTestHandler()
		return h
	}

	t.Run("list collections file reader error", func(t *testing.T) {
		h := setupH(t)
		h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return nil, fmt.Errorf("reader failed")
		}
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("list collections def read error", func(t *testing.T) {
		h := setupH(t)
		h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return &fakeFileReader{files: map[string][]byte{}}, nil
		}
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("read record file reader error", func(t *testing.T) {
		h := setupH(t)
		h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
			return nil, fmt.Errorf("reader failed")
		}
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("read record invalid key", func(t *testing.T) {
		h := setupH(t)
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=invalid-key-no-slash", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("read record db open error", func(t *testing.T) {
		h := setupH(t)
		h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
			return nil, fmt.Errorf("db fail")
		}
		req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("create record body read error", func(t *testing.T) {
		h := setupH(t)
		req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/new", &errorReader{})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("update record body read error", func(t *testing.T) {
		h := setupH(t)
		req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", &errorReader{})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

// ---------------------------------------------------------------------------
// NewHandlerWithAuth closure coverage
// ---------------------------------------------------------------------------

// TestNewHandlerWithAuth_ExchangeClosureBody exercises the exchangeCodeForToken
// closure created inside NewHandlerWithAuth so that the closure body is counted
// as covered.  We pass an empty code so ExchangeCodeForToken returns immediately
// with "code is required" (no network I/O).
func TestNewHandlerWithAuth_ExchangeClosureBody(t *testing.T) {
	t.Parallel()
	cfg := auth.Config{
		GitHubClientID:     "id",
		GitHubClientSecret: "secret",
		CallbackURL:        "http://localhost/cb",
	}
	h := NewHandlerWithAuth(cfg, false)
	_, err := h.exchangeCodeForToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty code, got nil")
	}
}

// TestNewHandlerWithAuth_ValidateClosureBody exercises the validateToken
// closure created inside NewHandlerWithAuth so that the closure body is counted
// as covered.  We pass an empty token so ValidateGitHubToken returns immediately
// with "token is required" (no network I/O).
func TestNewHandlerWithAuth_ValidateClosureBody(t *testing.T) {
	t.Parallel()
	h := NewHandlerWithAuth(auth.Config{}, false)
	err := h.validateToken(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

// ---------------------------------------------------------------------------
// randomOAuthState
// ---------------------------------------------------------------------------

func TestRandomOAuthState_RandError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.randRead = func(b []byte) (int, error) {
		return 0, fmt.Errorf("rand failure")
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to generate oauth state") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// oauthStateCookieNameForConfig
// ---------------------------------------------------------------------------

func TestOAuthStateCookieNameForConfig_Secure(t *testing.T) {
	t.Parallel()
	cfg := auth.Config{CookieSecure: true}
	got := oauthStateCookieNameForConfig(cfg)
	if got != oauthStateCookieName {
		t.Errorf("got %q, want %q", got, oauthStateCookieName)
	}
}

func TestOAuthStateCookieNameForConfig_Insecure(t *testing.T) {
	t.Parallel()
	cfg := auth.Config{CookieSecure: false}
	got := oauthStateCookieNameForConfig(cfg)
	if got != legacyOAuthStateCookieName {
		t.Errorf("got %q, want %q", got, legacyOAuthStateCookieName)
	}
}

// ---------------------------------------------------------------------------
// oauthStateCookieFromRequest
// ---------------------------------------------------------------------------

func TestOAuthStateCookieFromRequest_HostOnly(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "state-val"})
	cookie, name, err := oauthStateCookieFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != oauthStateCookieName {
		t.Errorf("got name %q, want %q", name, oauthStateCookieName)
	}
	if cookie.Value != "state-val" {
		t.Errorf("got value %q, want %q", cookie.Value, "state-val")
	}
}

func TestOAuthStateCookieFromRequest_Legacy(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: legacyOAuthStateCookieName, Value: "legacy-val"})
	cookie, name, err := oauthStateCookieFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != legacyOAuthStateCookieName {
		t.Errorf("got name %q, want %q", name, legacyOAuthStateCookieName)
	}
	if cookie.Value != "legacy-val" {
		t.Errorf("got value %q, want %q", cookie.Value, "legacy-val")
	}
}

func TestOAuthStateCookieFromRequest_Missing(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, err := oauthStateCookieFromRequest(req)
	if err != http.ErrNoCookie {
		t.Errorf("expected http.ErrNoCookie, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// githubLogin – insecure config uses legacy cookie name
// ---------------------------------------------------------------------------

func TestGitHubLoginRedirect_InsecureCookie(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.authConfig.CookieSecure = false
	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	setCookieHeader := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookieHeader, legacyOAuthStateCookieName) {
		t.Errorf("expected legacy cookie name in Set-Cookie, got %q", setCookieHeader)
	}
}

// ---------------------------------------------------------------------------
// authenticatedToken – requireAuth=true paths
// ---------------------------------------------------------------------------

func TestAuthenticatedToken_RequireAuthValidToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	// validateToken is the no-op fake, so we should reach listCollections successfully.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthenticatedToken_RequireAuthInvalidToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	h.validateToken = func(_ context.Context, _ string) error {
		return fmt.Errorf("token rejected")
	}
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/collections?db=owner/repo", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "token rejected") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// readDefinitionFromGitHub – all remaining error paths
// ---------------------------------------------------------------------------

func TestReadDefinitionFromGitHub_RootReadError(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{},
		readErrors: map[string]error{
			".ingitdb/root-collections.yaml": fmt.Errorf("io error"),
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_RootYAMLInvalid(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			// nested map cannot unmarshal into map[string]string
			".ingitdb/root-collections.yaml": []byte("countries:\n  nested: value\n"),
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_CollectionReadError(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml": []byte(rootConfigYAML),
		},
		readErrors: map[string]error{
			"data/countries/.collection/countries.yaml": fmt.Errorf("col io error"),
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read collection def") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_CollectionNotFound(t *testing.T) {
	t.Parallel()
	// root config present, but collection def file is absent
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml": []byte(rootConfigYAML),
			// collection def intentionally omitted
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "collection definition not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_CollectionYAMLInvalid(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
			"data/countries/.collection/countries.yaml": []byte("key: [\n"), // broken YAML
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse collection def") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_SubscribersReadError(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
			"data/countries/.collection/countries.yaml": []byte(countryColDefYAML),
		},
		readErrors: map[string]error{
			".ingitdb/subscribers.yaml": fmt.Errorf("subscribers io error"),
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read subscribers config") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_SubscribersYAMLInvalid(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
			"data/countries/.collection/countries.yaml": []byte(countryColDefYAML),
			".ingitdb/subscribers.yaml":                 []byte("key: [\n"), // broken YAML
		},
	}
	_, err := readDefinitionFromGitHub(context.Background(), fr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse subscribers config") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadDefinitionFromGitHub_SubscribersPresent(t *testing.T) {
	t.Parallel()
	fr := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml":            []byte(rootConfigYAML),
			"data/countries/.collection/countries.yaml": []byte(countryColDefYAML),
			".ingitdb/subscribers.yaml": []byte("subscribers:\n" +
				"  webhook:\n" +
				"    url: https://example.com/hook\n"),
		},
	}
	def, err := readDefinitionFromGitHub(context.Background(), fr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.Subscribers) == 0 {
		t.Error("expected subscribers to be populated")
	}
}

// ---------------------------------------------------------------------------
// readRecord – additional error paths
// ---------------------------------------------------------------------------

func TestReadRecord_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReadRecord_DefReadError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return &fakeFileReader{files: map[string][]byte{}}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestReadRecord_TxError(t *testing.T) {
	t.Parallel()
	s := newFakeStore(map[string]map[string]any{})
	db := &fakeDB{s: s, readTxErr: fmt.Errorf("tx boom")}
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return db, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to read record") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// createRecord – additional error paths
// ---------------------------------------------------------------------------

func TestCreateRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRecord_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRecord_FileReaderError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return nil, fmt.Errorf("reader fail")
	}
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRecord_DefReadError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return &fakeFileReader{files: map[string][]byte{}}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRecord_InvalidKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=nonexistent/rec",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateRecord_DBOpenError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return nil, fmt.Errorf("db open fail")
	}
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestCreateRecord_TxError(t *testing.T) {
	t.Parallel()
	s := newFakeStore(map[string]map[string]any{})
	db := &fakeDB{s: s, writeTxErr: fmt.Errorf("insert fail")}
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return db, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to create record") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// updateRecord – additional error paths
// ---------------------------------------------------------------------------

func TestUpdateRecord_MissingKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateRecord_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?key=countries/ie",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateRecord_InvalidJSON(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateRecord_FileReaderError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return nil, fmt.Errorf("reader fail")
	}
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestUpdateRecord_DefReadError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return &fakeFileReader{files: map[string][]byte{}}, nil
	}
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestUpdateRecord_InvalidKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=nonexistent/rec",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateRecord_DBOpenError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return nil, fmt.Errorf("db open fail")
	}
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// TestUpdateRecord_TxGetError covers the path where tx.Get() returns a
// non-record-not-found error inside the update transaction.
func TestUpdateRecord_TxGetError(t *testing.T) {
	t.Parallel()
	s := newFakeStore(map[string]map[string]any{
		"countries/ie": {"title": "Ireland"},
	})
	db := &fakeDB{s: s, getTxErr: fmt.Errorf("get boom")}
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return db, nil
	}
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"Updated"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to update record") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// TestUpdateRecord_TxSetError covers the path where tx.Set() returns an error
// after a successful Get (record exists).
func TestUpdateRecord_TxSetError(t *testing.T) {
	t.Parallel()
	s := newFakeStore(map[string]map[string]any{
		"countries/ie": {"title": "Ireland"},
	})
	db := &fakeDB{s: s, setErr: fmt.Errorf("set boom")}
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return db, nil
	}
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"Updated"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to update record") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// deleteRecord – additional error paths
// ---------------------------------------------------------------------------

func TestDeleteRecord_MissingDB(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteRecord_FileReaderError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return nil, fmt.Errorf("reader fail")
	}
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDeleteRecord_DefReadError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubFileReader = func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
		return &fakeFileReader{files: map[string][]byte{}}, nil
	}
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDeleteRecord_InvalidKey(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=nonexistent/rec", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDeleteRecord_DBOpenError(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return nil, fmt.Errorf("db open fail")
	}
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDeleteRecord_TxError(t *testing.T) {
	t.Parallel()
	s := newFakeStore(map[string]map[string]any{
		"countries/ie": {"title": "Ireland"},
	})
	db := &fakeDB{s: s, writeTxErr: fmt.Errorf("delete fail")}
	h, _ := newTestHandler()
	h.newGitHubDBWithDef = func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
		return db, nil
	}
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "failed to delete record") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// requireAuth=true + no token: covers the `if !ok { return }` branch in each
// handler (each handler's branch is counted separately by the coverage tool).
// ---------------------------------------------------------------------------

func TestReadRecord_RequireAuthNoToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodGet, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateRecord_RequireAuthNoToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodPost, "/ingitdb/v0/record?db=owner/repo&key=countries/de",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdateRecord_RequireAuthNoToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodPut, "/ingitdb/v0/record?db=owner/repo&key=countries/ie",
		strings.NewReader(`{"title":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteRecord_RequireAuthNoToken(t *testing.T) {
	t.Parallel()
	h, _ := newTestHandler()
	h.requireAuth = true
	req := httptest.NewRequest(http.MethodDelete, "/ingitdb/v0/record?db=owner/repo&key=countries/ie", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
