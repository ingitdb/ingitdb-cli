// Package api implements the REST API server for api.ingitdb.com.
package api

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"github.com/ingitdb/ingitdb-cli/server/auth"
)

//go:embed index.html
var indexHTML []byte

// Handler is the HTTP handler for the API server. Fields can be replaced in
// tests to inject mock implementations.
type Handler struct {
	newGitHubFileReader  func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error)
	newGitHubDBWithDef   func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error)
	authConfig           auth.Config
	exchangeCodeForToken func(ctx context.Context, code string) (string, error)
	validateToken        func(ctx context.Context, token string) error
	requireAuth          bool
	router               *httprouter.Router
}

// NewHandler creates a Handler with the default (production) GitHub implementations.
func NewHandler() *Handler {
	cfg := auth.LoadConfigFromEnv()
	return NewHandlerWithAuth(cfg, true)
}

// NewHandlerWithAuth creates a handler with provided auth configuration and mode.
func NewHandlerWithAuth(cfg auth.Config, requireAuth bool) *Handler {
	h := &Handler{
		newGitHubFileReader: dalgo2ghingitdb.NewGitHubFileReader,
		newGitHubDBWithDef:  dalgo2ghingitdb.NewGitHubDBWithDef,
		authConfig:          cfg,
		exchangeCodeForToken: func(ctx context.Context, code string) (string, error) {
			return cfg.ExchangeCodeForToken(ctx, code, nil)
		},
		validateToken: func(ctx context.Context, token string) error {
			return auth.ValidateGitHubToken(ctx, token, nil)
		},
		requireAuth: requireAuth,
	}
	h.router = h.buildRouter()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

func (h *Handler) buildRouter() *httprouter.Router {
	r := httprouter.New()
	r.GET("/", h.serveIndex)
	r.GET("/auth/github/login", h.githubLogin)
	r.GET("/auth/github/logout", h.githubLogout)
	r.GET("/auth/github/callback", h.githubCallback)
	r.GET("/auth/github/status", h.githubStatus)
	r.GET("/ingitdb/v0/collections", h.listCollections)
	r.GET("/ingitdb/v0/record", h.readRecord)
	r.POST("/ingitdb/v0/record", h.createRecord)
	r.PUT("/ingitdb/v0/record", h.updateRecord)
	r.DELETE("/ingitdb/v0/record", h.deleteRecord)
	return r
}

const (
	oauthStateCookieName       = "__Host-ingitdb_oauth_state"
	legacyOAuthStateCookieName = "ingitdb_oauth_state"
)

// serveIndex serves the API index.html file.
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	_ = r
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

// parseDBParam parses the "db" query parameter as "owner/repo".
func parseDBParam(r *http.Request) (owner, repo string, err error) {
	db := r.URL.Query().Get("db")
	if db == "" {
		return "", "", fmt.Errorf("missing required query parameter: db")
	}
	parts := strings.SplitN(db, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid db parameter %q: expected owner/repo", db)
	}
	return parts[0], parts[1], nil
}

// githubToken extracts token from the Authorization header or auth cookie.
func githubToken(r *http.Request) string {
	return auth.ResolveTokenFromRequest(r, auth.LoadConfigFromEnv().CookieName)
}

// writeJSON writes v as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an error response as JSON.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomOAuthState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func oauthStateCookieNameForConfig(cfg auth.Config) string {
	if cfg.CookieSecure {
		return oauthStateCookieName
	}
	return legacyOAuthStateCookieName
}

func oauthStateCookieFromRequest(r *http.Request) (*http.Cookie, string, error) {
	names := []string{oauthStateCookieName, legacyOAuthStateCookieName}
	for _, name := range names {
		cookie, err := r.Cookie(name)
		if err == nil {
			return cookie, name, nil
		}
		if err != http.ErrNoCookie {
			return nil, "", err
		}
	}
	return nil, "", http.ErrNoCookie
}

func cookieNames(r *http.Request) string {
	cookies := r.Cookies()
	if len(cookies) == 0 {
		return ""
	}
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return strings.Join(names, ",")
}

func logOAuthEvent(r *http.Request, event, details string) {
	_, _ = fmt.Fprintf(os.Stderr, "oauth event=%s host=%s path=%s details=%s\n", event, r.Host, r.URL.Path, details)
}

