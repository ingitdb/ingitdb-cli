package commands

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/docsbuilder"
)

func docsUpdate(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Update documentation files based on metadata",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "path",
				Usage: "path to the database directory",
			},
			&cli.StringFlag{
				Name:  "collection",
				Usage: "collection path or glob pattern (e.g. 'teams', 'agile.teams/*', 'agile.teams/**')",
			},
			&cli.StringFlag{
				Name:  "view",
				Usage: "Planned: view path to update. Do not use for now.",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			collectionGlob := cmd.String("collection")
			viewGlob := cmd.String("view")

			if collectionGlob == "" && viewGlob == "" {
				return cli.Exit("either --collection or --view flag must be provided", 1)
			}
			if viewGlob != "" {
				return cli.Exit("--view is not implemented yet", 1)
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
			dirPath = expanded

			validateOpt := ingitdb.Validate()
			def, err := readDefinition(dirPath, validateOpt)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			result, err := docsbuilder.UpdateDocs(ctx, def, collectionGlob)
			if err != nil {
				return fmt.Errorf("failed to update docs: %w", err)
			}

			logf(fmt.Sprintf("docs update completed: %d written, %d unchanged", result.FilesWritten, result.FilesUnchanged))
			if len(result.Errors) > 0 {
				for _, err := range result.Errors {
					logf(fmt.Sprintf("error: %v", err))
				}
				return cli.Exit("finished with errors", 1)
			}

			return nil
		},
	}
}
