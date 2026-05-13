package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// dropTestDeps returns a minimal DI set for the Drop command.
func dropTestDeps(t *testing.T, dir string) (
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

// runDropCmd invokes the Drop command with the given args and returns
// captured stdout + any error.
func runDropCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestDrop_RequiresKindAndName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "no args at all", args: []string{"--path=" + dir}},
		{name: "only kind, no name", args: []string{"--path=" + dir, "collection"}},
		{name: "unknown kind", args: []string{"--path=" + dir, "unknownkind", "x"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf, tc.args...)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}

func TestDrop_RegistersIfExistsAndCascade(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)

	// PersistentFlags must include --if-exists and --cascade so that
	// both subcommands inherit them.
	for _, name := range []string{"if-exists", "cascade", "path"} {
		if cmd.PersistentFlags().Lookup(name) == nil && cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered on drop", name)
		}
	}
}

func TestDrop_CollectionPlaceholder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "collection", "test.items",
	)
	if err == nil {
		t.Fatal("expected 'not yet implemented' until Task 2 lands")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' diagnostic, got: %v", err)
	}
}

func TestDrop_ViewPlaceholder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := dropTestDeps(t, dir)
	_, err := runDropCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "view", "some_view",
	)
	if err == nil {
		t.Fatal("expected 'not yet implemented' until Task 3 lands")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' diagnostic, got: %v", err)
	}
}
