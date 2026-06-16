package commands

// specscore: feature/cli/rebase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func Rebase(
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rebase",
		Short: "Rebase current branch on top of the base ref",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			baseRef, _ := cmd.Flags().GetString("base_ref")
			if baseRef == "" {
				baseRef = os.Getenv("BASE_REF")
				if baseRef == "" {
					baseRef = os.Getenv("GITHUB_BASE_REF")
				}
			}

			if baseRef == "" {
				return fmt.Errorf("base ref not provided. Use --base_ref or set BASE_REF / GITHUB_BASE_REF environment variables")
			}

			wd, err := getWd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			logf(fmt.Sprintf("rebasing on top of %s...", baseRef))

			rebaseCmd := exec.CommandContext(ctx, "git", "rebase", baseRef)
			rebaseCmd.Dir = wd
			rebaseOut, rebaseErr := rebaseCmd.CombinedOutput()

			if rebaseErr != nil {
				// Rebase halted, probably on a conflict. Delegate conflict
				// handling to the shared resolve engine.
				conflictedFiles, diffErr := gitConflictedFiles(ctx, wd)
				if diffErr != nil {
					return fmt.Errorf("rebase failed:\n%s\nfailed to check conflicts: %w", rebaseOut, diffErr)
				}
				if len(conflictedFiles) == 0 {
					return fmt.Errorf("rebase failed:\n%s", rebaseOut)
				}

				resolveStr, _ := cmd.Flags().GetString("resolve")
				resolveItems := parseResolveItems(resolveStr)

				validateOpt := ingitdb.Validate()
				def, readErr := readDefinition(wd, validateOpt)
				if readErr != nil {
					return fmt.Errorf("failed to read database definition: %v", readErr)
				}

				result, _, unresolved, resolveErr := resolveGeneratedConflicts(ctx, wd, def, resolveItems, conflictedFiles)
				if resolveErr != nil {
					return fmt.Errorf("failed to resolve docs:\n%w", resolveErr)
				}
				if len(unresolved) > 0 {
					unresolvedList := strings.Join(unresolved, "\n")
					if abortErr := gitRebaseAbort(ctx, wd); abortErr != nil {
						return fmt.Errorf("rebase has conflicts in files outside the --resolve scope, and 'git rebase --abort' also failed:\n%s\nunresolved:\n%s",
							abortErr, unresolvedList)
					}
					return fmt.Errorf("rebase aborted: conflicts in files outside the --resolve scope must be resolved manually:\n%s",
						unresolvedList)
				}
				if len(result.Errors) > 0 {
					return fmt.Errorf("failed to resolve docs:\n%v", result.Errors[0])
				}

				logf("README.md conflicts resolved. checking git status...")

				commitMsg := readRebaseCommitMessage(wd)
				if commitErr := gitCommitNoVerify(ctx, wd, commitMsg); commitErr != nil {
					return commitErr
				}
				if contErr := gitRebaseContinue(ctx, wd); contErr != nil {
					return contErr
				}
			}

			logf("rebase completed successfully.")
			return nil
		},
	}
	cmd.Flags().String("base_ref", "", "Base reference to rebase on top of (defaults to BASE_REF or GITHUB_BASE_REF env var)")
	cmd.Flags().String("resolve", "", "comma-separated list of file names to resolve conflicts for (e.g. 'readme,views')")
	return cmd
}

// readRebaseCommitMessage reads the pending commit message from git's
// rebase state directory. Checks rebase-merge first (modern default),
// then rebase-apply (patch-mode fallback).
func readRebaseCommitMessage(wd string) string {
	msgFile := filepath.Join(wd, ".git", "rebase-merge", "message")
	if b, err := os.ReadFile(msgFile); err == nil {
		return "chore(ingitdb): " + string(b)
	}
	msgFileApply := filepath.Join(wd, ".git", "rebase-apply", "msg")
	if b, err := os.ReadFile(msgFileApply); err == nil {
		return "chore(ingitdb): " + string(b)
	}
	return "chore(ingitdb): resolved README.md conflicts"
}

// gitCommitNoVerify stages and commits with --no-verify.
func gitCommitNoVerify(ctx context.Context, wd, message string) error {
	cCmd := exec.CommandContext(ctx, "git", "commit", "--no-verify", "-m", message)
	cCmd.Dir = wd
	out, err := cCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to commit resolved files:\n%s\n%v", out, err)
	}
	return nil
}

// gitRebaseAbort runs git rebase --abort to return the working tree to its
// pre-rebase state when conflicts fall outside the --resolve scope.
func gitRebaseAbort(ctx context.Context, wd string) error {
	abortCmd := exec.CommandContext(ctx, "git", "rebase", "--abort")
	abortCmd.Dir = wd
	out, err := abortCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to abort rebase:\n%s", out)
	}
	return nil
}

// gitRebaseContinue runs git rebase --continue with GIT_EDITOR=true.
func gitRebaseContinue(ctx context.Context, wd string) error {
	env := os.Environ()
	env = append(env, "GIT_EDITOR=true")
	contCmd := exec.CommandContext(ctx, "git", "rebase", "--continue")
	contCmd.Dir = wd
	contCmd.Env = env
	contOut, err := contCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to continue rebase:\n%s", contOut)
	}
	return nil
}
