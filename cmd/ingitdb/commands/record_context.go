package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
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

// resolveRecordContext resolves the DB and collection context for a record operation.
// It handles both GitHub and local-path sources transparently.
func resolveRecordContext(
	ctx context.Context,
	cmd *cli.Command,
	id string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
	githubValue := cmd.String("github")
	if githubValue != "" {
		return resolveGitHubRecordContext(ctx, cmd, id, githubValue)
	}
	return resolveLocalRecordContext(cmd, id, homeDir, getWd, readDefinition, newDB)
}

func resolveGitHubRecordContext(ctx context.Context, cmd *cli.Command, id, githubValue string) (recordContext, error) {
	spec, parseErr := parseGitHubRepoSpec(githubValue)
	if parseErr != nil {
		return recordContext{}, parseErr
	}
	def, collectionID, key, readErr := readRemoteDefinitionForID(ctx, spec, id)
	if readErr != nil {
		return recordContext{}, fmt.Errorf("failed to resolve remote definition: %w", readErr)
	}
	cfg := newGitHubConfig(spec, githubToken(cmd))
	db, err := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
	if err != nil {
		return recordContext{}, fmt.Errorf("failed to open github database: %w", err)
	}
	colDef := def.Collections[collectionID]
	if colDef == nil {
		return recordContext{}, fmt.Errorf("collection not found: %s", collectionID)
	}
	return recordContext{db: db, colDef: colDef, recordKey: key, def: def}, nil
}

func resolveLocalRecordContext(
	cmd *cli.Command,
	id string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
	dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
	if resolveErr != nil {
		return recordContext{}, resolveErr
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return recordContext{}, fmt.Errorf("failed to read database definition: %w", readErr)
	}
	colDef, recordKey, parseErr := dalgo2ingitdb.CollectionForKey(def, id)
	if parseErr != nil {
		return recordContext{}, fmt.Errorf("invalid --id: %w", parseErr)
	}
	db, err := newDB(dirPath, def)
	if err != nil {
		return recordContext{}, fmt.Errorf("failed to open database: %w", err)
	}
	return recordContext{db: db, colDef: colDef, recordKey: recordKey, dirPath: dirPath, def: def}, nil
}

// buildLocalViews materializes views for the collection. It is a no-op when
// the record context refers to a GitHub source (dirPath is empty).
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
