package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRootCollections(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ingitdb"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlContent := "tags: .collections/tags\ntasks: .collections/tasks\n"
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := readRootCollections(dir)
	if err != nil {
		t.Fatalf("readRootCollections: %v", err)
	}
	if entries["tags"] != ".collections/tags" {
		t.Errorf("expected tags entry, got: %v", entries)
	}
	if entries["tasks"] != ".collections/tasks" {
		t.Errorf("expected tasks entry, got: %v", entries)
	}
}

func TestReadRootCollections_MissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := readRootCollections(dir)
	if err == nil {
		t.Fatal("expected error when root-collections.yaml missing")
	}
	if !strings.Contains(err.Error(), "root-collections.yaml") {
		t.Errorf("error should name the missing file, got: %v", err)
	}
}

func TestWriteRootCollections_RemovesEntry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ingitdb"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := "tags: .collections/tags\ntasks: .collections/tasks\nstatuses: .collections/statuses\n"
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"), []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := writeRootCollectionsWithout(dir, "tags"); err != nil {
		t.Fatalf("writeRootCollectionsWithout: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"))
	if err != nil {
		t.Fatalf("read after write: %v", err)
	}
	if strings.Contains(string(got), "tags:") {
		t.Errorf("expected tags entry removed, got:\n%s", got)
	}
	if !strings.Contains(string(got), "tasks: .collections/tasks") {
		t.Errorf("expected tasks entry preserved, got:\n%s", got)
	}
	if !strings.Contains(string(got), "statuses: .collections/statuses") {
		t.Errorf("expected statuses entry preserved, got:\n%s", got)
	}
}

func TestWriteRootCollections_NonExistentEntryNoError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ingitdb"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"), []byte("a: .collections/a\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Removing a non-existent entry MUST succeed silently (idempotent
	// from the helper's perspective; the caller enforces --if-exists
	// semantics).
	if err := writeRootCollectionsWithout(dir, "nonexistent"); err != nil {
		t.Errorf("expected silent success, got: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, ".ingitdb", "root-collections.yaml"))
	if !strings.Contains(string(got), "a: .collections/a") {
		t.Errorf("existing entry should be preserved, got:\n%s", got)
	}
}
