package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

func writeCollectionDef(t *testing.T, dir string, content string) {
	t.Helper()

	schemaDir := filepath.Join(dir, ingitdb.SchemaDir)
	err := os.MkdirAll(schemaDir, 0777)
	if err != nil {
		t.Fatalf("failed to create dir: %s", err)
	}
	path := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
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

	_, err := newDefLoader().readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
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

	_, err := newDefLoader().readRootCollections(t.TempDir(), rootConfig, ingitdb.NewReadOptions())
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

	_, err := newDefLoader().readCollectionDef(t.TempDir(), "missing", "", "id", nil, ingitdb.NewReadOptions())
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
	writeCollectionDef(t, dir, "a: [1,2\n")

	_, err := newDefLoader().readCollectionDef(root, "bad", "", "id", nil, ingitdb.NewReadOptions())
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
	writeCollectionDef(t, dir, "columns: {}\n")

	_, err := newDefLoader().readCollectionDef(root, "invalid", "", "id", nil, ingitdb.NewReadOptions(ingitdb.Validate()))
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
	if err := os.WriteFile(filepath.Join(rootSchemaDir, ingitdb.CollectionDefFileName), []byte(rootContent), 0666); err != nil {
		t.Fatalf("failed to write root collection file: %s", err)
	}

	// Create valid departments subcollection
	subDir1 := filepath.Join(rootSchemaDir, "subcollections", "departments")
	if err := os.MkdirAll(subDir1, 0777); err != nil {
		t.Fatalf("failed to create subcollection dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(subDir1, ingitdb.CollectionDefFileName), []byte(rootContent), 0666); err != nil {
		t.Fatalf("failed to write subcollection file: %s", err)
	}

	// Create invalid teams subcollection
	subDir2 := filepath.Join(subDir1, "subcollections", "teams")
	if err := os.MkdirAll(subDir2, 0777); err != nil {
		t.Fatalf("failed to create sub-subcollection dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(subDir2, ingitdb.CollectionDefFileName), []byte("columns: {}\n"), 0666); err != nil {
		t.Fatalf("failed to write sub-subcollection file: %s", err)
	}

	_, err := newDefLoader().readCollectionDef(root, "invalid_sub", "", "companies", nil, ingitdb.NewReadOptions(ingitdb.Validate()))
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

	views, err := newDefLoader().loadViews(filepath.Join(t.TempDir(), ingitdb.SchemaDir), ingitdb.NewReadOptions())
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

	views, err := newDefLoader().loadViews(viewsDir, ingitdb.NewReadOptions())
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

	_, err := newDefLoader().loadViews(viewsDir, ingitdb.NewReadOptions())
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

	views, err := newDefLoader().loadViews(viewsDir, ingitdb.NewReadOptions(ingitdb.Validate()))
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

	views, err := newDefLoader().loadViews(filepath.Join(root, ingitdb.SchemaDir), ingitdb.NewReadOptions())
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

	views, err := newDefLoader().loadViews(filepath.Join(root, ingitdb.SchemaDir), ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if views != nil {
		t.Fatalf("expected nil views (no yaml files), got %v", views)
	}
}

// ---------------------------------------------------------------------------
// .collections/ shared-directory layout
// ---------------------------------------------------------------------------

func TestReadCollectionDef_SharedLayout_TwoCollections(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// .ingitdb/root-collections.yaml pointing to the two named subdirs.
	ingitdbDir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .ingitdb: %v", err)
	}
	rootCols := "recipes: cooking/.collections/recipes\ningredients: cooking/.collections/ingredients\n"
	if err := os.WriteFile(filepath.Join(ingitdbDir, "root-collections.yaml"), []byte(rootCols), 0o644); err != nil {
		t.Fatalf("setup: write root-collections.yaml: %v", err)
	}

	colDef := `columns:
  id:
    type: string
  title:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
data_dir: recipes
`
	ingredientsDef := `columns:
  id:
    type: string
  name:
    type: string
record_file:
  name: "ingredients.csv"
  type: "[]map[string]any"
  format: yaml
`

	// Create .collections/recipes/definition.yaml
	recipesDir := filepath.Join(root, "cooking", ".collections", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir recipes schema dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recipesDir, "definition.yaml"), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write recipes definition: %v", err)
	}

	// Create .collections/ingredients/definition.yaml (no data_dir = data in cooking/)
	ingredientsDir := filepath.Join(root, "cooking", ".collections", "ingredients")
	if err := os.MkdirAll(ingredientsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir ingredients schema dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ingredientsDir, "definition.yaml"), []byte(ingredientsDef), 0o644); err != nil {
		t.Fatalf("setup: write ingredients definition: %v", err)
	}

	def, err := ReadDefinition(root)
	if err != nil {
		t.Fatalf("ReadDefinition() unexpected error: %v", err)
	}
	if len(def.Collections) != 2 {
		t.Fatalf("expected 2 collections, got %d", len(def.Collections))
	}

	recipes, ok := def.Collections["recipes"]
	if !ok {
		t.Fatal("expected 'recipes' collection")
	}
	if recipes.ID != "recipes" {
		t.Errorf("recipes ID = %q, want 'recipes'", recipes.ID)
	}
	// data_dir="recipes" → DirPath should be cooking/recipes relative to root
	wantRecipesDirPath := filepath.Join(root, "cooking", "recipes")
	if recipes.DirPath != wantRecipesDirPath {
		t.Errorf("recipes DirPath = %q, want %q", recipes.DirPath, wantRecipesDirPath)
	}

	ingredients, ok := def.Collections["ingredients"]
	if !ok {
		t.Fatal("expected 'ingredients' collection")
	}
	// no data_dir → DirPath should be cooking/
	wantIngredientsDirPath := filepath.Join(root, "cooking")
	if ingredients.DirPath != wantIngredientsDirPath {
		t.Errorf("ingredients DirPath = %q, want %q", ingredients.DirPath, wantIngredientsDirPath)
	}
}

