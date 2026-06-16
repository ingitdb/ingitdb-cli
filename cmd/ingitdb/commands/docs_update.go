package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/docsbuilder"
	"github.com/ingitdb/ingitdb-go/ingitdb/materializer"
)

func docsUpdate(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update documentation files based on metadata",
		// Note the plural --collections on the replacement command, versus the
		// singular --collection flag here.
		Deprecated: "use `ingitdb materialize --collections` instead (note the plural flag name).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			collectionGlob, _ := cmd.Flags().GetString("collection")
			viewGlob, _ := cmd.Flags().GetString("view")

			if collectionGlob == "" && viewGlob == "" {
				return fmt.Errorf("either --collection or --view flag must be provided")
			}
			if viewGlob != "" {
				return fmt.Errorf("--view is not implemented yet")
			}

			dirPath, _ := cmd.Flags().GetString("path")
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

			ctx := cmd.Context()
			validateOpt := ingitdb.Validate()
			def, err := readDefinition(dirPath, validateOpt)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			err = runDocsUpdate(ctx, dirPath, def, collectionGlob, "", logf)
			if err != nil {
				return err
			}

			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("collection", "", "collection path or glob pattern (e.g. 'teams', 'agile.teams/*', 'agile.teams/**')")
	cmd.Flags().String("view", "", "Planned: view path to update. Do not use for now.")
	return cmd
}

func runDocsUpdate(ctx context.Context, dirPath string, def *ingitdb.Definition, collectionGlob string, resolveStr string, logf func(...any)) error {
	var result *ingitdb.MaterializeResult
	if resolveStr != "" {
		resolveItems := parseResolveItems(resolveStr)

		conflictedFiles, err := gitConflictedFiles(ctx, dirPath)
		if err != nil {
			return err
		}
		if len(conflictedFiles) == 0 {
			logf("no conflicts found to resolve")
			return nil
		}

		res, _, unresolved, resolveErr := resolveGeneratedConflicts(ctx, dirPath, def, resolveItems, conflictedFiles)
		if resolveErr != nil {
			return resolveErr
		}
		if len(unresolved) > 0 {
			return fmt.Errorf("unresolved conflicts remain:\n%s", strings.Join(unresolved, "\n"))
		}
		result = res
	} else {
		recordsReader := materializer.NewFileRecordsReader()
		result, _ = docsbuilder.UpdateDocs(ctx, def, collectionGlob, dirPath, recordsReader)
	}

	logf(fmt.Sprintf("docs update completed: %d updated, %d unchanged", result.FilesUpdated, result.FilesUnchanged))
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			logf(fmt.Sprintf("error: %v", err))
		}
		return fmt.Errorf("finished with errors")
	}

	return nil
}
