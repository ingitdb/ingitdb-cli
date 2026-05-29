package validator

// Tests that exercise the defLoader seam (injectable readFile / readDir) and
// the two remaining error paths in ReadDefinition that require special filesystem
// setup.  Each test targets one or more of the previously-uncovered lines.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// ReadDefinition – error from config.ReadRootConfigFromFile  (line 29-31)
// ---------------------------------------------------------------------------

// TestReadDefinition_RootConfigError covers the branch where
// config.ReadRootConfigFromFile returns an error (bad settings.yaml content).
func TestReadDefinition_RootConfigError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	ingitdbDir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("setup: create .ingitdb dir: %v", err)
	}

	// settings.yaml with invalid YAML syntax → readSettingsFromFile returns an error
	settingsPath := filepath.Join(ingitdbDir, "settings.yaml")
	if err := os.WriteFile(settingsPath, []byte("languages: [bad yaml\n"), 0o644); err != nil {
		t.Fatalf("setup: write settings.yaml: %v", err)
	}

	_, err := ReadDefinition(root)
	if err == nil {
		t.Fatal("ReadDefinition() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read root config from") {
		t.Errorf("ReadDefinition() error = %q, want substring %q", err.Error(), "failed to read root config from")
	}
}

// ---------------------------------------------------------------------------
// ReadDefinition – error from ReadSubscribers  (line 38-40)
// ---------------------------------------------------------------------------

// TestReadDefinition_SubscribersError covers the branch where ReadSubscribers
// returns an error.  We create a valid DB layout but place an invalid
// subscribers.yaml so ReadSubscribers fails after collections load fine.
func TestReadDefinition_SubscribersError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// .ingitdb directory with a valid root-collections.yaml
	ingitdbDir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(ingitdbDir, 0o755); err != nil {
		t.Fatalf("setup: create .ingitdb dir: %v", err)
	}
	rootColPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	if err := os.WriteFile(rootColPath, []byte("items: items\n"), 0o644); err != nil {
		t.Fatalf("setup: write root-collections.yaml: %v", err)
	}

	// Valid collection definition
	schemaDir := filepath.Join(root, "items", ingitdb.SchemaDir)
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("setup: create collection schema dir: %v", err)
	}
	colDef := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(schemaDir, ingitdb.CollectionDefFileName), []byte(colDef), 0o644); err != nil {
		t.Fatalf("setup: write collection definition: %v", err)
	}

	// subscribers.yaml with content that cannot be decoded (unknown field triggers KnownFields error)
	subsPath := filepath.Join(ingitdbDir, "subscribers.yaml")
	if err := os.WriteFile(subsPath, []byte("unknown_field: value\n"), 0o644); err != nil {
		t.Fatalf("setup: write subscribers.yaml: %v", err)
	}

	_, err := ReadDefinition(root)
	if err == nil {
		t.Fatal("ReadDefinition() expected error, got nil")
	}
	// The error propagates unchanged from ReadSubscribers.
	if !strings.Contains(err.Error(), "failed to parse subscribers config file") {
		t.Errorf("ReadDefinition() error = %q, want to contain %q", err.Error(), "failed to parse subscribers config file")
	}
}

// ---------------------------------------------------------------------------
// loadSubCollections – readDir returns a non-NotExist error  (line 147-149)
// ---------------------------------------------------------------------------

