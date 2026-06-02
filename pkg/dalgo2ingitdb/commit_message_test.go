package dalgo2ingitdb_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

// gitInit initialises a git repo at dir with a usable identity so commits work
// deterministically in CI (no global config, no GPG signing).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func franceRecord() dal.Record {
	return dal.NewRecordWithData(
		dal.NewKeyWithID("countries", "france"),
		map[string]any{"name": "France", "population": 67000000},
	)
}

// TestRunReadwriteTransaction_CommitsWithMessage verifies that a read-write
// transaction with a message commits exactly the files it wrote, using the
// message as the commit subject, when the project is a git repository.
func TestRunReadwriteTransaction_CommitsWithMessage(t *testing.T) {
	ctx := context.Background()
	db, root := setupSingleRecordDB(t)
	gitInit(t, root)

	const msg = "add France"
	if err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, franceRecord())
	}, dal.TxWithMessage(msg)); err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}

	if got := git(t, root, "log", "-1", "--pretty=%s"); got != msg {
		t.Errorf("commit subject: got %q, want %q", got, msg)
	}
	if n := git(t, root, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("commit count: got %s, want 1", n)
	}
	// Only the written record file is committed; schema/registration files the
	// harness created remain untracked.
	files := git(t, root, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(files, "france") {
		t.Errorf("committed files should include the france record, got:\n%s", files)
	}
	if strings.Contains(files, ".ingitdb") {
		t.Errorf("schema/registration files must not be committed, got:\n%s", files)
	}
}

// TestRunReadwriteTransaction_NoMessageNoCommit verifies that without a message
// the transaction writes files but creates no commit (behaviour unchanged).
func TestRunReadwriteTransaction_NoMessageNoCommit(t *testing.T) {
	ctx := context.Background()
	db, root := setupSingleRecordDB(t)
	gitInit(t, root)
	git(t, root, "commit", "--allow-empty", "-m", "init")

	if err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, franceRecord())
	}); err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}

	if n := git(t, root, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("no-message tx must not commit: commit count got %s, want 1", n)
	}
	// The record was still written (just left uncommitted in the working tree).
	got := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
	if err := db.Get(ctx, got); err != nil {
		t.Fatalf("record should still be written: %v", err)
	}
}

// TestRunReadwriteTransaction_SetMessageDuringExecution verifies a message set
// at runtime via tx.Options().SetMessage drives the commit.
func TestRunReadwriteTransaction_SetMessageDuringExecution(t *testing.T) {
	ctx := context.Background()
	db, root := setupSingleRecordDB(t)
	gitInit(t, root)

	const msg = "set during execution"
	if err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		if err := tx.Set(ctx, franceRecord()); err != nil {
			return err
		}
		tx.Options().SetMessage(msg)
		return nil
	}); err != nil {
		t.Fatalf("RunReadwriteTransaction: %v", err)
	}

	if got := git(t, root, "log", "-1", "--pretty=%s"); got != msg {
		t.Errorf("commit subject: got %q, want %q", got, msg)
	}
}

// TestRunReadwriteTransaction_NonGitDirNoError verifies that a message in a
// non-git directory is a no-op (file written, no error) rather than failing.
func TestRunReadwriteTransaction_NonGitDirNoError(t *testing.T) {
	ctx := context.Background()
	db, _ := setupSingleRecordDB(t) // no gitInit: plain directory

	if err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, franceRecord())
	}, dal.TxWithMessage("no git here")); err != nil {
		t.Fatalf("RunReadwriteTransaction in non-git dir should not error: %v", err)
	}

	got := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
	if err := db.Get(ctx, got); err != nil {
		t.Fatalf("record should still be written: %v", err)
	}
}