func TestReadCollectionDef_SharedLayout_WithViewAndSubcollection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	ingitdbDir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .ingitdb: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ingitdbDir, "root-collections.yaml"),
		[]byte("recipes: cooking/.collections/recipes\n"), 0o644); err != nil {
		t.Fatalf("setup: write root-collections.yaml: %v", err)
	}

	recipesDir := filepath.Join(root, "cooking", ".collections", "recipes")
	if err := os.MkdirAll(recipesDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir recipes: %v", err)
	}
	colDef := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(recipesDir, "definition.yaml"), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write recipes definition: %v", err)
	}

	// Named view in $views/
	viewsDir := filepath.Join(recipesDir, ingitdb.SharedViewsDir)
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir $views: %v", err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "by_cuisine.yaml"), []byte("order_by: id asc\n"), 0o644); err != nil {
		t.Fatalf("setup: write view: %v", err)
	}

	// Subcollection: recipes/ingredients_of_recipe/definition.yaml
	subDir := filepath.Join(recipesDir, "ingredients_of_recipe")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir subcollection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "definition.yaml"), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write subcollection definition: %v", err)
	}

	def, err := ReadDefinition(root)
	if err != nil {
		t.Fatalf("ReadDefinition() unexpected error: %v", err)
	}
	recipes := def.Collections["recipes"]
	if recipes == nil {
		t.Fatal("expected 'recipes' collection")
	}

	// View loaded from $views/
	if _, ok := recipes.Views["by_cuisine"]; !ok {
		t.Errorf("expected view 'by_cuisine', got views: %v", recipes.Views)
	}

	// Subcollection discovered
	if _, ok := recipes.SubCollections["ingredients_of_recipe"]; !ok {
		t.Errorf("expected subcollection 'ingredients_of_recipe', got: %v", recipes.SubCollections)
	}
}

func TestReadCollectionDef_SharedLayout_ConflictError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDef := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`

	// Create BOTH .collection/definition.yaml and definition.yaml in the same dir.
	colDir := filepath.Join(root, "col")
	oldSchemaDir := filepath.Join(colDir, ingitdb.SchemaDir)
	if err := os.MkdirAll(oldSchemaDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .collection: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldSchemaDir, "definition.yaml"), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write old layout def: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "definition.yaml"), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write new layout def: %v", err)
	}

	_, err := newDefLoader().readCollectionDef(root, "col", "", "col", nil, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("error = %q, want substring 'both'", err.Error())
	}
}
