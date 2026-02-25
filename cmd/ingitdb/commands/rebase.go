package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/urfave/cli/v3"
)

func Rebase(
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "rebase",
		Usage: "Rebase current branch on top of the base ref",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "base_ref",
				Usage: "Base reference to rebase on top of (defaults to BASE_REF or GITHUB_BASE_REF env var)",
			},
			&cli.StringFlag{
				Name:  "resolve",
				Usage: "comma-separated list of file names to resolve conflicts for (e.g. 'readme,views')",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			baseRef := cmd.String("base_ref")
			if baseRef == "" {
				baseRef = os.Getenv("BASE_REF")
				if baseRef == "" {
					baseRef = os.Getenv("GITHUB_BASE_REF")
				}
			}

			if baseRef == "" {
				return cli.Exit("base ref not provided. Use --base_ref or set BASE_REF / GITHUB_BASE_REF environment variables", 1)
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
				// Rebase failed, probably conflict
				diffCmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
				diffCmd.Dir = wd
				diffOut, diffErr := diffCmd.Output()
				if diffErr != nil {
					return cli.Exit(fmt.Sprintf("rebase failed:\n%s\nfailed to check conflicts: %v", rebaseOut, diffErr), 1)
				}

				resolveStr := cmd.String("resolve")
				resolveItems := make(map[string]bool)
				if resolveStr != "" {
					for _, p := range strings.Split(resolveStr, ",") {
						resolveItems[strings.ToLower(strings.TrimSpace(p))] = true
					}
				}

				conflictedFiles := strings.Split(strings.TrimSpace(string(diffOut)), "\n")
				var hasNonReadmeConflicts bool
				var actualConflictedFiles []string
				for _, f := range conflictedFiles {
					if f == "" {
						continue
					}
					actualConflictedFiles = append(actualConflictedFiles, f)
					if strings.ToLower(filepath.Base(f)) != "readme.md" {
						hasNonReadmeConflicts = true
					}
				}

				if hasNonReadmeConflicts || len(actualConflictedFiles) == 0 {
					return cli.Exit(fmt.Sprintf("rebase failed with unresolved conflicts in files other than README.md:\n%s\nOutput:\n%s", strings.Join(actualConflictedFiles, "\n"), rebaseOut), 1)
				}

				logf("only README.md files are in conflict. resolving via docs update...")

				validateOpt := ingitdb.Validate()
				def, err := readDefinition(wd, validateOpt)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to read database definition: %v", err), 1)
				}

				err = runDocsUpdate(ctx, wd, def, "", resolveStr, logf)
				if err != nil {
					return cli.Exit(fmt.Sprintf("failed to resolve docs:\n%v", err), 1)
				}

				logf("README.md conflicts resolved. checking git status...")

				// Commit if rebase stopped
				msgFile := filepath.Join(wd, ".git", "rebase-merge", "message")
				msgBytes, err := os.ReadFile(msgFile)
				commitMsg := "chore(ingitdb): resolved README.md conflicts"
				if err == nil {
					commitMsg = "chore(ingitdb): " + string(msgBytes)
				} else {
					msgFileApply := filepath.Join(wd, ".git", "rebase-apply", "msg")
					if b, err2 := os.ReadFile(msgFileApply); err2 == nil {
						commitMsg = "chore(ingitdb): " + string(b)
					}
				}

				cCmd := exec.CommandContext(ctx, "git", "commit", "--no-verify", "-m", commitMsg)
				cCmd.Dir = wd
				if out, err := cCmd.CombinedOutput(); err != nil {
					return cli.Exit(fmt.Sprintf("failed to commit resolved files:\n%s\n%v", out, err), 1)
				}

				// Continue rebase
				env := os.Environ()
				env = append(env, "GIT_EDITOR=true")
				contCmd := exec.CommandContext(ctx, "git", "rebase", "--continue")
				contCmd.Dir = wd
				contCmd.Env = env
				contOut, contErr := contCmd.CombinedOutput()
				if contErr != nil {
					return cli.Exit(fmt.Sprintf("failed to continue rebase:\n%s", contOut), 1)
				}
			}

			logf("rebase completed successfully.")
			return nil
		},
	}

}
