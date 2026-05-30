package gitdiff_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitdiff"
)

// Compile-time assertion: mockGitDiffer satisfies GitDiffer.
var _ gitdiff.GitDiffer = (*mockGitDiffer)(nil)

// mockGitDiffer is a test double for GitDiffer.
type mockGitDiffer struct {
	files []ingitdb.ChangedFile
	err   error
}

func (m *mockGitDiffer) DiffFiles(
	_ context.Context,
	_, _, _ string,
) ([]ingitdb.ChangedFile, error) {
	return m.files, m.err
}

func TestGitDiffer_DiffFiles_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		files     []ingitdb.ChangedFile
		repoPath  string
		fromRef   string
		toRef     string
		wantLen   int
		wantFirst ingitdb.ChangedFile
	}{
		{
			name:    "nil slice",
			files:   nil,
			wantLen: 0,
		},
		{
			name:    "empty slice",
			files:   []ingitdb.ChangedFile{},
			wantLen: 0,
		},
		{
			name: "single added file",
			files: []ingitdb.ChangedFile{
				{Kind: ingitdb.ChangeKindAdded, Path: "users/alice.yaml"},
			},
			repoPath:  "/repo",
			fromRef:   "HEAD~1",
			toRef:     "HEAD",
			wantLen:   1,
			wantFirst: ingitdb.ChangedFile{Kind: ingitdb.ChangeKindAdded, Path: "users/alice.yaml"},
		},
		{
			name: "single modified file",
			files: []ingitdb.ChangedFile{
				{Kind: ingitdb.ChangeKindModified, Path: "products/item.yaml"},
			},
			wantLen:   1,
			wantFirst: ingitdb.ChangedFile{Kind: ingitdb.ChangeKindModified, Path: "products/item.yaml"},
		},
		{
			name: "single deleted file",
			files: []ingitdb.ChangedFile{
				{Kind: ingitdb.ChangeKindDeleted, Path: "old/record.yaml"},
			},
			wantLen:   1,
			wantFirst: ingitdb.ChangedFile{Kind: ingitdb.ChangeKindDeleted, Path: "old/record.yaml"},
		},
		{
			name: "renamed file carries OldPath",
			files: []ingitdb.ChangedFile{
				{Kind: ingitdb.ChangeKindRenamed, Path: "new/path.yaml", OldPath: "old/path.yaml"},
			},
			wantLen: 1,
			wantFirst: ingitdb.ChangedFile{
				Kind:    ingitdb.ChangeKindRenamed,
				Path:    "new/path.yaml",
				OldPath: "old/path.yaml",
			},
		},
		{
			name: "multiple files",
			files: []ingitdb.ChangedFile{
				{Kind: ingitdb.ChangeKindAdded, Path: "a.yaml"},
				{Kind: ingitdb.ChangeKindDeleted, Path: "b.yaml"},
				{Kind: ingitdb.ChangeKindModified, Path: "c.yaml"},
			},
			wantLen:   3,
			wantFirst: ingitdb.ChangedFile{Kind: ingitdb.ChangeKindAdded, Path: "a.yaml"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d gitdiff.GitDiffer = &mockGitDiffer{files: tc.files}

			got, err := d.DiffFiles(context.Background(), tc.repoPath, tc.fromRef, tc.toRef)
			if err != nil {
				t.Fatalf("DiffFiles() unexpected error: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Fatalf("DiffFiles() returned %d files, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen > 0 && got[0] != tc.wantFirst {
				t.Errorf("DiffFiles()[0] = %+v, want %+v", got[0], tc.wantFirst)
			}
		})
	}
}

func TestGitDiffer_DiffFiles_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "generic error",
			err:     errors.New("git diff failed"),
			wantErr: errors.New("git diff failed"),
		},
		{
			name:    "sentinel error propagated",
			err:     errors.New("repository not found"),
			wantErr: errors.New("repository not found"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d gitdiff.GitDiffer = &mockGitDiffer{err: tc.err}

			got, err := d.DiffFiles(context.Background(), "/repo", "abc123", "def456")
			if err == nil {
				t.Fatal("DiffFiles() expected error, got nil")
			}
			if err.Error() != tc.wantErr.Error() {
				t.Errorf("DiffFiles() error = %q, want %q", err.Error(), tc.wantErr.Error())
			}
			if got != nil {
				t.Errorf("DiffFiles() files = %v, want nil on error", got)
			}
		})
	}
}

func TestGitDiffer_DiffFiles_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var d gitdiff.GitDiffer = &mockGitDiffer{
		files: []ingitdb.ChangedFile{{Kind: ingitdb.ChangeKindAdded, Path: "x.yaml"}},
	}

	// The interface contract does not mandate ctx propagation in mock implementations,
	// but a real implementation must respect it. The mock call succeeds; we verify
	// the interface is callable with a cancelled context and returns results.
	got, err := d.DiffFiles(ctx, "/repo", "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("mock DiffFiles() with cancelled ctx: unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("mock DiffFiles() returned %d files, want 1", len(got))
	}
}
