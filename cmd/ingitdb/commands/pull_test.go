package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// fakePullViewBuilder records BuildViews calls and is a no-op otherwise.
type fakePullViewBuilder struct{ calls int }

var _ materializer.ViewBuilder = (*fakePullViewBuilder)(nil)

func (f *fakePullViewBuilder) BuildViews(_ context.Context, _, _ string, _ *ingitdb.CollectionDef, _ *ingitdb.Definition) (*ingitdb.MaterializeResult, error) {
	f.calls++
	return &ingitdb.MaterializeResult{}, nil
}

func newPullCmd(vb materializer.ViewBuilder, readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error), logf func(...any)) *cobra.Command {
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	if readDef == nil {
		readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
			return &ingitdb.Definition{}, nil
		}
	}
	if logf == nil {
		logf = func(...any) {}
	}
	return Pull(homeDir, getWd, readDef, vb, logf, func() bool { return false },
		func(context.Context, []string) error { return nil })
}

func TestPull_ReturnsCommand(t *testing.T) {
	t.Parallel()
	cmd := newPullCmd(&fakePullViewBuilder{}, nil, nil)
	if cmd == nil {
		t.Fatal("Pull() returned nil")
		return
	}
	if cmd.Use != "pull" {
		t.Errorf("expected name 'pull', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected RunE to be set")
	}
}

func TestPull_InvalidStrategy(t *testing.T) {
	t.Parallel()
	cmd := newPullCmd(&fakePullViewBuilder{}, nil, nil)
	err := runCobraCommand(cmd, "--strategy=bogus")
	if err == nil || !strings.Contains(err.Error(), "strategy") {
		t.Fatalf("expected invalid-strategy error, got: %v", err)
	}
}

func TestPullArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                     string
		strategy, remote, branch string
		want                     []string
	}{
		{"defaults to rebase + origin", "", "", "", []string{"pull", "--rebase", "origin"}},
		{"merge strategy", "merge", "", "", []string{"pull", "--no-rebase", "origin"}},
		{"explicit remote + branch", "rebase", "upstream", "main", []string{"pull", "--rebase", "upstream", "main"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pullArgs(tc.strategy, tc.remote, tc.branch)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("pullArgs(%q,%q,%q) = %v, want %v", tc.strategy, tc.remote, tc.branch, got, tc.want)
			}
		})
	}
}

// TestPull_SyncsResolvesRebuildsAndSummarizes is the end-to-end pipeline test:
// a clone behind its upstream pulls a newly-added record, the view builder is
// invoked, and the summary reports the added record.
func TestPull_SyncsRebuildsAndSummarizes(t *testing.T) {
	base := t.TempDir()

	// Bare upstream.
	runGit(t, base, "init", "--bare", "upstream.git")
	upstream := filepath.Join(base, "upstream.git")

	// Working clone: seed an initial record and push.
	work := filepath.Join(base, "work")
	runGit(t, base, "clone", upstream, "work")
	disableGitBackgroundMaintenance(t, work)
	runGit(t, work, "config", "user.email", "t@example.com")
	runGit(t, work, "config", "user.name", "T")
	writePullRecord(t, work, "alice", "name: Alice\n")
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-m", "seed alice")
	runGit(t, work, "push", "-u", "origin", "HEAD")

	// Publisher clone: add bob and push it upstream.
	pub := filepath.Join(base, "pub")
	runGit(t, base, "clone", upstream, "pub")
	runGit(t, pub, "config", "user.email", "p@example.com")
	runGit(t, pub, "config", "user.name", "P")
	writePullRecord(t, pub, "bob", "name: Bob\n")
	runGit(t, pub, "add", ".")
	runGit(t, pub, "commit", "-m", "add bob")
	runGit(t, pub, "push", "origin", "HEAD")

	// bob must not exist in work yet.
	if _, err := os.Stat(filepath.Join(work, "people", "$records", "bob.yaml")); err == nil {
		t.Fatal("precondition: bob.yaml should not exist in work before pull")
	}

	def := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"people": {
					ID:      "people",
					DirPath: filepath.Join(work, "people"),
					RecordFile: &ingitdb.RecordFileDef{
						Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
					},
					Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
				},
			},
		}, nil
	}

	vb := &fakePullViewBuilder{}
	var logs []string
	logf := func(args ...any) {
		logs = append(logs, fmt.Sprint(args...))
	}

	homeDir := func() (string, error) { return base, nil }
	getWd := func() (string, error) { return work, nil }
	cmd := Pull(homeDir, getWd, def, vb, logf, func() bool { return false },
		func(context.Context, []string) error { return nil })

	if err := runCobraCommand(cmd, "--path="+work); err != nil {
		t.Fatalf("pull: %v", err)
	}

	// bob was pulled in.
	if _, err := os.Stat(filepath.Join(work, "people", "$records", "bob.yaml")); err != nil {
		t.Errorf("expected bob.yaml to be pulled into work: %v", err)
	}
	// Views were rebuilt.
	if vb.calls == 0 {
		t.Error("expected the view builder to be invoked")
	}
	// Summary mentions one added record file.
	joined := strings.Join(logs, " | ")
	if !strings.Contains(joined, "1 record file(s) added") {
		t.Errorf("expected summary to report 1 added record file, logs: %s", joined)
	}
}

func writePullRecord(t *testing.T, repo, key, content string) {
	t.Helper()
	dir := filepath.Join(repo, "people", "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", key, err)
	}
}
