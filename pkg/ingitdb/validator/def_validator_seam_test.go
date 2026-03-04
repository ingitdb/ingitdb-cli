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
