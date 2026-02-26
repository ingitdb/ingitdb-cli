// Package mcp implements the MCP (Model Context Protocol) HTTP server for
// mcp.ingitdb.com. It exposes inGitDB tools over a stateless HTTP endpoint
// using a synchronous request/response transport backed by GitHub.
package mcp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/julienschmidt/httprouter"
	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"gopkg.in/yaml.v3"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"github.com/ingitdb/ingitdb-cli/server/auth"
)

//go:embed index.html
var indexHTML []byte

// Handler is the HTTP handler for the MCP server.
type Handler struct {
	newGitHubFileReader func(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error)
	newGitHubDBWithDef  func(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error)
	authConfig          auth.Config
	validateToken       func(ctx context.Context, token string) error
	requireAuth         bool
	router              *httprouter.Router
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
	r.GET("/auth/github/login", h.redirectToAPILogin)
	r.POST("/mcp", h.handleMCP)
	return r
}

// serveIndex serves the MCP index.html file.
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	_ = r
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(indexHTML)
}

func (h *Handler) redirectToAPILogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	loginURL := strings.TrimRight(h.authConfig.AuthAPIBaseURL, "/") + "/auth/github/login"
	http.Redirect(w, r, loginURL, http.StatusFound)
}

// singleRequestTransport is a synchronous, in-memory MCP transport used for
// stateless HTTP request/response handling. Each HTTP POST creates one instance.
type singleRequestTransport struct {
	msgHandler func(ctx context.Context, msg *transport.BaseJsonRpcMessage)
	respCh     chan *transport.BaseJsonRpcMessage
}

func newSingleRequestTransport() *singleRequestTransport {
	return &singleRequestTransport{
		respCh: make(chan *transport.BaseJsonRpcMessage, 1),
	}
}

func (t *singleRequestTransport) Start(_ context.Context) error { return nil }

func (t *singleRequestTransport) Send(_ context.Context, msg *transport.BaseJsonRpcMessage) error {
	t.respCh <- msg
	return nil
}

func (t *singleRequestTransport) Close() error { return nil }

func (t *singleRequestTransport) SetCloseHandler(_ func()) {}

func (t *singleRequestTransport) SetErrorHandler(_ func(error)) {}

func (t *singleRequestTransport) SetMessageHandler(handler func(ctx context.Context, msg *transport.BaseJsonRpcMessage)) {
	t.msgHandler = handler
}

// MCP tool argument types for cloud (GitHub-backed) tools.

type listCollectionsArgs struct {
	DB string `json:"db" jsonschema:"required,description=GitHub repository in owner/repo format"`
}

type createRecordArgs struct {
	DB   string `json:"db"   jsonschema:"required,description=GitHub repository in owner/repo format"`
	ID   string `json:"id"   jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie)"`
	Data string `json:"data" jsonschema:"required,description=Record data as JSON (e.g. {\"title\":\"Ireland\"})"`
}

type readRecordArgs struct {
	DB string `json:"db" jsonschema:"required,description=GitHub repository in owner/repo format"`
	ID string `json:"id" jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie)"`
}

type updateRecordArgs struct {
	DB     string `json:"db"     jsonschema:"required,description=GitHub repository in owner/repo format"`
	ID     string `json:"id"     jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie)"`
	Fields string `json:"fields" jsonschema:"required,description=Fields to update as JSON (e.g. {\"title\":\"New Name\"})"`
}

type deleteRecordArgs struct {
	DB string `json:"db" jsonschema:"required,description=GitHub repository in owner/repo format"`
	ID string `json:"id" jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie)"`
}

// parseDBArg parses an "owner/repo" string into owner and repo parts.
func parseDBArg(db string) (owner, repo string, err error) {
	if db == "" {
		return "", "", fmt.Errorf("db cannot be empty")
	}
	parts := strings.SplitN(db, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid db %q: expected owner/repo", db)
	}
	return parts[0], parts[1], nil
}

// readDefinitionFromGitHub reads the inGitDB definition (all collections) from
// a GitHub repository using the provided FileReader.
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

