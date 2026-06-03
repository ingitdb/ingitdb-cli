package commands

// specscore: feature/computed-columns-via-dalgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AC: delete-where-on-computed — delete in set mode filters on a computed
// column's value, deleting exactly the matching records.
func TestDelete_SetMode_WhereOnComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := peopleSelectDeps(t, dir)
	seedPerson(t, dir, "ada", "Ada", "Lovelace")
	seedPerson(t, dir, "alan", "Alan", "Turing")

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", `--where=full_name == "Ada Lovelace"`)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "ada.yaml")); !os.IsNotExist(statErr) {
		t.Errorf("ada.yaml should have been deleted (matched computed full_name)")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "alan.yaml")); statErr != nil {
		t.Errorf("alan.yaml should remain (did not match): %v", statErr)
	}
}

// delete --where referencing an erroring computed column fails loud.
func TestDelete_SetMode_WhereOnErroringComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := ratioSelectDeps(t, dir)
	seedPersonFields(t, dir, "a", map[string]any{"qty": 3})

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--where=ratio == 1")
	if err == nil {
		t.Fatal("delete --where on an erroring computed column must fail loud")
	}
	if !strings.Contains(err.Error(), "ratio") {
		t.Errorf("error should name the ratio column, got: %v", err)
	}
}

// delete --where on the $id pseudo-field exercises the whereColumnNames $id skip
// (the key is resolved from the row, not read as a recordset column).
func TestDelete_SetMode_WhereOnID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := peopleSelectDeps(t, dir)
	seedPerson(t, dir, "ada", "Ada", "Lovelace")
	seedPerson(t, dir, "alan", "Alan", "Turing")

	_, err := runDeleteCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", `--where=$id == "ada"`)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "ada.yaml")); !os.IsNotExist(statErr) {
		t.Errorf("ada.yaml should have been deleted by $id match")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "alan.yaml")); statErr != nil {
		t.Errorf("alan.yaml should remain: %v", statErr)
	}
}
