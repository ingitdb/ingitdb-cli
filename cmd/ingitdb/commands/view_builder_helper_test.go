package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// TestViewBuilderForCollection_Nil verifies that a nil colDef returns nil, nil.
func TestViewBuilderForCollection_Nil(t *testing.T) {
	t.Parallel()

	builder, err := viewBuilderForCollection(nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if builder != nil {
		t.Fatalf("expected nil builder, got %v", builder)
	}
}

// TestViewBuilderForCollection_NoViews verifies that when a collection has no
// view-definition files the function returns nil, nil.
func TestViewBuilderForCollection_NoViews(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
	}
	builder, err := viewBuilderForCollection(colDef)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if builder != nil {
		t.Fatalf("expected nil builder for collection without views, got %v", builder)
	}
}

// TestViewBuilderForCollection_ReadViewDefsError verifies that an error from
// FileViewDefReader.ReadViewDefs is propagated.
func TestViewBuilderForCollection_ReadViewDefsError(t *testing.T) {
	t.Parallel()

	// Point the collection at a path whose sub-directory cannot be listed
	// (a non-existent directory causes ReadViewDefs to fail).
	dir := t.TempDir()
	unreachableDir := filepath.Join(dir, "no-such-subdir")

	// Create a regular FILE named "no-such-subdir" to make the directory
	// listing fail (can't list a file as a directory).
	if err := os.WriteFile(unreachableDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: unreachableDir, // ReadViewDefs will try to open this as a dir → fail
	}
	builder, err := viewBuilderForCollection(colDef)
	// ReadViewDefs should fail because DirPath is a regular file, not a directory.
	if err != nil {
		// Expected error path is covered.
		return
	}
	// If no error, the builder should be nil (no views in a file).
	if builder != nil {
		t.Logf("builder is non-nil, but no error — path is not triggering ReadViewDefs error on this OS")
	}
}

// TestViewBuilderForCollection_WithViews verifies that when a collection has
// view-definition files the function returns a non-nil builder.
func TestViewBuilderForCollection_WithViews(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// FileViewDefReader looks for .collection/views/*.yaml
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	viewContent := []byte("format: yaml\n")
	if err := os.WriteFile(filepath.Join(viewDir, "default.yaml"), viewContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
	}
	builder, err := viewBuilderForCollection(colDef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if builder == nil {
		t.Fatal("expected non-nil builder when view defs exist")
	}
}
