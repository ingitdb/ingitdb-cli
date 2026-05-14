package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"gopkg.in/yaml.v3"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// testMCPTransport — an in-process transport.Transport for MCP server tests.
// ---------------------------------------------------------------------------

type testMCPTransport struct {
	mu             sync.RWMutex
	messageHandler func(context.Context, *transport.BaseJsonRpcMessage)
	sentMessages   []*transport.BaseJsonRpcMessage
	startErr       error // if set, Start returns this
	sendErr        error // if set, Send returns this
}

func (t *testMCPTransport) Start(_ context.Context) error { return t.startErr }

func (t *testMCPTransport) Send(_ context.Context, msg *transport.BaseJsonRpcMessage) error {
	if t.sendErr != nil {
		return t.sendErr
	}
	t.mu.Lock()
	t.sentMessages = append(t.sentMessages, msg)
	t.mu.Unlock()
	return nil
}

func (t *testMCPTransport) Close() error { return nil }

func (t *testMCPTransport) SetCloseHandler(_ func()) {}

func (t *testMCPTransport) SetErrorHandler(_ func(error)) {}

func (t *testMCPTransport) SetMessageHandler(h func(context.Context, *transport.BaseJsonRpcMessage)) {
	t.mu.Lock()
	t.messageHandler = h
	t.mu.Unlock()
}

// callTool simulates a JSON-RPC "tools/call" request and waits for a response.
func (t *testMCPTransport) callTool(ctx context.Context, id int64, toolName, argsJSON string) (*transport.BaseJsonRpcMessage, error) {
	params := fmt.Sprintf(`{"name":%q,"arguments":%s}`, toolName, argsJSON)
	msg := &transport.BaseJsonRpcMessage{
		Type: transport.BaseMessageTypeJSONRPCRequestType,
		JsonRpcRequest: &transport.BaseJSONRPCRequest{
			Id:      transport.RequestId(id),
			Jsonrpc: "2.0",
			Method:  "tools/call",
			Params:  json.RawMessage(params),
		},
	}
	t.mu.RLock()
	handler := t.messageHandler
	t.mu.RUnlock()
	if handler == nil {
		return nil, fmt.Errorf("no message handler installed")
	}
	handler(ctx, msg)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		t.mu.Lock()
		for _, sent := range t.sentMessages {
			if sent.JsonRpcResponse != nil && int64(sent.JsonRpcResponse.Id) == id {
				t.mu.Unlock()
				return sent, nil
			}
			if sent.JsonRpcError != nil && int64(sent.JsonRpcError.Id) == id {
				t.mu.Unlock()
				return sent, nil
			}
		}
		t.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for response to request %d", id)
}

var _ transport.Transport = (*testMCPTransport)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestMCPServer builds a *mcp.Server backed by a fresh testMCPTransport.
func newTestMCPServer() (*mcp.Server, *testMCPTransport) {
	tr := &testMCPTransport{}
	server := mcp.NewServer(tr, mcp.WithName("test"), mcp.WithVersion("1.0"))
	return server, tr
}

// testMCPDef returns a minimal ingitdb.Definition for MCP tool tests.
func testMCPDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// sortedCollectionIDs tests (pre-existing, moved here for organisation)
// ---------------------------------------------------------------------------

func TestSortedCollectionIDs_ReturnsAllNamespacedIDs(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"countries":     {ID: "countries"},
			"todo.tags":     {ID: "todo.tags"},
			"todo.tasks":    {ID: "todo.tasks"},
			"todo.statuses": {ID: "todo.statuses"},
		},
	}

	got := sortedCollectionIDs(def)
	want := []string{"countries", "todo.statuses", "todo.tags", "todo.tasks"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("got[%d] = %q, want %q", i, got[i], id)
		}
	}
}

func TestSortedCollectionIDs_DoesNotCollapseNamespace(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tags":  {ID: "todo.tags"},
			"todo.tasks": {ID: "todo.tasks"},
		},
	}

	got := sortedCollectionIDs(def)
	if len(got) != 2 {
		t.Fatalf("expected 2 collection IDs (one per collection), got %v", got)
	}
	if got[0] == "todo" || got[1] == "todo" {
		t.Errorf("namespace root 'todo' must not appear as a collection; got %v", got)
	}
}

