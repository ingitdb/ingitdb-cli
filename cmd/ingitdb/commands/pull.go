package commands

// specscore: feature/cli/pull

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go"
	"github.com/ingitdb/ingitdb-go/datavalidator"
	"github.com/ingitdb/ingitdb-go/gitdiff"
	"github.com/ingitdb/ingitdb-go/gitrepo"
	"github.com/ingitdb/ingitdb-go/materializer"
)

// Pull returns the pull command. It runs `git pull` with the chosen strategy,
// delegates conflict resolution to the shared working-tree engine, rebuilds
// materialized views, and prints a summary of the record changes the pull
// brought in.
func Pull(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
	isTerminal func() bool,
	runConflictsTUI func(context.Context, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull latest changes, resolve conflicts, and rebuild views",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			strategy, _ := cmd.Flags().GetString("strategy")
			if strategy != "" && strategy != "rebase" && strategy != "merge" {
				return fmt.Errorf("invalid --strategy=%q (must be \"rebase\" or \"merge\")", strategy)
			}
			remote, _ := cmd.Flags().GetString("remote")
			branch, _ := cmd.Flags().GetString("branch")

			// Record the pre-pull HEAD so we can summarize what the pull brought in.
			beforeRef, err := gitHeadRef(ctx, dirPath)
			if err != nil {
				return fmt.Errorf("failed to read current HEAD: %w", err)
			}

			// (1) git pull. A non-zero exit may simply mean conflicts to resolve,
			// so don't fail yet — inspect the working tree first.
			pullErr := runGitPull(ctx, dirPath, strategy, remote, branch, logf)

			// (2)+(3) resolve any conflicts via the shared engine.
			conflicted, diffErr := gitConflictedFiles(ctx, dirPath)
			if diffErr != nil {
				return fmt.Errorf("failed to check conflicts: %w", diffErr)
			}
			if len(conflicted) > 0 {
				def, defErr := readDefinition(dirPath, ingitdb.Validate())
				if defErr != nil {
					return fmt.Errorf("failed to read database definition: %w", defErr)
				}
				if rErr := resolveWorkingTreeConflicts(ctx, dirPath, def, conflicted, isTerminal, runConflictsTUI, logf); rErr != nil {
					return rErr
				}
			} else if pullErr != nil {
				// Failed for a reason other than conflicts (network, bad ref, …).
				return fmt.Errorf("git pull failed: %w", pullErr)
			}

			// (4) rebuild materialized views + READMEs.
			def, defErr := readDefinition(dirPath)
			if defErr != nil {
				return fmt.Errorf("failed to read database definition: %w", defErr)
			}
			if err := rebuildViews(ctx, dirPath, def, viewBuilder, logf); err != nil {
				return err
			}

			// (5) summarize record changes brought in by the pull.
			summary, sErr := summarizeRecordChanges(ctx, dirPath, def, beforeRef)
			if sErr != nil {
				logf(fmt.Sprintf("pulled successfully; change summary unavailable: %v", sErr))
				return nil
			}
			logf(summary)
			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("strategy", "", "git pull strategy: rebase (default) or merge")
	cmd.Flags().String("remote", "", "remote name (default: origin)")
	cmd.Flags().String("branch", "", "branch to pull (default: tracking branch)")
	return cmd
}

// pullArgs builds the `git pull` argument list from the strategy/remote/branch
// flags. Empty strategy defaults to rebase; empty remote defaults to origin;
// an empty branch is omitted so git uses the configured tracking branch.
func pullArgs(strategy, remote, branch string) []string {
	args := []string{"pull"}
	if strategy == "merge" {
		args = append(args, "--no-rebase")
	} else {
		args = append(args, "--rebase")
	}
	if remote == "" {
		remote = "origin"
	}
	args = append(args, remote)
	if branch != "" {
		args = append(args, branch)
	}
	return args
}

// runGitPull runs `git pull` in dirPath. It returns the command error (if any);
// the caller decides whether that error is a conflict (recoverable) or fatal.
func runGitPull(ctx context.Context, dirPath, strategy, remote, branch string, logf func(...any)) error {
	args := pullArgs(strategy, remote, branch)
	logf(fmt.Sprintf("git %s", strings.Join(args, " ")))
	c := exec.CommandContext(ctx, "git", args...)
	c.Dir = dirPath
	out, err := c.CombinedOutput()
	if err != nil {
		logf(strings.TrimSpace(string(out)))
	}
	return err
}

// gitHeadRef returns the current HEAD commit SHA in dirPath.
func gitHeadRef(ctx context.Context, dirPath string) (string, error) {
	c := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	c.Dir = dirPath
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// rebuildViews materializes views and READMEs for every collection, mirroring
// the `materialize` command.
func rebuildViews(ctx context.Context, dirPath string, def *ingitdb.Definition, viewBuilder materializer.ViewBuilder, logf func(...any)) error {
	if viewBuilder == nil {
		return nil
	}
	repoRoot, err := gitrepo.FindRepoRoot(dirPath)
	if err != nil {
		repoRoot = ""
	}
	var total ingitdb.MaterializeResult
	for _, col := range def.Collections {
		result, buildErr := viewBuilder.BuildViews(ctx, dirPath, repoRoot, col, def)
		if buildErr != nil {
			return fmt.Errorf("failed to rebuild views for collection %s: %w", col.ID, buildErr)
		}
		total.FilesCreated += result.FilesCreated
		total.FilesUpdated += result.FilesUpdated
		total.FilesDeleted += result.FilesDeleted
		total.FilesUnchanged += result.FilesUnchanged
	}
	logf(fmt.Sprintf("rebuilt views: %d created, %d updated, %d deleted, %d unchanged",
		total.FilesCreated, total.FilesUpdated, total.FilesDeleted, total.FilesUnchanged))
	return nil
}

// summarizeRecordChanges diffs beforeRef..HEAD and reports how many record
// files were added, updated, and deleted by the pull. Counting is at
// record-file granularity (within-file row changes for map/list layouts are
// reported as a single updated file).
func summarizeRecordChanges(ctx context.Context, dirPath string, def *ingitdb.Definition, beforeRef string) (string, error) {
	changed, err := gitdiff.NewGitDiffer().DiffFiles(ctx, dirPath, beforeRef, "HEAD")
	if err != nil {
		return "", err
	}
	affected, err := datavalidator.NewChangeSetResolver().Resolve(dirPath, def, changed)
	if err != nil {
		return "", err
	}
	added, updated, deleted := 0, 0, 0
	for _, ar := range affected {
		switch ar.ChangeKind {
		case ingitdb.ChangeKindAdded:
			added++
		case ingitdb.ChangeKindDeleted:
			deleted++
		default:
			updated++
		}
	}
	return fmt.Sprintf("pulled: %d record file(s) added, %d updated, %d deleted", added, updated, deleted), nil
}
