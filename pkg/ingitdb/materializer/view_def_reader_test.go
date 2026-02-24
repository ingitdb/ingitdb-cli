package materializer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileViewDefReader_ReadViewDefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	viewDir := filepath.Join(dir, ".collection", "views")
	if err := os.MkdirAll(viewDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	viewPath := filepath.Join(viewDir, "README.yaml")
	content := []byte("order_by: title\ntemplate: .ingitdb-view.README.md\nfile_name: README.md\nrecords_var_name: tags\n")
	if err := os.WriteFile(viewPath, content, 0o644); err != nil {
		t.Fatalf("write view def: %v", err)
	}

	otherPath := filepath.Join(viewDir, "secondary.yaml")
	if err := os.WriteFile(otherPath, []byte("order_by: title\n"), 0o644); err != nil {
		t.Fatalf("write secondary view def: %v", err)
	}

	reader := FileViewDefReader{}
	defs, err := reader.ReadViewDefs(dir)
	if err != nil {
		t.Fatalf("ReadViewDefs: %v", err)
	}
	if len(defs) != 2 {
		t.Fatalf("expected 2 view defs, got %d", len(defs))
	}

	readme := defs["README"]
	if readme == nil {
		t.Fatalf("README view def not found")
	}
	if readme.ID != "README" {
		t.Fatalf("expected ID README, got %q", readme.ID)
	}
	if readme.Template != ".ingitdb-view.README.md" {
		t.Fatalf("expected template .ingitdb-view.README.md, got %q", readme.Template)
	}
	if readme.FileName != "README.md" {
		t.Fatalf("expected file name README.md, got %q", readme.FileName)
	}
	if readme.RecordsVarName != "tags" {
		t.Fatalf("expected records var name tags, got %q", readme.RecordsVarName)
	}
}

func TestViewNameFromPath_Invalid(t *testing.T) {
	t.Parallel()

	if _, err := viewNameFromPath("README.yml"); err == nil {
		t.Fatalf("expected error for missing suffix")
	}
	if _, err := viewNameFromPath(".yaml"); err == nil {
		t.Fatalf("expected error for empty view name")
	}
}
