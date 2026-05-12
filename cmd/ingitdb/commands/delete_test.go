package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// deleteTestDeps returns a minimal DI set for the Delete command.
func deleteTestDeps(t *testing.T, dir string) (
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

// runDeleteCmd invokes the Delete command with the given args and
// returns captured stdout + any error.
func runDeleteCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	args ...string,
) (string, error) {
	t.Helper()
	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// deleteSeedItem seeds a record under test.items.
func deleteSeedItem(t *testing.T, dir, key string, data map[string]any) { //nolint:unused // used by Tasks 2+
	t.Helper()
	if err := seedRecord(t, dir, "test.items", key, data); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

// itemExists reports whether a record file exists on disk.
func itemExists(t *testing.T, dir, key string) bool { //nolint:unused // used by Tasks 2+
	t.Helper()
	colDef := testDef(dir).Collections["test.items"]
	_, err := os.Stat(filepath.Join(colDef.DirPath, "$records", key+".yaml"))
	return err == nil
}

func TestDelete_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "all", "min-affected", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestDelete_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/x", "--from=test.items",
	)
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
}

func TestDelete_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)
	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir,
	)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}

func TestDelete_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := deleteTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "into rejected", args: []string{"--id=test.items/x", "--into=other"}},
		{name: "set rejected", args: []string{"--id=test.items/x", "--set=foo=bar"}},
		{name: "unset rejected", args: []string{"--id=test.items/x", "--unset=foo"}},
		{name: "order-by rejected", args: []string{"--id=test.items/x", "--order-by=name"}},
		{name: "fields rejected", args: []string{"--id=test.items/x", "--fields=a,b"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}
