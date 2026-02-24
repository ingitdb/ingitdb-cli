package materializer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileViewDefReader_ReadViewDefs_GlobError(t *testing.T) {
	t.Parallel()

	// Use a path that will cause glob to fail on most systems
	// Glob typically fails with ErrBadPattern or when path is invalid
	reader := FileViewDefReader{}
	_, err := reader.ReadViewDefs("/tmp/[invalid")
	if err == nil {
		t.Fatal("expected error for invalid glob pattern")
	}
}

func TestFileViewDefReader_ReadViewDefs_ReadFileError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	viewPath := filepath.Join(viewDir, "test.yaml")
	// Create a directory instead of a file to cause read error
	if err := os.Mkdir(viewPath, 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}

	reader := FileViewDefReader{}
	_, err := reader.ReadViewDefs(dir)
	if err == nil {
		t.Fatal("expected error when reading directory as file")
	}
}

func TestFileViewDefReader_ReadViewDefs_ParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	viewPath := filepath.Join(viewDir, "test.yaml")
	// Write invalid YAML
	content := []byte("invalid: yaml: content:\n  - unclosed")
	if err := os.WriteFile(viewPath, content, 0o644); err != nil {
		t.Fatalf("write invalid yaml: %v", err)
	}

	reader := FileViewDefReader{}
	_, err := reader.ReadViewDefs(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestFileViewDefReader_ReadViewDefs_InvalidFileName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create a file that matches the pattern but has invalid name format
	viewPath := filepath.Join(viewDir, ".yaml")
	content := []byte("order_by: title\n")
	if err := os.WriteFile(viewPath, content, 0o644); err != nil {
		t.Fatalf("write view def: %v", err)
	}

	reader := FileViewDefReader{}
	_, err := reader.ReadViewDefs(dir)
	if err == nil {
		t.Fatal("expected error for empty view name")
	}
}

func TestFileViewDefReader_ReadViewDefs_NoFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	reader := FileViewDefReader{}
	defs, err := reader.ReadViewDefs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected empty map, got %d definitions", len(defs))
	}
}

func TestViewNameFromPath_ValidPath(t *testing.T) {
	t.Parallel()

	name, err := viewNameFromPath("/tmp/.collection/views/README.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "README" {
		t.Errorf("expected README, got %q", name)
	}
}

func TestViewNameFromPath_MissingPrefix(t *testing.T) {
	t.Parallel()

	_, err := viewNameFromPath("/tmp/view.README.yml")
	if err == nil {
		t.Fatal("expected error for missing suffix")
	}
}

func TestViewNameFromPath_MissingSuffix(t *testing.T) {
	t.Parallel()

	_, err := viewNameFromPath("/tmp/README.yml")
	if err == nil {
		t.Fatal("expected error for wrong suffix")
	}
}

func TestViewNameFromPath_EmptyName(t *testing.T) {
	t.Parallel()

	_, err := viewNameFromPath(".yaml")
	if err == nil {
		t.Fatal("expected error for empty view name")
	}
}
