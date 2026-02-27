package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitrepo"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// Materialize returns the materialize command.
func Materialize(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "materialize",
		Usage: "Materialize views in the database",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "views",
				Usage: "comma-separated list of views to materialize",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if viewBuilder == nil {
				return cli.Exit("not yet implemented", 1)
			}
			dirPath := cmd.String("path")
			if dirPath == "" {
				wd, err := getWd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				dirPath = wd
			}
			expanded, err := expandHome(dirPath, homeDir)
			if err != nil {
				return err
			}
			dirPath, err = filepath.Abs(expanded)
			if err != nil {
				return fmt.Errorf("failed to resolve absolute path: %w", err)
			}
			logf("inGitDB db path: ", dirPath)

			repoRoot, err := gitrepo.FindRepoRoot(dirPath)
			if err != nil {
				logf(fmt.Sprintf("Could not find git repository root for default view export: %v", err))
				repoRoot = ""
			}

			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}
			var totalResult ingitdb.MaterializeResult
			for _, col := range def.Collections {
				result, buildErr := viewBuilder.BuildViews(ctx, dirPath, repoRoot, col, def)
				if buildErr != nil {
					return fmt.Errorf("failed to materialize views for collection %s: %w", col.ID, buildErr)
				}
				totalResult.FilesCreated += result.FilesCreated
				totalResult.FilesUpdated += result.FilesUpdated
				totalResult.FilesUnchanged += result.FilesUnchanged
				totalResult.FilesDeleted += result.FilesDeleted
				totalResult.Errors = append(totalResult.Errors, result.Errors...)
			}
			logf(fmt.Sprintf("materialized views: %d created, %d updated, %d deleted, %d unchanged",
				totalResult.FilesCreated, totalResult.FilesUpdated, totalResult.FilesDeleted, totalResult.FilesUnchanged))
			return nil
		},
	}
}
