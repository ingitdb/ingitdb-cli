package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// insertContext holds the resolved state needed to insert a record.
// It mirrors recordContext but is built from --into (a collection ID)
// instead of --id (a collection/key pair). recordKey is empty here;
// it is resolved separately in insert.go from --key or $id-in-data.
type insertContext struct {
	db      dal.DB
	colDef  *ingitdb.CollectionDef
	dirPath string // empty when source is GitHub
	def     *ingitdb.Definition
}

// toRecordContext converts an insertContext into a recordContext
// suitable for buildLocalViews. The recordKey field is left empty —
// callers that need it should set it explicitly.
func (ictx insertContext) toRecordContext() recordContext {
	return recordContext{
		db:      ictx.db,
		colDef:  ictx.colDef,
		dirPath: ictx.dirPath,
		def:     ictx.def,
	}
}

// resolveInsertContext loads the database definition (local or
// GitHub), validates that the target collection exists, opens a DB,
// and returns the assembled insertContext.
//
// The caller supplies the collection ID directly (from --into) rather
// than parsing it out of an --id value.
func resolveInsertContext(
	ctx context.Context,
	cmd *cobra.Command,
	collectionID string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (insertContext, error) {
	remoteVal, _ := cmd.Flags().GetString("remote")
	pathVal, _ := cmd.Flags().GetString("path")
	if remoteVal != "" && pathVal != "" {
		return insertContext{}, fmt.Errorf("--path with --remote is not supported")
	}
	if remoteVal != "" {
		return resolveInsertContextRemote(ctx, cmd, collectionID, remoteVal)
	}
	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return insertContext{}, err
	}
	def, err := readDefinition(dirPath)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to read database definition: %w", err)
	}
	colDef, ok := def.Collections[collectionID]
	if !ok {
		return insertContext{}, fmt.Errorf("collection %q not found in definition", collectionID)
	}
	db, err := newDB(dirPath, def)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to open database: %w", err)
	}
	return insertContext{
		db:      db,
		colDef:  colDef,
		dirPath: dirPath,
		def:     def,
	}, nil
}

// resolveInsertContextRemote is the remote-source variant. It uses
// the existing readRemoteDefinitionForCollection helper to load only
// the named collection's definition from the remote repo.
func resolveInsertContextRemote(
	ctx context.Context,
	cmd *cobra.Command,
	collectionID, remoteValue string,
) (insertContext, error) {
	spec, err := resolveRemoteFromFlags(cmd, remoteValue)
	if err != nil {
		return insertContext{}, err
	}
	def, err := readRemoteDefinitionForCollection(ctx, spec, collectionID)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to resolve remote definition: %w", err)
	}
	colDef, ok := def.Collections[collectionID]
	if !ok {
		return insertContext{}, fmt.Errorf("collection %q not found in remote definition", collectionID)
	}
	cfg := newGitHubConfig(spec, remoteToken(cmd, spec.Host))
	db, err := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
	if err != nil {
		return insertContext{}, fmt.Errorf("failed to open remote database: %w", err)
	}
	return insertContext{
		db:      db,
		colDef:  colDef,
		dirPath: "", // empty signals remote source
		def:     def,
	}, nil
}
