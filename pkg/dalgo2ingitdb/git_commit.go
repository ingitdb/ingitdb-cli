package dalgo2ingitdb

import (
	"context"
	"fmt"
	"os/exec"
)

// gitCommitPaths stages exactly the given record-file paths in the git
// repository that contains repoDir and commits them with message. It is used
// by RunReadwriteTransaction for the opt-in commit triggered by a transaction
// message. Paths are deduplicated (a record written several times in one
// transaction yields one staged path). git is invoked with -C repoDir so the
// enclosing repository is discovered even when repoDir is a subdirectory.
func gitCommitPaths(ctx context.Context, repoDir string, paths []string, message string) error {
	staged := dedupeStrings(paths)
	if len(staged) == 0 {
		return nil
	}
	// The transaction message is also a general annotation (e.g. for logging),
	// not necessarily a request to commit. Only commit when repoDir is inside a
	// git work tree; otherwise the message simply has no commit target and the
	// written files are left in place. This keeps dalgo2ingitdb usable on plain
	// directories (and in tests) without requiring a git repository.
	if !isInsideGitWorkTree(ctx, repoDir) {
		return nil
	}
	addArgs := append([]string{"-C", repoDir, "add", "--"}, staged...)
	if out, err := exec.CommandContext(ctx, "git", addArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("dalgo2ingitdb: git add: %w: %s", err, out)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", repoDir, "commit", "-m", message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("dalgo2ingitdb: git commit: %w: %s", err, out)
	}
	return nil
}

// isInsideGitWorkTree reports whether dir is inside a git work tree.
func isInsideGitWorkTree(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// dedupeStrings returns the input with duplicates removed, preserving order.
func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
