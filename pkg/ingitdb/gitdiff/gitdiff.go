package gitdiff

// specscore: feature/cli/validate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// GitDiffer lists files changed between two git refs.
// The default implementation shells out to `git diff --name-status fromRef toRef`.
type GitDiffer interface {
	DiffFiles(ctx context.Context, repoPath, fromRef, toRef string) ([]ingitdb.ChangedFile, error)
}

// NewGitDiffer returns the default GitDiffer, which shells out to git.
func NewGitDiffer() GitDiffer {
	return cmdGitDiffer{}
}

type cmdGitDiffer struct{}

// DiffFiles runs `git diff --name-status <fromRef> [<toRef>]` in repoPath and
// parses the result. An empty toRef diffs fromRef against the working tree.
func (cmdGitDiffer) DiffFiles(ctx context.Context, repoPath, fromRef, toRef string) ([]ingitdb.ChangedFile, error) {
	if fromRef == "" {
		return nil, fmt.Errorf("from ref is required")
	}
	args := []string{"diff", "--name-status", fromRef}
	if toRef != "" {
		args = append(args, toRef)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	return parseNameStatus(string(out)), nil
}

// parseNameStatus parses `git diff --name-status` output into ChangedFile
// entries. Each line is a tab-separated status and path(s); rename/copy lines
// carry the score-suffixed status, an old path, and a new path.
func parseNameStatus(out string) []ingitdb.ChangedFile {
	var changed []ingitdb.ChangedFile
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		status := fields[0]
		switch status[0] {
		case 'A':
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindAdded, Path: fields[1]})
		case 'D':
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: fields[1]})
		case 'R', 'C':
			if len(fields) < 3 {
				continue
			}
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindRenamed, OldPath: fields[1], Path: fields[2]})
		default: // M, T (type change) and anything else → treat as modified
			changed = append(changed, ingitdb.ChangedFile{Kind: ingitdb.ChangeKindModified, Path: fields[1]})
		}
	}
	return changed
}
