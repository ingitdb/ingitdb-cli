package commands

import (
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// selectTestDeps returns a minimal DI set for the Select command.
func selectTestDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	return homeDir, getWd, readDef, newDB, logf
}

func TestSelect_RegistersAllSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	for _, name := range []string{"id", "from", "where", "order-by", "fields", "limit", "min-affected", "format", "path", "github"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestSelect_RejectsBothIDAndFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--id=todo.items/x", "--from=todo.items")
	if err == nil {
		t.Fatal("expected error when both --id and --from supplied")
	}
	if !strings.Contains(err.Error(), "--id") && !strings.Contains(err.Error(), "--from") {
		t.Errorf("error should name --id or --from, got: %v", err)
	}
}

func TestSelect_RejectsNeitherIDNorFrom(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := selectTestDeps(t, dir)
	cmd := Select(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when neither --id nor --from supplied")
	}
}