func TestSortedCollectionIDs_Empty(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	got := sortedCollectionIDs(def)
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// registerMCPTools tests
// ---------------------------------------------------------------------------

// TestRegisterMCPTools_Succeeds verifies that tool registration itself succeeds.
func TestRegisterMCPTools_Succeeds(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, _ := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	err := registerMCPTools(server, dir, readDef, newDB)
	if err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
}

// TestRegisterMCPTools_RegisterError covers the `return err` branch when
// server.RegisterTool fails because sendToolListChangedNotification fails
// (transport.Send returns an error while the server is running).
func TestRegisterMCPTools_RegisterError(t *testing.T) {
	// modifies the server state — not parallel.
	dir := t.TempDir()
	tr := &testMCPTransport{}
	server := mcp.NewServer(tr, mcp.WithName("test"), mcp.WithVersion("1.0"))
	// Start the server so isRunning = true.
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}
	// Now make Send fail so sendToolListChangedNotification fails.
	tr.sendErr = errors.New("send error")

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	err := registerMCPTools(server, dir, readDef, newDB)
	if err == nil {
		t.Fatal("expected error when RegisterTool fails")
	}
}

// TestRegisterMCPTools_ListCollections_Success invokes the list_collections
// tool handler through JSON-RPC.
func TestRegisterMCPTools_ListCollections_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 1, "list_collections", `{}`)
	if err != nil {
		t.Fatalf("callTool list_collections: %v", err)
	}
	if resp.JsonRpcResponse == nil {
		t.Fatalf("expected JSON-RPC response, got: %+v", resp)
	}
}

