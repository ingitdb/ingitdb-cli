package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitrepo"
)

// recordContext holds the resolved state needed to operate on a single record.
type recordContext struct {
	db        dal.DB
	colDef    *ingitdb.CollectionDef
	recordKey string
	dirPath   string // empty when source is GitHub
	def       *ingitdb.Definition
}

func resolveRemoteRecordContext(ctx context.Context, cmd *cobra.Command, id, remoteValue string) (recordContext, error) {
	spec, err := resolveRemoteFromFlags(cmd, remoteValue)
	if err != nil {
		return recordContext{}, err
	}
	def, collectionID, key, readErr := readRemoteDefinitionForID(ctx, spec, id)
	if readErr != nil {
		return recordContext{}, fmt.Errorf("failed to resolve remote definition: %w", readErr)
	}
	cfg := newGitHubConfig(spec, remoteToken(cmd, spec.Host))
	db, err := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
	if err != nil {
		return recordContext{}, fmt.Errorf("failed to open remote database: %w", err)
	}
	colDef := def.Collections[collectionID]
	if colDef == nil {
		return recordContext{}, fmt.Errorf("collection not found: %s", collectionID)
	}
	return recordContext{db: db, colDef: colDef, recordKey: key, def: def}, nil
}

// buildLocalViews materializes views for the collection. It is a no-op when
// the record context refers to a remote source (dirPath is empty).
func buildLocalViews(ctx context.Context, rctx recordContext) error {
	if rctx.dirPath == "" {
		return nil
	}
	builder, err := viewBuilderFactory.ViewBuilderForCollection(rctx.colDef)
	if err != nil {
		return fmt.Errorf("failed to init view builder for collection %s: %w", rctx.colDef.ID, err)
	}
	if builder == nil {
		return nil
	}
	repoRoot, err := gitrepo.FindRepoRoot(rctx.dirPath)
	if err != nil {
		// Log warning but continue
		repoRoot = ""
	}
	_, buildErr := builder.BuildViews(ctx, rctx.dirPath, repoRoot, rctx.colDef, rctx.def)
	if buildErr != nil {
		return fmt.Errorf("failed to materialize views for collection %s: %w", rctx.colDef.ID, buildErr)
	}
	return nil
}
