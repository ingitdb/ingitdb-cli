package commands

// specscore: feature/computed-columns-via-dalgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// update set-mode filters on a computed column and patches the matching record;
// the computed column is never written back (the write path rejects stored
// computed values), so the migration must persist stored fields only.
func TestUpdate_SetMode_WhereOnComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := peopleSelectDeps(t, dir)
	seedPerson(t, dir, "ada", "Ada", "Lovelace")
	seedPerson(t, dir, "alan", "Alan", "Turing")

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", `--where=full_name == "Ada Lovelace"`,
		"--set=last_name=Byron")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "$records", "ada.yaml"))
	if err != nil {
		t.Fatalf("read ada.yaml: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["last_name"] != "Byron" {
		t.Errorf("last_name = %v, want Byron", got["last_name"])
	}
	if _, hasComputed := got["full_name"]; hasComputed {
		t.Errorf("computed full_name must NOT be persisted, got: %v", got["full_name"])
	}

	// alan untouched.
	rawAlan, _ := os.ReadFile(filepath.Join(dir, "$records", "alan.yaml"))
	if !strings.Contains(string(rawAlan), "Turing") {
		t.Errorf("alan.yaml should be unchanged, got: %s", rawAlan)
	}
}

// update --where referencing an erroring computed column fails loud (covers the
// RowData error branch in the update read loop).
func TestUpdate_SetMode_WhereOnErroringComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := ratioSelectDeps(t, dir)
	seedPersonFields(t, dir, "a", map[string]any{"qty": 3})

	_, err := runUpdateCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--where=ratio == 1", "--set=qty=5")
	if err == nil {
		t.Fatal("update --where on an erroring computed column must fail loud")
	}
	if !strings.Contains(err.Error(), "ratio") {
		t.Errorf("error should name the ratio column, got: %v", err)
	}
}