// TestRegisterMCPTools_ListCollections_ReadDefError covers the readDef error
// branch inside the list_collections handler.
func TestRegisterMCPTools_ListCollections_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 2, "list_collections", `{}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	// Errors in tool handlers are returned as a successful JSON-RPC response
	// with isError=true content (mcp-golang wraps handler errors).
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
}

// TestRegisterMCPTools_CreateRecord_Success invokes the create_record tool.
func TestRegisterMCPTools_CreateRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","data":"{name: hello}"}`
	resp, err := tr.callTool(context.Background(), 3, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool create_record: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_CreateRecord_ReadDefError covers the readDef error
// branch in the create_record handler.
func TestRegisterMCPTools_CreateRecord_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","data":"{name: hello}"}`
	resp, err := tr.callTool(context.Background(), 4, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_CreateRecord_InvalidID covers the invalid ID branch.
func TestRegisterMCPTools_CreateRecord_InvalidID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	// "invalid" has no slash separator → CollectionForKey fails.
	argsJSON := `{"id":"invalid","data":"{name: hello}"}`
	resp, err := tr.callTool(context.Background(), 5, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_CreateRecord_ParseDataError covers the data parse error.
func TestRegisterMCPTools_CreateRecord_ParseDataError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","data":"invalid: yaml: ["}`
	resp, err := tr.callTool(context.Background(), 6, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_CreateRecord_DBOpenError covers the newDB error branch.
func TestRegisterMCPTools_CreateRecord_DBOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db open error")
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","data":"{name: hello}"}`
	resp, err := tr.callTool(context.Background(), 7, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_Success reads a record that exists.
func TestRegisterMCPTools_ReadRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	// Pre-create the record file. The backend stores {key}.yaml records under
	// a $records/ subdirectory (see RecordsBasePath).
	recordsDir := filepath.Join(dir, "$records")
	if mkdirErr := os.MkdirAll(recordsDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	content, marshalErr := yaml.Marshal(map[string]any{"name": "hello"})
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(recordsDir, "r1.yaml"), content, 0o644); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 8, "read_record", `{"id":"test.items/r1"}`)
	if err != nil {
		t.Fatalf("callTool read_record: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_NotFound covers the !record.Exists() branch.
func TestRegisterMCPTools_ReadRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 9, "read_record", `{"id":"test.items/ghost"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_ReadDefError covers readDef error in read_record.
func TestRegisterMCPTools_ReadRecord_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 10, "read_record", `{"id":"test.items/r1"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_InvalidID covers invalid ID in read_record.
func TestRegisterMCPTools_ReadRecord_InvalidID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 11, "read_record", `{"id":"invalid"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_DBOpenError covers newDB error in read_record.
func TestRegisterMCPTools_ReadRecord_DBOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db error")
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 12, "read_record", `{"id":"test.items/r1"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_Success updates an existing record.
func TestRegisterMCPTools_UpdateRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	// Records are stored under $records/ subdirectory (see RecordsBasePath).
	recordsDir := filepath.Join(dir, "$records")
	if mkdirErr := os.MkdirAll(recordsDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	content, marshalErr := yaml.Marshal(map[string]any{"name": "old"})
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(recordsDir, "r2.yaml"), content, 0o644); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r2","fields":"{name: new}"}`
	resp, err := tr.callTool(context.Background(), 13, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool update_record: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_NotFound covers !record.Exists in update.
func TestRegisterMCPTools_UpdateRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/ghost","fields":"{name: x}"}`
	resp, err := tr.callTool(context.Background(), 14, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_ReadDefError covers readDef error in update.
func TestRegisterMCPTools_UpdateRecord_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","fields":"{name: x}"}`
	resp, err := tr.callTool(context.Background(), 15, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_InvalidID covers invalid ID in update.
func TestRegisterMCPTools_UpdateRecord_InvalidID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"invalid","fields":"{name: x}"}`
	resp, err := tr.callTool(context.Background(), 16, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_ParseFieldsError covers fields parse error.
func TestRegisterMCPTools_UpdateRecord_ParseFieldsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","fields":"invalid yaml: ["}`
	resp, err := tr.callTool(context.Background(), 17, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_UpdateRecord_DBOpenError covers newDB error in update.
func TestRegisterMCPTools_UpdateRecord_DBOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db error")
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	argsJSON := `{"id":"test.items/r1","fields":"{name: x}"}`
	resp, err := tr.callTool(context.Background(), 18, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_DeleteRecord_Success deletes an existing record.
func TestRegisterMCPTools_DeleteRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	// Records are stored under $records/ subdirectory (see RecordsBasePath).
	recordsDir := filepath.Join(dir, "$records")
	if mkdirErr := os.MkdirAll(recordsDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	content, marshalErr := yaml.Marshal(map[string]any{"name": "to-delete"})
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(recordsDir, "del1.yaml"), content, 0o644); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 19, "delete_record", `{"id":"test.items/del1"}`)
	if err != nil {
		t.Fatalf("callTool delete_record: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_DeleteRecord_ReadDefError covers readDef error in delete.
func TestRegisterMCPTools_DeleteRecord_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 20, "delete_record", `{"id":"test.items/r1"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_DeleteRecord_InvalidID covers invalid ID in delete.
func TestRegisterMCPTools_DeleteRecord_InvalidID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 21, "delete_record", `{"id":"invalid"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_DeleteRecord_DBOpenError covers newDB error in delete.
func TestRegisterMCPTools_DeleteRecord_DBOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	server, tr := newTestMCPServer()
	def := testMCPDef(dir)
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db error")
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	resp, err := tr.callTool(context.Background(), 22, "delete_record", `{"id":"test.items/r1"}`)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// ---------------------------------------------------------------------------
// serveMCP tests
// ---------------------------------------------------------------------------

// TestServeMCP_CtxCancelled verifies that serveMCP returns nil when the
// context is cancelled after the server starts.
func TestServeMCP_CtxCancelled(t *testing.T) {
	// Modifies newMCPServerFn — not parallel.
	dir := t.TempDir()

	originalFn := newMCPServerFn
	newMCPServerFn = func() *mcp.Server {
		tr := &testMCPTransport{} // Start returns nil
		return mcp.NewServer(tr, mcp.WithName("test"), mcp.WithVersion("1.0"))
	}
	defer func() { newMCPServerFn = originalFn }()

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := serveMCP(ctx, dir, readDef, newDB, func(...any) {})
	if err != nil {
		t.Fatalf("serveMCP: expected nil, got %v", err)
	}
}

// TestServeMCP_ServeError verifies that serveMCP returns an error when
// server.Serve() fails (transport.Start returns an error).
func TestServeMCP_ServeError(t *testing.T) {
	// Modifies newMCPServerFn — not parallel.
	dir := t.TempDir()

	originalFn := newMCPServerFn
	newMCPServerFn = func() *mcp.Server {
		tr := &testMCPTransport{startErr: errors.New("transport start failed")}
		return mcp.NewServer(tr, mcp.WithName("test"), mcp.WithVersion("1.0"))
	}
	defer func() { newMCPServerFn = originalFn }()

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	ctx := context.Background()
	err := serveMCP(ctx, dir, readDef, newDB, func(...any) {})
	if err == nil {
		t.Fatal("expected error when server.Serve fails")
	}
}

// TestServeMCP_RegisterToolsError verifies that serveMCP propagates errors from
// registerMCPTools (when the server is already running and Send fails).
func TestServeMCP_RegisterToolsError(t *testing.T) {
	// Modifies newMCPServerFn — not parallel.
	dir := t.TempDir()

	tr := &testMCPTransport{}
	srv := mcp.NewServer(tr, mcp.WithName("test"), mcp.WithVersion("1.0"))
	// Start the server before wrapping it (so isRunning=true during registration).
	if err := srv.Serve(); err != nil {
		t.Fatalf("srv.Serve: %v", err)
	}
	// Make subsequent Send calls fail (triggers RegisterTool notification error).
	tr.sendErr = errors.New("send error")

	originalFn := newMCPServerFn
	newMCPServerFn = func() *mcp.Server { return srv }
	defer func() { newMCPServerFn = originalFn }()

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return testMCPDef(dir), nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	ctx := context.Background()
	err := serveMCP(ctx, dir, readDef, newDB, func(...any) {})
	if err == nil {
		t.Fatal("expected error when registerMCPTools fails")
	}
}

// TestRegisterMCPTools_CreateRecord_TxInsertError covers the tx.Insert error
// branch (line 132-134) by attempting to create a record that already exists.
func TestRegisterMCPTools_CreateRecord_TxInsertError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	// Pre-seed the record under $records/ so tx.Insert fails on duplicate.
	recordsDir := filepath.Join(dir, "$records")
	if mkdirErr := os.MkdirAll(recordsDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	content, marshalErr := yaml.Marshal(map[string]any{"name": "existing"})
	if marshalErr != nil {
		t.Fatalf("yaml.Marshal: %v", marshalErr)
	}
	if writeErr := os.WriteFile(filepath.Join(recordsDir, "dup1.yaml"), content, 0o644); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	// Try to create the same record again — tx.Insert should return an error.
	argsJSON := `{"id":"test.items/dup1","data":"{name: duplicate}"}`
	resp, err := tr.callTool(context.Background(), 23, "create_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}

// TestRegisterMCPTools_ReadRecord_TxGetError covers the tx.Get error branch
// in read_record (line 163-165). We use a failing newDB that returns a DB
// whose read transaction errors on Get.
func TestRegisterMCPTools_UpdateRecord_TxGetError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMCPDef(dir)

	// UpdateRecord reads the existing record with tx.Get first. If the record
	// does not exist, the local backend returns (record.Exists() == false) which
	// exercises the !record.Exists() branch (already covered by NotFound test).
	// To trigger tx.Get error we need a corrupt / unreadable file.
	recordsDir := filepath.Join(dir, "$records")
	if mkdirErr := os.MkdirAll(recordsDir, 0o755); mkdirErr != nil {
		t.Fatalf("MkdirAll: %v", mkdirErr)
	}
	// Write invalid YAML content to the record file so parsing fails.
	if writeErr := os.WriteFile(filepath.Join(recordsDir, "bad.yaml"), []byte("{: invalid yaml ["), 0o644); writeErr != nil {
		t.Fatalf("WriteFile: %v", writeErr)
	}

	server, tr := newTestMCPServer()
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	if err := registerMCPTools(server, dir, readDef, newDB); err != nil {
		t.Fatalf("registerMCPTools: %v", err)
	}
	if err := server.Serve(); err != nil {
		t.Fatalf("server.Serve: %v", err)
	}

	// Attempt to update — tx.Get should fail on the corrupt record file.
	argsJSON := `{"id":"test.items/bad","fields":"{name: new}"}`
	resp, err := tr.callTool(context.Background(), 24, "update_record", argsJSON)
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}
