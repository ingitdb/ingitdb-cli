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

func materializeCommandRunE(
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

		dirPath, err := resolveMaterializePath(cmd, homeDir, getWd)
		if err != nil {
			return err
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
			mergeMaterializeResult(&totalResult, result)
		}

		logf(materializeSummary(&totalResult))
		return nil
	}
}

// resolveMaterializePath resolves the --path flag (or working directory) into an
// absolute, home-expanded database directory path.
func resolveMaterializePath(
	cmd *cobra.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
) (string, error) {
	dirPath, _ := cmd.Flags().GetString("path")
	if dirPath == "" {
		wd, err := getWd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		dirPath = wd
	}
	expanded, err := expandHome(dirPath, homeDir)
	if err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(expanded)
	return abs, nil
}

// mergeMaterializeResult accumulates src into dst.
func mergeMaterializeResult(dst *ingitdb.MaterializeResult, src *ingitdb.MaterializeResult) {
	if src == nil {
		return
	}
	dst.FilesCreated += src.FilesCreated
	dst.FilesUpdated += src.FilesUpdated
	dst.FilesUnchanged += src.FilesUnchanged
	dst.FilesDeleted += src.FilesDeleted
	dst.Errors = append(dst.Errors, src.Errors...)
}

// materializeSummary renders the created/updated/deleted/unchanged tally line.
func materializeSummary(r *ingitdb.MaterializeResult) string {
	return fmt.Sprintf("materialized: %d created, %d updated, %d deleted, %d unchanged",
		r.FilesCreated, r.FilesUpdated, r.FilesDeleted, r.FilesUnchanged)
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
		Short: "Regenerate derived artifacts: collection READMEs and materialized views",
		RunE:  materializeCommandRunE(homeDir, getWd, readDefinition, viewBuilder, logf),
	}
	addMaterializeCommandFlags(cmd)
	return cmd
}
