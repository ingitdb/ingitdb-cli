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

const (
	tagsDefinitionYAML = `titles:
  en: Tags

record_file:
  name: "tags.json"
  type: "map[$record_id]map[$field_name]any"
  format: json

columns:
  title:
    type: string
    locale: en
    required: true

  titles:
    type: "map[locale]string"
    required: false

  description:
    type: string
    required: false
`
	tagsViewYAML = `order_by: title
columns:
  - title
  - description
template: .ingitdb-view.README.md
file_name: README.md
records_var_name: tags
`
	tagsViewTemplate = "# Tags\n\n| Title | Description |\n| ----- | ----------- |\n" +
		"{{- range .tags }}\n| **{{ .title }}** | {{ .description }} |\n{{- end }}\n"
)

// TestCRUDRecord_UpdatesTagsReadme builds a minimal todo.tags collection
// from scratch (no external fixtures) and verifies that Insert, Update,
// and Delete each update tags.json and re-materialize the README view.
func TestCRUDRecord_UpdatesTagsReadme(t *testing.T) {
	tmpDir := t.TempDir()

	// Lay out a minimal shared-layout collection at tmpDir/tags-data/.
	// DirPath resolves to the parent of .collections/ (tmpDir/tags-data),
	// so tags.json and README.md are written there.
	dataDir := filepath.Join(tmpDir, "tags-data")
	schemaDir := filepath.Join(dataDir, ".collections", "tags")
	viewsDir := filepath.Join(schemaDir, "$views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	writeFile(t, filepath.Join(schemaDir, "definition.yaml"), tagsDefinitionYAML)
	writeFile(t, filepath.Join(viewsDir, "README.yaml"), tagsViewYAML)
	writeFile(t, filepath.Join(dataDir, ".ingitdb-view.README.md"), tagsViewTemplate)

	ingitDBDir := filepath.Join(tmpDir, ".ingitdb")
	if err := os.MkdirAll(ingitDBDir, 0o755); err != nil {
		t.Fatalf("create .ingitdb dir: %v", err)
	}
	rootCollections := []byte("todo.tags: tags-data/.collections/tags\n")
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

	insertCmd := Insert(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	if err := runCobraCommand(insertCmd, "--path="+tmpDir, "--into=todo.tags", "--key=urgent", `--data={"title": "Urgent"}`); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	assertTagTitle(t, dataDir, "urgent", "Urgent")
	assertReadmeContains(t, dataDir, "**Urgent**")

	updateCmd := Update(homeDir, getWd, readDef, newDB, logf)
	if err := runCobraCommand(updateCmd, "--path="+tmpDir, "--id=todo.tags/urgent", "--set=titles={en: Updated}"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	assertTagTitle(t, dataDir, "urgent", "Updated")
	assertReadmeContains(t, dataDir, "**Updated**")

	deleteCmd := Delete(homeDir, getWd, readDef, newDB, logf)
	if err := runCobraCommand(deleteCmd, "--path="+tmpDir, "--id=todo.tags/urgent"); err != nil {
		t.Fatalf("Delete record: %v", err)
	}
	assertTagMissing(t, dataDir, "urgent")
	assertReadmeNotContains(t, dataDir, "**Updated**")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
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
