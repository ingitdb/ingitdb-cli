package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/docsbuilder"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
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
		// Parse resolve items
		resolveItems := make(map[string]bool)
		for _, p := range strings.Split(resolveStr, ",") {
			resolveItems[strings.ToLower(strings.TrimSpace(p))] = true
		}

		// Get conflicted files
		gitCmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		gitCmd.Dir = dirPath
		out, err := gitCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get conflicted files: %w", err)
		}

		var conflictedFiles []string
		outStr := strings.TrimSpace(string(out))
		if outStr != "" {
			conflictedFiles = strings.Split(outStr, "\n")
		}

		if len(conflictedFiles) == 0 {
			logf("no conflicts found to resolve")
			return nil
		}

		collectionsToUpdate, readmesToUpdate, unresolved := docsbuilder.FindCollectionsForConflictingFiles(def, dirPath, conflictedFiles, resolveItems)

		if len(unresolved) > 0 {
			return fmt.Errorf("unresolved conflicts remain:\n%s", strings.Join(unresolved, "\n"))
		}

		recordsReader := materializer.NewFileRecordsReader()
		result = &ingitdb.MaterializeResult{}
		for _, col := range collectionsToUpdate {
			changed, err := docsbuilder.ProcessCollection(ctx, def, col, dirPath, recordsReader)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("collection %s: %w", col.ID, err))
				continue
			}
			if changed {
				result.FilesUpdated++
			} else {
				result.FilesUnchanged++
			}
		}

		// Stage the resolved items
		if len(collectionsToUpdate) > 0 {
			args := []string{"add"}
			args = append(args, readmesToUpdate...)
			addCmd := exec.Command("git", args...)
			addCmd.Dir = dirPath
			if err := addCmd.Run(); err != nil {
				return fmt.Errorf("failed to stage resolved files: %w", err)
			}
		}

	} else {
		var err error
		recordsReader := materializer.NewFileRecordsReader()
		result, err = docsbuilder.UpdateDocs(ctx, def, collectionGlob, dirPath, recordsReader)
		if err != nil {
			return fmt.Errorf("failed to update docs: %w", err)
		}
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
