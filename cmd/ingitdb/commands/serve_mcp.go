package commands

// specscore: feature/cli/serve

import (
	"context"
	"fmt"
	"maps"
	"sort"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"gopkg.in/yaml.v3"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type listCollectionsArgs struct{}

type createRecordArgs struct {
	ID   string `json:"id"   jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie or todo.tags/abc)"`
	Data string `json:"data" jsonschema:"required,description=Record data as YAML or JSON (e.g. {title: Ireland})"`
}

type readRecordArgs struct {
	ID string `json:"id" jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie or todo.tags/abc)"`
}

type updateRecordArgs struct {
	ID     string `json:"id"     jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie or todo.tags/abc)"`
	Fields string `json:"fields" jsonschema:"required,description=Fields to update as YAML or JSON (e.g. {title: New Name})"`
}

type deleteRecordArgs struct {
	ID string `json:"id" jsonschema:"required,description=Record ID in format collection/path/key (e.g. countries/ie or todo.tags/abc)"`
}

// newMCPServerFn creates a new MCP server backed by the stdio transport.
// It is a package-level variable so tests can replace it with a server
// backed by a fake transport. Tests that replace it MUST NOT run in
// parallel (same rule as the other seam variables in seams.go).
var newMCPServerFn = func() *mcp.Server {
	tr := stdio.NewStdioServerTransport()
	return mcp.NewServer(tr, mcp.WithName("ingitdb"), mcp.WithVersion("1.0"))
}

func serveMCP(
	ctx context.Context,
	dirPath string,
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) error {
	_ = logf
	server := newMCPServerFn()
	if err := registerMCPTools(server, dirPath, readDefinition, newDB); err != nil {
		return err
	}
	if err := server.Serve(); err != nil {
		return fmt.Errorf("MCP server failed to start: %w", err)
	}
	<-ctx.Done()
	return nil
}

// sortedCollectionIDs returns all collection IDs from def in ascending alphabetical order.
// Collection IDs are explicit in .ingitdb/root-collections.yaml (one key per collection path).
func sortedCollectionIDs(def *ingitdb.Definition) []string {
	ids := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// registerMCPTools registers the four CRUD tools on server. Extracted so tests
// can supply a custom transport without going through serveMCP.
func registerMCPTools(
	server *mcp.Server,
	dirPath string,
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	if err := server.RegisterTool(
		"list_collections",
		"List collections in the database",
		func(ctx context.Context, args listCollectionsArgs) (*mcp.ToolResponse, error) {
			_, _ = ctx, args
			def, err := readDefinition(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read database definition: %w", err)
			}
			ids := sortedCollectionIDs(def)
			out, err := yaml.Marshal(ids)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal collections: %w", err)
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(out))), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register list_collections: %w", err)
	}

	if err := server.RegisterTool(
		"create_record",
		"Create a new record in a collection",
		func(ctx context.Context, args createRecordArgs) (*mcp.ToolResponse, error) {
			def, err := readDefinition(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read database definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			var data map[string]any
			if err = yaml.Unmarshal([]byte(args.Data), &data); err != nil {
				return nil, fmt.Errorf("failed to parse data: %w", err)
			}
			db, err := newDB(dirPath, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			}); err != nil {
				return nil, err
			}
			content := mcp.NewTextContent("record created: " + args.ID)
			return mcp.NewToolResponse(content), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register create_record: %w", err)
	}

	if err := server.RegisterTool(
		"read_record",
		"Read a single record by its ID, returns YAML",
		func(ctx context.Context, args readRecordArgs) (*mcp.ToolResponse, error) {
			def, err := readDefinition(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read database definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			db, err := newDB(dirPath, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			data := map[string]any{}
			record := dal.NewRecordWithData(key, data)
			if err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
				return tx.Get(ctx, record)
			}); err != nil {
				return nil, err
			}
			if !record.Exists() {
				return nil, fmt.Errorf("record not found: %s", args.ID)
			}
			out, err := yaml.Marshal(data)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal record: %w", err)
			}
			content := mcp.NewTextContent(string(out))
			return mcp.NewToolResponse(content), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register read_record: %w", err)
	}

	if err := server.RegisterTool(
		"update_record",
		"Update fields of an existing record",
		func(ctx context.Context, args updateRecordArgs) (*mcp.ToolResponse, error) {
			def, err := readDefinition(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read database definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			var patch map[string]any
			if err = yaml.Unmarshal([]byte(args.Fields), &patch); err != nil {
				return nil, fmt.Errorf("failed to parse fields: %w", err)
			}
			db, err := newDB(dirPath, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				data := map[string]any{}
				record := dal.NewRecordWithData(key, data)
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
			content := mcp.NewTextContent("record updated: " + args.ID)
			return mcp.NewToolResponse(content), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register update_record: %w", err)
	}

	if err := server.RegisterTool(
		"delete_record",
		"Delete a record by its ID",
		func(ctx context.Context, args deleteRecordArgs) (*mcp.ToolResponse, error) {
			def, err := readDefinition(dirPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read database definition: %w", err)
			}
			colDef, recordKey, err := dalgo2ingitdb.CollectionForKey(def, args.ID)
			if err != nil {
				return nil, fmt.Errorf("invalid id: %w", err)
			}
			db, err := newDB(dirPath, def)
			if err != nil {
				return nil, fmt.Errorf("failed to open database: %w", err)
			}
			key := dal.NewKeyWithID(colDef.ID, recordKey)
			if err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Delete(ctx, key)
			}); err != nil {
				return nil, err
			}
			content := mcp.NewTextContent("record deleted: " + args.ID)
			return mcp.NewToolResponse(content), nil
		},
	); err != nil {
		return fmt.Errorf("failed to register delete_record: %w", err)
	}

	return nil
}
