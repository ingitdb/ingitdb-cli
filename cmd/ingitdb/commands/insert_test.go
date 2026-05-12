package commands

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// insertTestDeps returns a minimal DI set for the Insert command.
// stdin/isStdinTTY/openEditor default to inert values; tests that
// exercise those paths override them via runInsertCmd's variants.
func insertTestDeps(t *testing.T, dir string) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return dir, nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB = func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf = func(...any) {}
	return
}

// runInsertCmd invokes the Insert command with stdin set to the given
// reader and stdin-TTY simulation flag, captures stdout, and returns
// the captured output + any error.
func runInsertCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	stdinIsTTY bool,
	openEditor func(string) error,
	args ...string,
) (string, error) {
	t.Helper()
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, stdin, func() bool { return stdinIsTTY }, openEditor)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestInsert_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, strings.NewReader(""), func() bool { return true }, nil)
	for _, name := range []string{"into", "key", "data", "edit", "empty", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestInsert_RequiresInto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--key=x", "--data={}",
	)
	if err == nil {
		t.Fatal("expected error when --into is missing")
	}
	if !strings.Contains(err.Error(), "into") {
		t.Errorf("error should mention --into, got: %v", err)
	}
}

func TestInsert_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
		want string // substring expected in error
	}{
		{name: "from rejected", args: []string{"--into=test.items", "--from=other", "--key=x", "--data={}"}, want: "from"},
		{name: "id rejected", args: []string{"--into=test.items", "--id=test.items/x", "--data={}"}, want: "id"},
		{name: "where rejected", args: []string{"--into=test.items", "--key=x", "--data={}", "--where=a==1"}, want: "where"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("expected error to mention %q, got: %v", tc.want, err)
			}
		})
	}
}
