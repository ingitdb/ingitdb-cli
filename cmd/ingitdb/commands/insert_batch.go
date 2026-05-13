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
	// MapOfRecords collections store every record in one shared file,
	// which means every batch record resolves to the same path. The
	// rollback path is built around per-record path tracking and would
	// either dedupe the shared path (correct, but only by accident) or,
	// for untracked files, os.Remove the shared file — destroying
	// pre-existing records in the collection. Refuse batch mode for
	// these collections until a proper SetMulti-based path exists.
	if ictx.colDef.RecordFile != nil && ictx.colDef.RecordFile.RecordType == ingitdb.MapOfRecords {
		return fmt.Errorf("batch mode does not yet support collections with record_type=%s; only single-record collections are supported (collection: %s)", ictx.colDef.RecordFile.RecordType, ictx.colDef.ID)
	}
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
			// IMPORTANT: append only AFTER tx.Insert succeeds. The
			// current dalgo2fsingitdb backend performs the collision
			// check BEFORE writing, so a failing Insert means no file
			// was created. Appending the path pre-insert would cause
			// rollback to remove a pre-existing file with the same
			// path (e.g. the colliding record from a prior insert),
			// destroying real data — verified by
			// TestInsertBatch_JSONL_CollisionWithExistingRecord.
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
	rctx := ictx.toRecordContext()
	viewErr := buildLocalViews(ctx, rctx)
	if viewErr != nil {
		return fmt.Errorf("records inserted but view materialization failed: %w", viewErr)
	}
	_, _ = fmt.Fprintf(stderr, "%d records inserted\n", len(records))
	return nil
}

// parseBatchStream routes to the format-specific parser.
func parseBatchStream(format, keyColumn string, fields []string, r io.Reader) ([]dalgo2ingitdb.ParsedRecord, error) {
	switch format {
	case "jsonl":
		return dalgo2ingitdb.ParseBatchJSONL(r)
	case "yaml":
		return dalgo2ingitdb.ParseBatchYAMLStream(r)
	case "ingr":
		return dalgo2ingitdb.ParseBatchINGR(r)
	case "csv":
		return dalgo2ingitdb.ParseBatchCSV(r, dalgo2ingitdb.CSVParseOptions{
			KeyColumn: keyColumn,
			Fields:    fields,
		})
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
	var trackedPaths []string
	var untrackedPaths []string
	if isGit {
		for _, path := range paths {
			if isTracked(ctx, repoRoot, path) {
				trackedPaths = append(trackedPaths, path)
			} else {
				untrackedPaths = append(untrackedPaths, path)
			}
		}
	} else {
		untrackedPaths = paths
	}
	var firstErr error
	if len(trackedPaths) > 0 {
		checkoutErr := gitCheckoutPaths(ctx, repoRoot, trackedPaths)
		if checkoutErr != nil {
			firstErr = checkoutErr
		}
	}
	for _, path := range untrackedPaths {
		removeErr := os.Remove(path)
		if removeErr != nil && !os.IsNotExist(removeErr) && firstErr == nil {
			firstErr = fmt.Errorf("remove %s: %w", path, removeErr)
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

// isTracked reports whether path is tracked by git in the working tree
// rooted at repoRoot.
//
// `git ls-files --error-unmatch` exits 0 when the path is tracked and
// 1 when it is not. Any OTHER exit (transient .git/index.lock, git
// not on PATH, network errors on remote repos, etc.) is treated as
// "unknown — assume tracked": running `git checkout HEAD -- <path>`
// on an untracked file is a benign no-op, but failing to restore a
// tracked file because of a transient git error would destroy real
// data on rollback.
func isTracked(ctx context.Context, repoRoot, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "ls-files", "--error-unmatch", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	runErr := cmd.Run()
	if runErr == nil {
		return true
	}
	exitErr, ok := runErr.(*exec.ExitError)
	if ok && exitErr.ExitCode() == 1 {
		return false
	}
	return true
}

// gitCheckoutPaths restores the supplied paths to their HEAD-committed
// state in a single invocation of `git checkout HEAD -- <path>...`.
// This is meaningful on large batches where the per-path overhead of
// forking git would otherwise dominate the rollback cost.
func gitCheckoutPaths(ctx context.Context, repoRoot string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := make([]string, 0, len(paths)+5)
	args = append(args, "-C", repoRoot, "checkout", "HEAD", "--")
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil {
		return fmt.Errorf("git checkout %d paths: %w (%s)", len(paths), runErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}
