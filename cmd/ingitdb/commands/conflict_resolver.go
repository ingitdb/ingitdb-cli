package commands

// specscore: feature/cli/resolve

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/docsbuilder"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// gitConflictedFiles returns the unmerged (conflicted) files in the working
// tree at dirPath, as reported by `git diff --name-only --diff-filter=U`.
// The list is independent of which git operation (rebase, merge, cherry-pick,
// stash pop) produced the conflict.
func gitConflictedFiles(ctx context.Context, dirPath string) ([]string, error) {
	gitCmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
	gitCmd.Dir = dirPath
	out, err := gitCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get conflicted files: %w", err)
	}
	var files []string
	outStr := strings.TrimSpace(string(out))
	if outStr != "" {
		files = strings.Split(outStr, "\n")
	}
	return files, nil
}

// parseResolveItems parses a comma-separated list of generated-file categories
// (e.g. "readme") into a lookup set, lower-cased and trimmed. Empty entries are
// ignored.
func parseResolveItems(resolveStr string) map[string]bool {
	items := make(map[string]bool)
	for p := range strings.SplitSeq(resolveStr, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			items[p] = true
		}
	}
	return items
}

// resolveGeneratedConflicts auto-resolves merge conflicts in generated files
// among conflictedFiles by regenerating each from its source records and
// staging it with `git add`. resolveItems selects which categories are eligible
// (currently "readme" -> collection README.md).
//
// readmes lists the files that were regenerated and staged. unresolved lists
// conflicted files that fall outside the eligible categories; when it is
// non-empty nothing is regenerated or staged so the caller can abort cleanly.
// result carries the per-collection regeneration outcome (including any
// ProcessCollection errors in result.Errors). A non-nil error indicates an
// infrastructure failure (e.g. `git add` failed).
func resolveGeneratedConflicts(
	ctx context.Context,
	dirPath string,
	def *ingitdb.Definition,
	resolveItems map[string]bool,
	conflictedFiles []string,
) (result *ingitdb.MaterializeResult, readmes []string, unresolved []string, err error) {
	collectionsToUpdate, readmesToUpdate, unresolved := docsbuilder.FindCollectionsForConflictingFiles(def, dirPath, conflictedFiles, resolveItems)
	if len(unresolved) > 0 {
		return nil, nil, unresolved, nil
	}

	result = &ingitdb.MaterializeResult{}
	recordsReader := materializer.NewFileRecordsReader()
	for _, col := range collectionsToUpdate {
		changed, procErr := docsbuilder.ProcessCollection(ctx, def, col, dirPath, recordsReader)
		if procErr != nil {
			result.Errors = append(result.Errors, fmt.Errorf("collection %s: %w", col.ID, procErr))
			continue
		}
		if changed {
			result.FilesUpdated++
		} else {
			result.FilesUnchanged++
		}
	}

	if len(collectionsToUpdate) > 0 {
		args := append([]string{"add"}, readmesToUpdate...)
		addCmd := exec.CommandContext(ctx, "git", args...)
		addCmd.Dir = dirPath
		if addErr := addCmd.Run(); addErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to stage resolved files: %w", addErr)
		}
	}

	return result, readmesToUpdate, nil, nil
}
