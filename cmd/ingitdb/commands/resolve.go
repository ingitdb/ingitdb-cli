package commands

// specscore: feature/cli/resolve

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// Resolve returns the resolve command. It is the working-tree conflict engine:
// it auto-resolves merge conflicts in generated files (collection README.md) by
// regenerating them from source records and staging them, independent of which
// git operation produced the conflict. Source-data conflicts that need a human
// decision are handed to runConflictsTUI (interactive) on a terminal, or
// reported as text otherwise.
func Resolve(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
	isTerminal func() bool,
	runConflictsTUI func(ctx context.Context, files []string) error,
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

			return resolveWorkingTreeConflicts(ctx, dirPath, def, conflictedFiles, isTerminal, runConflictsTUI, logf)
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("file", "", "specific file to resolve")
	return cmd
}

// resolveWorkingTreeConflicts is the shared conflict-resolution pipeline used by
// both `resolve` and `pull`. It auto-resolves generated-file conflicts
// (collection README.md) by regenerating them, then attempts a record-aware
// three-way merge of the remaining source-data files, and finally hands any
// still-unresolved source conflicts to the interactive resolver. It returns a
// non-nil error when conflicts remain unresolved or regeneration fails.
func resolveWorkingTreeConflicts(
	ctx context.Context,
	dirPath string,
	def *ingitdb.Definition,
	conflictedFiles []string,
	isTerminal func() bool,
	runConflictsTUI func(context.Context, []string) error,
	logf func(...any),
) error {
	resolveItems := map[string]bool{"readme": true}
	result, readmes, unresolved, err := resolveGeneratedConflicts(ctx, dirPath, def, resolveItems, conflictedFiles)
	if err != nil {
		return err
	}
	if len(unresolved) > 0 {
		mergedFiles, stillUnresolved, mErr := resolveRecordMergeConflicts(ctx, dirPath, def, unresolved)
		if mErr != nil {
			return mErr
		}
		if len(mergedFiles) > 0 {
			logf(fmt.Sprintf("auto-merged %d data file(s)", len(mergedFiles)))
		}
		if len(stillUnresolved) > 0 {
			return reportSourceConflicts(ctx, stillUnresolved, isTerminal, runConflictsTUI, logf)
		}
		return nil
	}
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			logf(fmt.Sprintf("error: %v", e))
		}
		return fmt.Errorf("finished with errors")
	}

	logf(fmt.Sprintf("auto-resolved %d generated file(s)", len(readmes)))
	return nil
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

// reportSourceConflicts hands source-data conflicts to the interactive resolver
// on a terminal, or prints a text placeholder otherwise. Either way it returns a
// non-nil error because manual resolution is not implemented yet, so unresolved
// conflicts always produce a non-zero exit.
func reportSourceConflicts(
	ctx context.Context,
	files []string,
	isTerminal func() bool,
	runConflictsTUI func(context.Context, []string) error,
	logf func(...any),
) error {
	if isTerminal() {
		if err := runConflictsTUI(ctx, files); err != nil {
			return err
		}
	} else {
		logf("Interactive conflict resolution")
		logf("The following files have source-data conflicts that need a human decision:")
		for _, f := range files {
			logf("  - " + f)
		}
		logf("Planned: pick a winner per field instead of editing raw conflict markers.")
		logf("Sorry, not implemented yet.")
	}
	return fmt.Errorf("interactive resolution of source-data conflicts is not implemented yet")
}