func (h *Handler) githubLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := h.authConfig.ValidateForHTTPMode(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	state, err := randomOAuthState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stateCookieName := oauthStateCookieNameForConfig(h.authConfig)
	stateCookie := &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	}
	http.SetCookie(w, stateCookie)
	stateLen := strconv.Itoa(len(state))
	logOAuthEvent(r, "login_set_state", "cookie="+stateCookieName+" state_len="+stateLen)
	http.Redirect(w, r, h.authConfig.AuthorizeURL(state), http.StatusFound)
}

func (h *Handler) githubCallback(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := h.authConfig.ValidateForHTTPMode(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	codeLen := strconv.Itoa(len(code))
	stateLen := strconv.Itoa(len(state))
	logOAuthEvent(r, "callback_received", "code_len="+codeLen+" state_len="+stateLen)
	if code == "" || state == "" {
		logOAuthEvent(r, "callback_missing_query", "code_or_state_missing=true")
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}
	stateCookie, stateCookieSource, err := oauthStateCookieFromRequest(r)
	if err != nil {
		logOAuthEvent(r, "callback_missing_state_cookie", "cookies="+cookieNames(r))
		writeError(w, http.StatusBadRequest, "missing oauth state cookie")
		return
	}
	if state != stateCookie.Value {
		cookieStateLen := strconv.Itoa(len(stateCookie.Value))
		logOAuthEvent(r, "callback_state_mismatch", "cookie="+stateCookieSource+" query_state_len="+stateLen+" cookie_state_len="+cookieStateLen)
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	token, err := h.exchangeCodeForToken(r.Context(), code)
	if err != nil {
		logOAuthEvent(r, "callback_token_exchange_failed", err.Error())
		writeError(w, http.StatusBadGateway, fmt.Sprintf("oauth token exchange failed: %v", err))
		return
	}
	logOAuthEvent(r, "callback_token_exchange_success", "cookie="+stateCookieSource)
	tokenCookie := &http.Cookie{
		Name:     h.authConfig.CookieName,
		Value:    token,
		Path:     "/",
		Domain:   h.authConfig.CookieDomain,
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	}
	http.SetCookie(w, tokenCookie)
	clearStateCookie := &http.Cookie{
		Name:     stateCookieSource,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearStateCookie)
	// Backward compatibility for previously issued domain-scoped legacy state cookie.
	clearLegacyStateCookie := &http.Cookie{
		Name:     legacyOAuthStateCookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.authConfig.CookieDomain,
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearLegacyStateCookie)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<html><body><h1>Successfully authenticated</h1><p><a href="/auth/github/status">Check authentication status</a></p></body></html>`))
}

func (h *Handler) githubLogout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	clearTokenCookie := &http.Cookie{
		Name:     h.authConfig.CookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.authConfig.CookieDomain,
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearTokenCookie)
	clearStateCookie := &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearStateCookie)
	clearLegacyHostOnlyStateCookie := &http.Cookie{
		Name:     legacyOAuthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearLegacyHostOnlyStateCookie)
	clearLegacyStateCookie := &http.Cookie{
		Name:     legacyOAuthStateCookieName,
		Value:    "",
		Path:     "/",
		Domain:   h.authConfig.CookieDomain,
		HttpOnly: true,
		Secure:   h.authConfig.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, clearLegacyStateCookie)
	logOAuthEvent(r, "logout_cleared_cookies", "token_cookie="+h.authConfig.CookieName)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<html><body><h1>Successfully logged out</h1><p><a href="/auth/github/login">Authenticate again</a></p></body></html>`))
}

func (h *Handler) githubStatus(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token := auth.ResolveTokenFromRequest(r, h.authConfig.CookieName)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing github token")
		return
	}
	if err := h.validateToken(r.Context(), token); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

func (h *Handler) authenticatedToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !h.requireAuth {
		token := auth.ResolveTokenFromRequest(r, h.authConfig.CookieName)
		return token, true
	}
	token := auth.ResolveTokenFromRequest(r, h.authConfig.CookieName)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing github token")
		return "", false
	}
	if err := h.validateToken(r.Context(), token); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return "", false
	}
	return token, true
}

