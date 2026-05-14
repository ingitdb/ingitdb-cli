package commands

// specscore: feature/cli/materialize

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitrepo"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

func materializeRunE(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if viewBuilder == nil {
			return fmt.Errorf("not yet implemented")
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
		dirPath, err = filepath.Abs(expanded)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		logf("inGitDB db path: ", dirPath)

		ctx := cmd.Context()
		repoRoot, err := gitrepo.FindRepoRoot(dirPath)
		if err != nil {
			logf(fmt.Sprintf("Could not find git repository root for default view export: %v", err))
			repoRoot = ""
		}

		def, err := readDefinition(dirPath)
		if err != nil {
			return fmt.Errorf("failed to read database definition: %w", err)
		}
		var recordsDelimiter *int
		if cmd.Flags().Changed("records-delimiter") {
			v, _ := cmd.Flags().GetInt("records-delimiter")
			recordsDelimiter = &v
		}
		def.RuntimeOverrides.RecordsDelimiter = recordsDelimiter
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
	}
}

// Materialize returns the materialize command.
func Materialize(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "materialize",
		Short: "Materialize views in the database",
		RunE:  materializeRunE(homeDir, getWd, readDefinition, viewBuilder, logf),
	}
	addMaterializeFlags(cmd)
	return cmd
}
