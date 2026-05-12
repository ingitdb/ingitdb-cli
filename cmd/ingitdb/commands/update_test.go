package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// updateTestDeps returns a minimal DI set for the Update command.
func updateTestDeps(t *testing.T, dir string) (
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

// runUpdateCmd invokes the new Update command with the given args
// and returns captured stdout + any error.
func runUpdateCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestUpdate_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	cmd := Update(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "set", "unset", "all", "min-affected", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestUpdate_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x", "--from=test.items", "--set=a=1",
	)
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
}

func TestUpdate_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--set=a=1",
	)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}

func TestUpdate_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "into rejected", args: []string{"--id=test.items/x", "--into=other", "--set=a=1"}},
		{name: "order-by rejected", args: []string{"--id=test.items/x", "--order-by=name", "--set=a=1"}},
		{name: "fields rejected", args: []string{"--id=test.items/x", "--fields=a,b", "--set=a=1"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}

func TestUpdate_NoPatchRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x",
	)
	if err == nil {
		t.Fatal("expected error when neither --set nor --unset supplied")
	}
	if !strings.Contains(err.Error(), "set") || !strings.Contains(err.Error(), "unset") {
		t.Errorf("error should mention both --set and --unset, got: %v", err)
	}
}