// readDefinitionFromGitHub reads the inGitDB definition from a GitHub repository.
func readDefinitionFromGitHub(ctx context.Context, fileReader dalgo2ghingitdb.FileReader) (*ingitdb.Definition, error) {
	rootCollectionsPath := path.Join(config.IngitDBDirName, config.RootCollectionsFileName)
	rootConfigContent, found, err := fileReader.ReadFile(ctx, rootCollectionsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", rootCollectionsPath, err)
	}
	if !found {
		return nil, fmt.Errorf("file not found: %s", rootCollectionsPath)
	}
	var rootCollections map[string]string
	if err = yaml.Unmarshal(rootConfigContent, &rootCollections); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", rootCollectionsPath, err)
	}
	rootConfig := config.RootConfig{RootCollections: rootCollections}
	def := &ingitdb.Definition{Collections: make(map[string]*ingitdb.CollectionDef)}
	for id, colPath := range rootConfig.RootCollections {
		colDefPath := path.Join(colPath, ingitdb.SchemaDir, id+".yaml")
		colDefContent, colFound, readErr := fileReader.ReadFile(ctx, colDefPath)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read collection def %s: %w", colDefPath, readErr)
		}
		if !colFound {
			return nil, fmt.Errorf("collection definition not found: %s", colDefPath)
		}
		colDef := &ingitdb.CollectionDef{}
		if unmarshalErr := yaml.Unmarshal(colDefContent, colDef); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse collection def %s: %w", colDefPath, unmarshalErr)
		}
		colDef.ID = id
		colDef.DirPath = path.Clean(colPath)
		def.Collections[id] = colDef
	}
	subscribersContent, subFound, subErr := fileReader.ReadFile(ctx, config.SubscribersConfigFileName)
	if subErr != nil {
		return nil, fmt.Errorf("failed to read subscribers config %s: %w", config.SubscribersConfigFileName, subErr)
	}
	if subFound {
		var subCfg config.SubscribersConfig
		if err = yaml.Unmarshal(subscribersContent, &subCfg); err != nil {
			return nil, fmt.Errorf("failed to parse subscribers config %s: %w", config.SubscribersConfigFileName, err)
		}
		def.Subscribers = subCfg.Subscribers
	}
	return def, nil
}

// listCollections handles GET /ingitdb/v0/collections?db=owner/repo
func (h *Handler) listCollections(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token, ok := h.authenticatedToken(w, r)
	if !ok {
		return
	}
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	ids := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writeJSON(w, http.StatusOK, ids)
}

// readRecord handles GET /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) readRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token, ok := h.authenticatedToken(w, r)
	if !ok {
		return
	}
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	data := map[string]any{}
	record := dal.NewRecordWithData(dalKey, data)
	if err = db.RunReadonlyTransaction(r.Context(), func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read record: %v", err))
		return
	}
	if !record.Exists() {
		writeError(w, http.StatusNotFound, fmt.Sprintf("record not found: %s", key))
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// createRecord handles POST /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token, ok := h.authenticatedToken(w, r)
	if !ok {
		return
	}
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	var data map[string]any
	if err = json.Unmarshal(body, &data); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	record := dal.NewRecordWithData(dalKey, data)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create record: %v", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"key": key})
}

// updateRecord handles PUT /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token, ok := h.authenticatedToken(w, r)
	if !ok {
		return
	}
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to read request body: %v", err))
		return
	}
	var patch map[string]any
	if err = json.Unmarshal(body, &patch); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		data := map[string]any{}
		record := dal.NewRecordWithData(dalKey, data)
		if getErr := tx.Get(ctx, record); getErr != nil {
			return getErr
		}
		if !record.Exists() {
			return fmt.Errorf("record not found: %s", key)
		}
		maps.Copy(data, patch)
		return tx.Set(ctx, record)
	}); err != nil {
		if strings.Contains(err.Error(), "record not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update record: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}

// deleteRecord handles DELETE /v0/record?db=owner/repo&key=col/record_id
func (h *Handler) deleteRecord(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	token, ok := h.authenticatedToken(w, r)
	if !ok {
		return
	}
	owner, repo, err := parseDBParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "missing required query parameter: key")
		return
	}
	cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
	fileReader, err := h.newGitHubFileReader(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create file reader: %v", err))
		return
	}
	def, err := readDefinitionFromGitHub(r.Context(), fileReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read definition: %v", err))
		return
	}
	colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, key)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid key: %v", err))
		return
	}
	db, err := h.newGitHubDBWithDef(cfg, def)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open database: %v", err))
		return
	}
	dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
	if err = db.RunReadwriteTransaction(r.Context(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dalKey)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete record: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}