// TestLoadSubCollections_ReadDirError covers the branch where os.ReadDir on the
// subcollections path fails with something other than "not exist".
func TestLoadSubCollections_ReadDirError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("read dir exploded")
	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, sentinel
		},
	}

	_, err := dl.loadSubCollections("/root", "rel", nil, "parent", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("loadSubCollections() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read subcollections directory") {
		t.Errorf("got error %q, want substring %q", err.Error(), "failed to read subcollections directory")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadSubCollections – non-directory entry is skipped  (line 154-156 continue)
// ---------------------------------------------------------------------------

// TestLoadSubCollections_SkipsNonDirEntries covers the `continue` branch that
// skips regular files in the subcollections directory.
func TestLoadSubCollections_SkipsNonDirEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDir := filepath.Join(root, "col")

	// Create .collection/subcollections/ with a regular file (not a dir)
	subColsDir := filepath.Join(colDir, ingitdb.SchemaDir, "subcollections")
	if err := os.MkdirAll(subColsDir, 0o755); err != nil {
		t.Fatalf("setup: create subcollections dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subColsDir, "not-a-dir.yaml"), []byte("key: val\n"), 0o644); err != nil {
		t.Fatalf("setup: write file in subcollections: %v", err)
	}

	result, err := newDefLoader().loadSubCollections(root, "col", nil, "parent", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("loadSubCollections() unexpected error: %v", err)
	}
	// The file should have been skipped; no subcollections loaded.
	if len(result) != 0 {
		t.Errorf("expected 0 subcollections, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// loadViews – readDir returns a non-NotExist error  (line 179-181)
// ---------------------------------------------------------------------------

// TestLoadViews_ReadDirError covers the branch where os.ReadDir on the views
// path fails with something other than "not exist".
func TestLoadViews_ReadDirError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("readdir failed")
	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, sentinel
		},
	}

	_, err := dl.loadViews("/some/schema", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("loadViews() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read views directory") {
		t.Errorf("got error %q, want substring %q", err.Error(), "failed to read views directory")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadViews – readFile returns an error for a view file  (line 193-195)
// ---------------------------------------------------------------------------

// TestLoadViews_ReadFileError covers the branch where os.ReadFile fails while
// reading an individual view YAML file.
func TestLoadViews_ReadFileError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("setup: create views dir: %v", err)
	}
	// Create a real file so the real os.ReadDir can list it …
	if err := os.WriteFile(filepath.Join(viewsDir, "myview.yaml"), []byte("order_by: id\n"), 0o644); err != nil {
		t.Fatalf("setup: write view file: %v", err)
	}

	sentinel := errors.New("file read failed")
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return nil, sentinel },
		readDir:  os.ReadDir,
	}

	_, err := dl.loadViews(viewsDir, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("loadViews() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("got error %q, want substring %q", err.Error(), "failed to read file")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadViews – viewDef.Validate() returns an error  (line 204-206)
// ---------------------------------------------------------------------------

// TestLoadViews_ViewValidationError covers the branch where viewDef.Validate()
// fails when validation is enabled.  ViewDef.Validate rejects invalid format
// names, which we supply here.
func TestLoadViews_ViewValidationError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	viewsDir := filepath.Join(root, ingitdb.SchemaDir, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("setup: create views dir: %v", err)
	}

	// format value "badformat" is not in the allowed set → Validate() returns error
	content := "format: badformat\n"
	if err := os.WriteFile(filepath.Join(viewsDir, "broken.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write view file: %v", err)
	}

	_, err := newDefLoader().loadViews(viewsDir, ingitdb.NewReadOptions(ingitdb.Validate()))
	if err == nil {
		t.Fatal("loadViews() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not valid definition of view") {
		t.Errorf("got error %q, want substring %q", err.Error(), "not valid definition of view")
	}
}

// ---------------------------------------------------------------------------
// readCollectionDef – loadViews returns an error  (line 117-120)
// ---------------------------------------------------------------------------

// TestReadCollectionDef_LoadViewsError covers the "failed to load views for"
// branch in readCollectionDef by injecting a readDir that errors on the views
// directory while still allowing the definition file itself to be read.
func TestReadCollectionDef_LoadViewsError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDir := filepath.Join(root, "mycol")
	schemaDir := filepath.Join(colDir, ingitdb.SchemaDir)
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatalf("setup: create schema dir: %v", err)
	}

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(schemaDir, ingitdb.CollectionDefFileName), []byte(colDefContent), 0o644); err != nil {
		t.Fatalf("setup: write collection definition: %v", err)
	}

	sentinel := errors.New("views readdir failed")
	dl := defLoader{
		readFile: os.ReadFile, // real ReadFile so definition.yaml is read OK
		readDir: func(path string) ([]os.DirEntry, error) {
			// Return an error only for the views directory; subcollections may
			// also call readDir – let those return NotExist so they are skipped.
			if strings.HasSuffix(path, "views") {
				return nil, sentinel
			}
			// Simulate "not exist" for all other readDir calls (subcollections).
			return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
		},
	}

	_, err := dl.readCollectionDef(root, "mycol", "", "mycol", nil, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("readCollectionDef() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load views for") {
		t.Errorf("got error %q, want substring %q", err.Error(), "failed to load views for")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDef – subPath > 0, readFile fails  (line 92-94)
// ---------------------------------------------------------------------------

func TestReadCollectionDef_SubPathReadFileError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("subcol readfile boom")
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return nil, sentinel },
		readDir:  os.ReadDir,
	}

	_, err := dl.readCollectionDef("/root", "rel", "parent", "sub", []string{"child"}, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("got error %q, want substring 'failed to read file'", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDef – both layouts missing, oldErr is NOT os.ErrNotExist  (line 113-115)
// ---------------------------------------------------------------------------

func TestReadCollectionDef_OldReadNonNotExistError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("permission denied")
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return nil, sentinel },
		readDir:  os.ReadDir,
	}

	_, err := dl.readCollectionDef("/root", "rel", "", "id", nil, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("got error %q, want substring 'failed to read file'", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDef – new layout, loadSubCollectionsShared fails  (line 164-166)
// ---------------------------------------------------------------------------

func TestReadCollectionDef_NewLayout_SubCollectionsSharedError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDir := filepath.Join(root, "mycol")
	// New layout: definition.yaml directly in colDir
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(colDir, ingitdb.CollectionDefFileName), []byte(colDefContent), 0o644); err != nil {
		t.Fatalf("setup: write definition: %v", err)
	}

	sentinel := errors.New("subcol shared boom")
	callCount := 0
	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(path string) ([]os.DirEntry, error) {
			callCount++
			if callCount == 1 {
				// First readDir call is loadSubCollectionsShared
				return nil, sentinel
			}
			return os.ReadDir(path)
		},
	}

	_, err := dl.readCollectionDef(root, "mycol", "", "mycol", nil, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load subcollections for") {
		t.Errorf("got error %q, want substring 'failed to load subcollections for'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDef – new layout, loadViews fails  (line 168-170)
// ---------------------------------------------------------------------------

func TestReadCollectionDef_NewLayout_LoadViewsError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDir := filepath.Join(root, "mycol")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(colDir, ingitdb.CollectionDefFileName), []byte(colDefContent), 0o644); err != nil {
		t.Fatalf("setup: write definition: %v", err)
	}

	sentinel := errors.New("views boom")
	callCount := 0
	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(path string) ([]os.DirEntry, error) {
			callCount++
			if callCount == 1 {
				// First readDir = loadSubCollectionsShared → no subcollections
				return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
			}
			// Second readDir = loadViews → error
			return nil, sentinel
		},
	}

	_, err := dl.readCollectionDef(root, "mycol", "", "mycol", nil, ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load views for") {
		t.Errorf("got error %q, want substring 'failed to load views for'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – readFile fails  (line 199-201)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_ReadFileError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("readfile boom")
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return nil, sentinel },
		readDir:  os.ReadDir,
	}

	_, err := dl.readCollectionDefShared("/schema", "/data", "", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Errorf("got error %q, want substring 'failed to read file'", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – YAML unmarshal fails  (line 204-206)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_InvalidYAML(t *testing.T) {
	t.Parallel()

	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte("a: [1,2\n"), nil },
		readDir:  os.ReadDir,
	}

	_, err := dl.readCollectionDefShared("/schema", "/data", "", "id", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse YAML file") {
		t.Errorf("got error %q, want substring 'failed to parse YAML file'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – DataDir set  (line 209-211)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_WithDataDir(t *testing.T) {
	t.Parallel()

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
data_dir: mydata
`
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte(colDefContent), nil },
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
		},
	}

	colDef, err := dl.readCollectionDefShared("/schema/mycol", "/data", "", "mycol", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantDirPath := filepath.Join("/data", "mydata")
	if colDef.DirPath != wantDirPath {
		t.Errorf("DirPath = %q, want %q", colDef.DirPath, wantDirPath)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – validation succeeds  (line 220-225 happy path)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_ValidationValid(t *testing.T) {
	t.Parallel()

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte(colDefContent), nil },
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
		},
	}

	colDef, err := dl.readCollectionDefShared("/schema/mycol", "/data", "parent", "mycol", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "mycol" {
		t.Errorf("ID = %q, want 'mycol'", colDef.ID)
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – validation fails  (line 221-223)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_ValidationError(t *testing.T) {
	t.Parallel()

	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte("columns: {}\n"), nil },
		readDir:  os.ReadDir,
	}

	_, err := dl.readCollectionDefShared("/schema/mycol", "/data", "parent", "mycol", ingitdb.NewReadOptions(ingitdb.Validate()))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not valid definition of collection 'parent/mycol'") {
		t.Errorf("got error %q, want substring about invalid collection", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – loadSubCollectionsShared fails  (line 228-230)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_SubCollectionsError(t *testing.T) {
	t.Parallel()

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	sentinel := errors.New("subcol boom")
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte(colDefContent), nil },
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, sentinel
		},
	}

	_, err := dl.readCollectionDefShared("/schema/mycol", "/data", "", "mycol", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load subcollections for") {
		t.Errorf("got error %q, want substring 'failed to load subcollections for'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – loadViews fails  (line 234-236)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_LoadViewsError(t *testing.T) {
	t.Parallel()

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	sentinel := errors.New("views boom")
	callCount := 0
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte(colDefContent), nil },
		readDir: func(path string) ([]os.DirEntry, error) {
			callCount++
			if callCount == 1 {
				// loadSubCollectionsShared → no subcols
				return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
			}
			// loadViews → error
			return nil, sentinel
		},
	}

	_, err := dl.readCollectionDefShared("/schema/mycol", "/data", "", "mycol", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load views for") {
		t.Errorf("got error %q, want substring 'failed to load views for'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readCollectionDefShared – DefaultView injection  (line 238-244)
// ---------------------------------------------------------------------------

func TestReadCollectionDefShared_DefaultViewInjection(t *testing.T) {
	t.Parallel()

	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
default_view:
  format: csv
  order_by: id
`
	dl := defLoader{
		readFile: func(string) ([]byte, error) { return []byte(colDefContent), nil },
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
		},
	}

	colDef, err := dl.readCollectionDefShared("/schema/mycol", "/data", "", "mycol", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.DefaultView == nil {
		t.Fatal("expected DefaultView to be non-nil")
	}
	if colDef.DefaultView.ID != ingitdb.DefaultViewID {
		t.Errorf("DefaultView.ID = %q, want %q", colDef.DefaultView.ID, ingitdb.DefaultViewID)
	}
	if !colDef.DefaultView.IsDefault {
		t.Error("expected DefaultView.IsDefault to be true")
	}
	if colDef.Views == nil {
		t.Fatal("expected Views to be non-nil")
	}
	if colDef.Views[ingitdb.DefaultViewID] != colDef.DefaultView {
		t.Error("expected Views to contain the default view")
	}
}

// ---------------------------------------------------------------------------
// loadSubCollectionsShared – readDir returns os.ErrNotExist  (line 255-257)
// ---------------------------------------------------------------------------

func TestLoadSubCollectionsShared_NotExist(t *testing.T) {
	t.Parallel()

	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
		},
	}

	result, err := dl.loadSubCollectionsShared("/schema", "/data", "parent", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// loadSubCollectionsShared – readDir returns non-NotExist error  (line 258-260)
// ---------------------------------------------------------------------------

func TestLoadSubCollectionsShared_ReadDirError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("readdir boom")
	dl := defLoader{
		readFile: os.ReadFile,
		readDir: func(string) ([]os.DirEntry, error) {
			return nil, sentinel
		},
	}

	_, err := dl.loadSubCollectionsShared("/schema", "/data", "parent", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read schema directory") {
		t.Errorf("got error %q, want substring 'failed to read schema directory'", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error chain does not wrap sentinel: %v", err)
	}
}

// ---------------------------------------------------------------------------
// loadSubCollectionsShared – readCollectionDefShared returns non-ErrNotExist
// error  (line 275)
// ---------------------------------------------------------------------------

func TestLoadSubCollectionsShared_SubCollectionError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Create a subdirectory (not $-prefixed) with invalid YAML definition
	subDir := filepath.Join(root, "badcol")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, ingitdb.CollectionDefFileName), []byte("a: [1,2\n"), 0o644); err != nil {
		t.Fatalf("setup: write definition: %v", err)
	}

	dl := newDefLoader()
	_, err := dl.loadSubCollectionsShared(root, "/data", "parent", ingitdb.NewReadOptions())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse YAML file") {
		t.Errorf("got error %q, want substring 'failed to parse YAML file'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// loadSubCollectionsShared – readCollectionDefShared returns ErrNotExist
// (continue/skip)  (line 272-273)
// ---------------------------------------------------------------------------

func TestLoadSubCollectionsShared_SkipsNonCollectionDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Create a subdirectory without definition.yaml → ErrNotExist → skip
	subDir := filepath.Join(root, "notacol")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	dl := newDefLoader()
	result, err := dl.loadSubCollectionsShared(root, "/data", "parent", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (dir skipped), got %v", result)
	}
}

// ---------------------------------------------------------------------------
// loadSubCollections – successfully loads a subcollection  (line 317-320)
// ---------------------------------------------------------------------------

func TestLoadSubCollections_SuccessfulLoad(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	colDir := filepath.Join(root, "col")
	subColsDir := filepath.Join(colDir, ingitdb.SchemaDir, "subcollections", "items")
	if err := os.MkdirAll(subColsDir, 0o755); err != nil {
		t.Fatalf("setup: create subcollection dir: %v", err)
	}
	colDefContent := `columns:
  id:
    type: string
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
`
	if err := os.WriteFile(filepath.Join(subColsDir, ingitdb.CollectionDefFileName), []byte(colDefContent), 0o644); err != nil {
		t.Fatalf("setup: write subcollection definition: %v", err)
	}

	result, err := newDefLoader().loadSubCollections(root, "col", nil, "parent", ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 subcollection, got %d", len(result))
	}
	sub, ok := result["items"]
	if !ok {
		t.Fatal("expected 'items' subcollection")
	}
	if sub.ID != "items" {
		t.Errorf("subcollection ID = %q, want 'items'", sub.ID)
	}
}