// registerMCPTools registers the inGitDB CRUD tools on the MCP server.
func (h *Handler) registerMCPTools(server *mcp_golang.Server, token string) error {
	if err := server.RegisterTool(
		"list_collections",
		"List collections in an inGitDB GitHub repository",
		func(ctx context.Context, args listCollectionsArgs) (*mcp_golang.ToolResponse, error) {
			owner, repo, err := parseDBArg(args.DB)
			if err != nil {
				return nil, err
			}
			cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
			fileReader, err := h.newGitHubFileReader(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create file reader: %w", err)
			}
			def, err := readDefinitionFromGitHub(ctx, fileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read definition: %w", err)
			}
			ids := make([]string, 0, len(def.Collections))
			for id := range def.Collections {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			out, err := yaml.Marshal(ids)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal collections: %w", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(out))), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register list_collections: %w", err)
	}

	if err := server.RegisterTool(
		"read_record",
		"Read a single record by its ID from an inGitDB GitHub repository, returns JSON",
		func(ctx context.Context, args readRecordArgs) (*mcp_golang.ToolResponse, error) {
			owner, repo, err := parseDBArg(args.DB)
			if err != nil {
				return nil, err
			}
			cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
			fileReader, err := h.newGitHubFileReader(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create file reader: %w", err)
			}
			def, err := readDefinitionFromGitHub(ctx, fileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			db, err := h.newGitHubDBWithDef(cfg, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
			data := map[string]any{}
			record := dal.NewRecordWithData(dalKey, data)
			if err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
				return tx.Get(ctx, record)
			}); err != nil {
				return nil, err
			}
			if !record.Exists() {
				return nil, fmt.Errorf("record not found: %s", args.ID)
			}
			out, err := json.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal record: %w", err)
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent(string(out))), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register read_record: %w", err)
	}

	if err := server.RegisterTool(
		"create_record",
		"Create a new record in an inGitDB GitHub repository",
		func(ctx context.Context, args createRecordArgs) (*mcp_golang.ToolResponse, error) {
			owner, repo, err := parseDBArg(args.DB)
			if err != nil {
				return nil, err
			}
			cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
			fileReader, err := h.newGitHubFileReader(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create file reader: %w", err)
			}
			def, err := readDefinitionFromGitHub(ctx, fileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			var data map[string]any
			if err = json.Unmarshal([]byte(args.Data), &data); err != nil {
				return nil, fmt.Errorf("failed to parse data: %w", err)
			}
			db, err := h.newGitHubDBWithDef(cfg, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
			record := dal.NewRecordWithData(dalKey, data)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			}); err != nil {
				return nil, err
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent("record created: " + args.ID)), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register create_record: %w", err)
	}

	if err := server.RegisterTool(
		"update_record",
		"Update fields of an existing record in an inGitDB GitHub repository",
		func(ctx context.Context, args updateRecordArgs) (*mcp_golang.ToolResponse, error) {
			owner, repo, err := parseDBArg(args.DB)
			if err != nil {
				return nil, err
			}
			cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
			fileReader, err := h.newGitHubFileReader(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create file reader: %w", err)
			}
			def, err := readDefinitionFromGitHub(ctx, fileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			var patch map[string]any
			if err = json.Unmarshal([]byte(args.Fields), &patch); err != nil {
				return nil, fmt.Errorf("failed to parse fields: %w", err)
			}
			db, err := h.newGitHubDBWithDef(cfg, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				data := map[string]any{}
				record := dal.NewRecordWithData(dalKey, data)
				if getErr := tx.Get(ctx, record); getErr != nil {
					return getErr
				}
				if !record.Exists() {
					return fmt.Errorf("record not found: %s", args.ID)
				}
				maps.Copy(data, patch)
				return tx.Set(ctx, record)
			}); err != nil {
				return nil, err
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent("record updated: " + args.ID)), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register update_record: %w", err)
	}

	if err := server.RegisterTool(
		"delete_record",
		"Delete a record by its ID from an inGitDB GitHub repository",
		func(ctx context.Context, args deleteRecordArgs) (*mcp_golang.ToolResponse, error) {
			owner, repo, err := parseDBArg(args.DB)
			if err != nil {
				return nil, err
			}
			cfg := dalgo2ghingitdb.Config{Owner: owner, Repo: repo, Token: token}
			fileReader, err := h.newGitHubFileReader(cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create file reader: %w", err)
			}
			def, err := readDefinitionFromGitHub(ctx, fileReader)
			if err != nil {
				return nil, fmt.Errorf("failed to read definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			db, err := h.newGitHubDBWithDef(cfg, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			dalKey := dal.NewKeyWithID(colDef.ID, recordKey)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Delete(ctx, dalKey)
			}); err != nil {
				return nil, err
			}
			return mcp_golang.NewToolResponse(mcp_golang.NewTextContent("record deleted: " + args.ID)), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register delete_record: %w", err)
	}

	return nil
}

// handleMCP processes a single MCP JSON-RPC request over HTTP POST /mcp.
func (h *Handler) handleMCP(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !h.authorize(w, r) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	var rawReq transport.BaseJSONRPCRequest
	if err = json.Unmarshal(body, &rawReq); err != nil {
		http.Error(w, "invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	tr := newSingleRequestTransport()
	server := mcp_golang.NewServer(tr, mcp_golang.WithName("ingitdb"), mcp_golang.WithVersion("1.0"))
	if err = h.registerMCPTools(server, githubToken(r)); err != nil {
		http.Error(w, fmt.Sprintf("failed to register tools: %v", err), http.StatusInternalServerError)
		return
	}
	if err = server.Serve(); err != nil {
		http.Error(w, fmt.Sprintf("failed to start MCP server: %v", err), http.StatusInternalServerError)
		return
	}

	// Deliver the incoming request to the MCP server's message handler.
	if tr.msgHandler != nil {
		tr.msgHandler(r.Context(), transport.NewBaseMessageRequest(&rawReq))
	}

	// Wait for the response from the MCP server.
	select {
	case resp := <-tr.respCh:
		w.Header().Set("Content-Type", "application/json")
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
		}
	case <-r.Context().Done():
		http.Error(w, "request timed out", http.StatusGatewayTimeout)
	}
}

func (h *Handler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if !h.requireAuth {
		return true
	}
	token := auth.ResolveTokenFromRequest(r, h.authConfig.CookieName)
	if token == "" {
		http.Error(w, "missing github token", http.StatusUnauthorized)
		return false
	}
	if err := h.validateToken(r.Context(), token); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return false
	}
	return true
}

// githubToken extracts a bearer token from the Authorization header.
func githubToken(r *http.Request) string {
	return auth.ResolveTokenFromRequest(r, auth.LoadConfigFromEnv().CookieName)
}
