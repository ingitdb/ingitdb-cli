package commands

import (
	"bytes"
	"os"
	"path/filepath"
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

func seedItem(t *testing.T, dir, key string, data map[string]any) {
	t.Helper()
	if err := seedRecord(t, dir, "test.items", key, data); err != nil {
		t.Fatalf("seed %s: %v", key, err)
	}
}

func readItem(t *testing.T, dir, key string) string {
	t.Helper()
	colDef := testDef(dir).Collections["test.items"]
	got, err := os.ReadFile(filepath.Join(colDef.DirPath, "$records", key+".yaml"))
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return string(got)
}

func TestUpdate_SingleRecord_Set(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "alpha", map[string]any{"title": "Alpha", "priority": float64(1)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/alpha", "--set=priority=5",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "alpha")
	if !strings.Contains(got, "priority: 5") {
		t.Errorf("expected priority: 5, got:\n%s", got)
	}
	if !strings.Contains(got, "title: Alpha") {
		t.Errorf("title should be preserved, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_MultipleSets(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "beta", map[string]any{"title": "Beta"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/beta",
		"--set=priority=3", "--set=active=true",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "beta")
	if !strings.Contains(got, "priority: 3") {
		t.Errorf("missing priority, got:\n%s", got)
	}
	if !strings.Contains(got, "active: true") {
		t.Errorf("missing active, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_Unset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "gamma", map[string]any{"title": "Gamma", "tmp": "scratch"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/gamma", "--unset=tmp",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "gamma")
	if strings.Contains(got, "tmp:") {
		t.Errorf("tmp field should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "title: Gamma") {
		t.Errorf("title should be preserved, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_SetAndUnset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "delta", map[string]any{"title": "Delta", "draft": true})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/delta",
		"--set=status=published", "--unset=draft",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	got := readItem(t, dir, "delta")
	if !strings.Contains(got, "status: published") {
		t.Errorf("missing status, got:\n%s", got)
	}
	if strings.Contains(got, "draft:") {
		t.Errorf("draft field should be removed, got:\n%s", got)
	}
}

func TestUpdate_SingleRecord_SetUnsetSameFieldRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "epsilon", map[string]any{"title": "Epsilon"})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/epsilon",
		"--set=foo=bar", "--unset=foo",
	)
	if err == nil {
		t.Fatal("expected error when --set and --unset reference the same field")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error should name the conflicting field, got: %v", err)
	}
}

func TestUpdate_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--id=test.items/missing", "--set=foo=bar",
	)
	if err == nil {
		t.Fatal("expected error when record not found")
	}
	if !strings.Contains(err.Error(), "missing") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention the missing id, got: %v", err)
	}
}

func TestUpdate_SetMode_WhereFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1), "active": true})
	seedItem(t, dir, "b", map[string]any{"priority": float64(5), "active": true})
	seedItem(t, dir, "c", map[string]any{"priority": float64(3), "active": true})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=priority>=3",
		"--set=active=false",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !strings.Contains(readItem(t, dir, "a"), "active: true") {
		t.Errorf("record a should be untouched (priority=1)")
	}
	if !strings.Contains(readItem(t, dir, "b"), "active: false") {
		t.Errorf("record b should be patched (priority=5)")
	}
	if !strings.Contains(readItem(t, dir, "c"), "active: false") {
		t.Errorf("record c should be patched (priority=3)")
	}
}

func TestUpdate_SetMode_All(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1)})
	seedItem(t, dir, "b", map[string]any{"priority": float64(2)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--all", "--set=checked=true",
	)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !strings.Contains(readItem(t, dir, "a"), "checked: true") {
		t.Errorf("record a should have checked: true")
	}
	if !strings.Contains(readItem(t, dir, "b"), "checked: true") {
		t.Errorf("record b should have checked: true")
	}
}

func TestUpdate_SetMode_WhereAndAllMutuallyExclusive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=a==1", "--all", "--set=b=2",
	)
	if err == nil {
		t.Fatal("expected error when --where and --all both supplied")
	}
}

func TestUpdate_SetMode_NeitherWhereNorAllRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--set=b=2",
	)
	if err == nil {
		t.Fatal("expected error when set mode has neither --where nor --all")
	}
}

func TestUpdate_SetMode_ZeroMatchesIsSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := updateTestDeps(t, dir)
	seedItem(t, dir, "a", map[string]any{"priority": float64(1)})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--where=priority>1000",
		"--set=checked=true",
	)
	if err != nil {
		t.Errorf("zero matches should succeed (exit 0), got: %v", err)
	}
	// Record should be unchanged.
	if !strings.Contains(readItem(t, dir, "a"), "priority: 1") {
		t.Errorf("record should be unchanged when no matches")
	}
}
