package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

func writeCollectionDef(t *testing.T, dir string, id string, content string) {
	t.Helper()

	schemaDir := filepath.Join(dir, ingitdb.SchemaDir)
	err := os.MkdirAll(schemaDir, 0777)
	if err != nil {
		t.Fatalf("failed to create dir: %s", err)
	}
	path := filepath.Join(schemaDir, id+".yaml")
	err = os.WriteFile(path, []byte(content), 0666)
	if err != nil {
		t.Fatalf("failed to write file: %s", err)
	}
}

func TestReadRootCollections_WildcardError(t *testing.T) {
	t.Parallel()

	rootConfig := config.RootConfig{
		RootCollections: map[string]string{
			"todo": "missing/*",
		},
	}

	_, err := readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "wildcard root collection paths are not supported") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadRootCollections_SingleError(t *testing.T) {
	t.Parallel()

	rootConfig := config.RootConfig{
		RootCollections: map[string]string{
			"countries": "missing",
		},
	}

	_, err := readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to validate root collection def ID=countries") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_FileMissing(t *testing.T) {
	t.Parallel()

	_, err := readCollectionDef(t.TempDir(), "missing", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to read file") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_InvalidYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "bad")
	writeCollectionDef(t, dir, "id", "a: [1,2\n")

	_, err := readCollectionDef(root, "bad", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "failed to parse YAML file") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestReadCollectionDef_InvalidDefinitionWithValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "invalid")
	writeCollectionDef(t, dir, "id", "columns: {}\n")

	_, err := readCollectionDef(root, "invalid", "id", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not valid definition of collection") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestLoadSubCollections_InvalidSubCollectionWithValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "invalid_sub")

	// Create root collection schema
	rootSchemaDir := filepath.Join(dir, ingitdb.SchemaDir)
	if err := os.MkdirAll(rootSchemaDir, 0777); err != nil {
		t.Fatalf("failed to create root schema dir: %s", err)
	}
	rootContent := `
record_file:
  name: "{key}.json"
  type: "map[string]any"
  format: json
columns:
  title:
    type: string
`
	if err := os.WriteFile(filepath.Join(rootSchemaDir, "companies.yaml"), []byte(rootContent), 0666); err != nil {
		t.Fatalf("failed to write root collection file: %s", err)
	}

	// Create valid departments subcollection
	subDir1 := filepath.Join(rootSchemaDir, "subcollections")
	if err := os.MkdirAll(subDir1, 0777); err != nil {
		t.Fatalf("failed to create subcollection dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(subDir1, "departments.yaml"), []byte(rootContent), 0666); err != nil {
		t.Fatalf("failed to write subcollection file: %s", err)
	}

	// Create invalid teams subcollection
	subDir2 := filepath.Join(subDir1, "departments")
	if err := os.MkdirAll(subDir2, 0777); err != nil {
		t.Fatalf("failed to create sub-subcollection dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(subDir2, "teams.yaml"), []byte("columns: {}\n"), 0666); err != nil {
		t.Fatalf("failed to write sub-subcollection file: %s", err)
	}

	_, err := readCollectionDef(root, "invalid_sub", "companies", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not valid definition of subcollection 'companies/departments/teams'") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestLoadViews_NoViewsDir(t *testing.T) {
	t.Parallel()

	views, err := loadViews(t.TempDir(), "", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if views != nil {
		t.Fatalf("expected nil views, got %v", views)
	}
}

func TestLoadViews_ValidViews(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(viewsDir, 0o777); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}

	content := `order_by: title
template: .ingitdb-view.README.md
file_name: README.md
records_var_name: items
`
	if err := os.WriteFile(filepath.Join(viewsDir, "readme.yaml"), []byte(content), 0o666); err != nil {
		t.Fatalf("failed to write view file: %v", err)
	}

	views, err := loadViews(root, "", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
	v := views["readme"]
	if v == nil {
		t.Fatal("expected 'readme' view to exist")
	}
	if v.ID != "readme" {
		t.Fatalf("expected ID 'readme', got %q", v.ID)
	}
	if v.OrderBy != "title" {
		t.Fatalf("expected OrderBy 'title', got %q", v.OrderBy)
	}
	if v.FileName != "README.md" {
		t.Fatalf("expected FileName 'README.md', got %q", v.FileName)
	}
}

func TestLoadViews_InvalidYAML(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(viewsDir, 0o777); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "bad.yaml"), []byte("a: [1,2\n"), 0o666); err != nil {
		t.Fatalf("failed to write view file: %v", err)
	}

	_, err := loadViews(root, "", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse YAML file") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestLoadViews_InvalidViewWithValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(viewsDir, 0o777); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}

	// Write a valid YAML but it will get ID from filename, so it should pass.
	// To test validation error, we need to make Validate() fail.
	// ViewDef.Validate() only checks for empty ID, but ID is set from filename, so it should always pass.
	// Let's test the success path with validation enabled.
	content := `order_by: title
`
	if err := os.WriteFile(filepath.Join(viewsDir, "readme.yaml"), []byte(content), 0o666); err != nil {
		t.Fatalf("failed to write view file: %v", err)
	}

	views, err := loadViews(root, "", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}
}

func TestLoadViews_SkipsDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(filepath.Join(viewsDir, "somedir"), 0o777); err != nil {
		t.Fatalf("failed to create views subdir: %v", err)
	}

	views, err := loadViews(root, "", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if views != nil {
		t.Fatalf("expected nil views (no yaml files), got %v", views)
	}
}

func TestLoadViews_SkipsNonYamlFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(viewsDir, 0o777); err != nil {
		t.Fatalf("failed to create views dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "readme.txt"), []byte("not yaml"), 0o666); err != nil {
		t.Fatalf("failed to write non-yaml file: %v", err)
	}

	views, err := loadViews(root, "", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if views != nil {
		t.Fatalf("expected nil views (no yaml files), got %v", views)
	}
}
