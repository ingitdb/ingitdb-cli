package materializer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestFileViewWriter_RenderAndWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.README.md",
		FileName:       "README.md",
		RecordsVarName: "tags",
	}
	templatePath := filepath.Join(dir, ".ingitdb-view.README.md")
	templateContent := "| Title |\n| ----- |\n{{ range .tags }}| {{ .title }} |\n{{ end }}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	records := []ingitdb.RecordEntry{
		{Data: map[string]any{"title": "Home"}},
		{Data: map[string]any{"title": "Work"}},
	}
	outPath := filepath.Join(dir, "README.md")
	written, err := writer.WriteView(context.Background(), col, view, records, outPath)
	if err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	if written == WriteOutcomeUnchanged {
		t.Fatalf("expected file to be written")
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	expected := "| Title |\n| ----- |\n| Home |\n| Work |\n"
	if string(content) != expected {
		t.Fatalf("unexpected output:\n%s", string(content))
	}
}

func TestFileViewWriter_Unchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.README.md",
		FileName:       "README.md",
		RecordsVarName: "tags",
	}
	templatePath := filepath.Join(dir, ".ingitdb-view.README.md")
	templateContent := "{{- range .tags }}{{ .title }}\n{{- end }}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	records := []ingitdb.RecordEntry{{Data: map[string]any{"title": "Home"}}}
	outPath := filepath.Join(dir, "README.md")
	if _, err := writer.WriteView(context.Background(), col, view, records, outPath); err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	written, err := writer.WriteView(context.Background(), col, view, records, outPath)
	if err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	if written != WriteOutcomeUnchanged {
		t.Fatalf("expected unchanged output")
	}
}

func TestFileViewWriter_MissingTemplate(t *testing.T) {
	t.Parallel()

	writer := NewFileViewWriter()
	_, err := writer.WriteView(context.Background(), &ingitdb.CollectionDef{}, &ingitdb.ViewDef{}, nil, "README.md")
	if err == nil {
		t.Fatalf("expected error for missing template")
	}
}

func TestFileViewWriter_StripsMarkdownComments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.README.md",
		FileName:       "README.md",
		RecordsVarName: "tags",
	}
	templatePath := filepath.Join(dir, ".ingitdb-view.README.md")
	templateContent := "# Tags\n[//]: # (comment)\n{{ range .tags }}- {{ .title }}\n{{ end }}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	records := []ingitdb.RecordEntry{{Data: map[string]any{"title": "Home"}}}
	outPath := filepath.Join(dir, "README.md")
	if _, err := writer.WriteView(context.Background(), col, view, records, outPath); err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(content), "[//]:") {
		t.Fatalf("expected markdown comments to be stripped, got:\n%s", string(content))
	}
}

func TestFileViewWriter_StripsMarkdownComments_EmptyLineCollapse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.README.md",
		FileName:       "README.md",
		RecordsVarName: "tags",
	}
	templatePath := filepath.Join(dir, ".ingitdb-view.README.md")
	templateContent := "# Tags\n\n[//]: # (comment)\n\n{{ range .tags }}- {{ .title }}\n{{ end }}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	records := []ingitdb.RecordEntry{{Data: map[string]any{"title": "Home"}}}
	outPath := filepath.Join(dir, "README.md")
	if _, err := writer.WriteView(context.Background(), col, view, records, outPath); err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	output := string(content)
	if strings.Contains(output, "[//]:") {
		t.Fatalf("expected markdown comments to be stripped, got:\n%s", output)
	}
	if strings.Contains(output, "\n\n\n") {
		t.Fatalf("expected one blank line between sections, got:\n%s", output)
	}
}