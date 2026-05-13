package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// runBatchInsert implements --format batch mode. It reads stdin per the
// selected stream format, inserts all records inside a single
// transaction, and materializes local views once after commit.
//
// On any pre-commit failure (parse, missing key, schema violation,
// collision, write error, commit error), the batch is rolled back and
// the error is returned. On post-commit view-materialization failure,
// the inserted records remain on disk and a distinct error is returned.
//
// The local-fs dalgo backend's RunReadwriteTransaction is currently a
// stub with no rollback semantics — each tx.Insert writes directly to
// disk. To honor req:batch-atomic without modifying the backend, this
// orchestrator tracks every file it writes and, on any mid-batch
// failure, rolls each back via git (for tracked files) or os.Remove
// (for untracked files).
func runBatchInsert(
	ctx context.Context,
	format string,
	keyColumn string,
	fields []string,
	stdin io.Reader,
	ictx insertContext,
	stderr io.Writer,
) error {
	records, err := parseBatchStream(format, keyColumn, fields, stdin)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		_, _ = fmt.Fprintln(stderr, "0 records inserted")
		return nil
	}
	// Pre-commit intra-batch duplicate check.
	err = rejectIntraBatchDuplicates(records)
	if err != nil {
		return err
	}
	// Atomic insert. Any individual failure aborts the whole batch.
	var writtenPaths []string
	commitErr := ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, rec := range records {
			key := dal.NewKeyWithID(ictx.colDef.ID, rec.Key)
			r := dal.NewRecordWithData(key, rec.Data)
			path := resolveBatchRecordPath(ictx.colDef, rec.Key)
			insertErr := tx.Insert(ctx, r)
			if insertErr != nil {
				return fmt.Errorf("record at position %d (key=%q): %w", rec.Position, rec.Key, insertErr)
			}
			writtenPaths = append(writtenPaths, path)
		}
		return nil
	})
	if commitErr != nil {
		rbErr := rollbackBatchWrites(ctx, ictx.dirPath, writtenPaths)
		if rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", commitErr, rbErr)
		}
		return commitErr
	}
	// Post-commit: materialize local views EXACTLY ONCE (req:batch-view-
	// materialization). Failures here cannot be rolled back — records
	// are on disk — and MUST be reported with a diagnostic distinct from
	// a pre-commit rollback (req:batch-post-commit-failure). The error
	// wrap on line below ("records inserted but view materialization
	// failed") is the contract.
	//
	// AC: batch-view-materialization-once and AC: batch-post-commit-
	// view-failure are verified by code inspection of THIS call site —
	// runtime tests would require a view-fault-injection test fixture
	// (ViewDef pointing at a column the records don't supply, or a
	// write-target that is read-only). That fixture infrastructure is
	// out of MVP scope; revisit when batch update/delete arrive and the
	// view path needs end-to-end coverage anyway.
	rctx := recordContext{
		db:      ictx.db,
		colDef:  ictx.colDef,
		dirPath: ictx.dirPath,
		def:     ictx.def,
	}
	viewErr := buildLocalViews(ctx, rctx)
	if viewErr != nil {
		return fmt.Errorf("records inserted but view materialization failed: %w", viewErr)
	}
	_, _ = fmt.Fprintf(stderr, "%d records inserted\n", len(records))
	return nil
}

// parseBatchStream routes to the format-specific parser.
func parseBatchStream(format, keyColumn string, fields []string, r io.Reader) ([]dalgo2ingitdb.ParsedRecord, error) {
	_, _ = keyColumn, fields
	switch format {
	case "jsonl":
		return dalgo2ingitdb.ParseBatchJSONL(r)
	case "yaml":
		return dalgo2ingitdb.ParseBatchYAMLStream(r)
	case "ingr":
		return dalgo2ingitdb.ParseBatchINGR(r)
	case "csv":
		return nil, fmt.Errorf("batch format %q not yet implemented", format)
	default:
		return nil, fmt.Errorf("unsupported batch format %q", format)
	}
}

// rejectIntraBatchDuplicates returns an error if two records in the
// batch share a resolved key.
func rejectIntraBatchDuplicates(records []dalgo2ingitdb.ParsedRecord) error {
	seen := make(map[string]int, len(records))
	for _, rec := range records {
		if prev, dup := seen[rec.Key]; dup {
			return fmt.Errorf("duplicate key %q in batch: positions %d and %d", rec.Key, prev, rec.Position)
		}
		seen[rec.Key] = rec.Position
	}
	return nil
}

// resolveBatchRecordPath mirrors the path-derivation logic that the
// dalgo2fsingitdb backend uses for SingleRecord writes:
// <colDef.DirPath>/<RecordsBasePath()>/<record_file.name with {key} substituted>.
// This is intentionally a local helper — the backend's resolveRecordPath
// is unexported, and duplicating ~3 lines of join logic is cheaper than
// changing the public surface of the backend package.
func resolveBatchRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	name := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	base := colDef.RecordFile.RecordsBasePath()
	return filepath.Join(colDef.DirPath, base, name)
}

// rollbackBatchWrites restores each path to its committed state. For
// paths that were tracked by git in the working tree, runs
// `git checkout HEAD -- <path>`. For untracked paths (new files this
// batch created), removes them via os.Remove. When the directory is not
// a git working tree, falls back to os.Remove for all paths — correct
// for INSERT, which only creates new files. Future Update/Delete
// batches in non-git directories would need a different approach.
//
// Returns the first error encountered (others are best-effort and
// silently skipped, so a partial rollback failure does not prevent
// reverting the remaining paths).
func rollbackBatchWrites(ctx context.Context, repoRoot string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	isGit := isGitWorkingTree(ctx, repoRoot)
	var firstErr error
	for _, path := range paths {
		var err error
		if isGit && isTracked(ctx, repoRoot, path) {
			err = gitCheckoutPath(ctx, repoRoot, path)
		} else {
			removeErr := os.Remove(path)
			if removeErr != nil && !os.IsNotExist(removeErr) {
				err = fmt.Errorf("remove %s: %w", path, removeErr)
			}
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// isGitWorkingTree returns true if dir is inside a git working tree.
func isGitWorkingTree(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// isTracked returns true if path is tracked by git in the working tree
// rooted at repoRoot.
func isTracked(ctx context.Context, repoRoot, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "ls-files", "--error-unmatch", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// gitCheckoutPath restores path to its HEAD-committed state via
// `git checkout HEAD -- <path>`.
func gitCheckoutPath(ctx context.Context, repoRoot, path string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "checkout", "HEAD", "--", path)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil {
		return fmt.Errorf("git checkout %s: %w (%s)", path, runErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}
