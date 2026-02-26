package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

func TestCRUDRecord_UpdatesTagsReadme(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()
	dstTagsDir := filepath.Join(tmpDir, "docs", "demo-apps", "todo", "tags")
	srcTagsDir := filepath.Join(repoRoot, "docs", "demo-apps", "todo", "tags")
	if err := copyDir(srcTagsDir, dstTagsDir); err != nil {
		t.Fatalf("copy tags dir: %v", err)
	}
	ingitDBDir := filepath.Join(tmpDir, ".ingitdb")
	if err := os.MkdirAll(ingitDBDir, 0755); err != nil {
		t.Fatalf("create .ingitdb dir: %v", err)
	}
	rootCollections := []byte("todo.tags: docs/demo-apps/todo/tags\n")
	if err := os.WriteFile(filepath.Join(ingitDBDir, "root-collections.yaml"), rootCollections, 0o644); err != nil {
		t.Fatalf("write root collections: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := validator.ReadDefinition
	newDB := func(root string, def *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, def)
	}
	logf := func(...any) {}

	createCmd := Create(homeDir, getWd, readDef, newDB, logf)
	if err := runCLICommand(createCmd, "record", "--path="+tmpDir, "--id=todo.tags/urgent", "--data={title: Urgent}"); err != nil {
		t.Fatalf("Create record: %v", err)
	}
	assertTagTitle(t, dstTagsDir, "urgent", "Urgent")
	assertReadmeContains(t, dstTagsDir, "**Urgent**")

	updateCmd := Update(homeDir, getWd, readDef, newDB, logf)
	if err := runCLICommand(updateCmd, "record", "--path="+tmpDir, "--id=todo.tags/urgent", "--set={titles: {en: Updated}}"); err != nil {
		t.Fatalf("Update record: %v", err)
	}
	assertTagTitle(t, dstTagsDir, "urgent", "Updated")
	assertReadmeContains(t, dstTagsDir, "**Updated**")

	deleteCmd := Delete(homeDir, getWd, readDef, newDB, logf)
	if err := runCLICommand(deleteCmd, "record", "--path="+tmpDir, "--id=todo.tags/urgent"); err != nil {
		t.Fatalf("Delete record: %v", err)
	}
	assertTagMissing(t, dstTagsDir, "urgent")
	assertReadmeNotContains(t, dstTagsDir, "**Updated**")
}

func assertTagTitle(t *testing.T, tagsDir, key, title string) {
	t.Helper()
	data := readTagsJSON(t, tagsDir)
	record, ok := data[key]
	if !ok {
		t.Fatalf("expected tag %q to exist", key)
	}
	value, ok := record["title"].(string)
	if !ok {
		t.Fatalf("expected tag %q to have title", key)
	}
	if value != title {
		t.Fatalf("expected tag %q title %q, got %q", key, title, value)
	}
}

func assertTagMissing(t *testing.T, tagsDir, key string) {
	t.Helper()
	data := readTagsJSON(t, tagsDir)
	if _, ok := data[key]; ok {
		t.Fatalf("expected tag %q to be removed", key)
	}
}

func readTagsJSON(t *testing.T, tagsDir string) map[string]map[string]any {
	t.Helper()
	path := filepath.Join(tagsDir, "tags.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tags.json: %v", err)
	}
	var data map[string]map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		t.Fatalf("parse tags.json: %v", err)
	}
	return data
}

func assertReadmeContains(t *testing.T, tagsDir, needle string) {
	t.Helper()
	content := readReadme(t, tagsDir)
	if !strings.Contains(content, needle) {
		t.Fatalf("expected README to include %s, got:\n%s", needle, content)
	}
}

func assertReadmeNotContains(t *testing.T, tagsDir, needle string) {
	t.Helper()
	content := readReadme(t, tagsDir)
	if strings.Contains(content, needle) {
		t.Fatalf("expected README to exclude %s, got:\n%s", needle, content)
	}
}

func readReadme(t *testing.T, tagsDir string) string {
	t.Helper()
	readmePath := filepath.Join(tagsDir, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	return string(content)
}
