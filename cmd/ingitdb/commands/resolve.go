package commands

// specscore: feature/cli/resolve

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Resolve returns the resolve command. It is the working-tree conflict engine:
// it auto-resolves merge conflicts in generated files (collection README.md) by
// regenerating them from source records and staging them, independent of which
// git operation produced the conflict.
func Resolve(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve merge conflicts in database files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			conflictedFiles, err := gitConflictedFiles(ctx, dirPath)
			if err != nil {
				return err
			}

			onlyFile, _ := cmd.Flags().GetString("file")
			if onlyFile != "" {
				conflictedFiles = filterConflictedByFile(conflictedFiles, onlyFile)
			}
			if len(conflictedFiles) == 0 {
				logf("no conflicts to resolve")
				return nil
			}

			def, err := readDefinition(dirPath, ingitdb.Validate())
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			resolveItems := map[string]bool{"readme": true}
			result, readmes, unresolved, err := resolveGeneratedConflicts(ctx, dirPath, def, resolveItems, conflictedFiles)
			if err != nil {
				return err
			}
			if len(unresolved) > 0 {
				return fmt.Errorf("could not auto-resolve %d conflict(s); resolve manually:\n%s",
					len(unresolved), strings.Join(unresolved, "\n"))
			}
			if len(result.Errors) > 0 {
				for _, e := range result.Errors {
					logf(fmt.Sprintf("error: %v", e))
				}
				return fmt.Errorf("finished with errors")
			}

			logf(fmt.Sprintf("auto-resolved %d generated file(s)", len(readmes)))
			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("file", "", "specific file to resolve")
	return cmd
}

// filterConflictedByFile keeps only the conflicted files matching onlyFile,
// comparing on cleaned path or base name so the caller can pass either a
// repo-relative path or a bare file name.
func filterConflictedByFile(conflictedFiles []string, onlyFile string) []string {
	want := filepath.Clean(onlyFile)
	wantBase := filepath.Base(onlyFile)
	var kept []string
	for _, f := range conflictedFiles {
		if filepath.Clean(f) == want || filepath.Base(f) == wantBase {
			kept = append(kept, f)
		}
	}
	return kept
}
