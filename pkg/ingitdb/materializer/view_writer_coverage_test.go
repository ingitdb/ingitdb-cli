package materializer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestFileViewWriter_WriteView_TemplateParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template: ".ingitdb-view.bad.md",
		FileName: "output.md",
	}

	// Create template file with invalid template syntax
	templatePath := filepath.Join(dir, ".ingitdb-view.bad.md")
	if err := os.WriteFile(templatePath, []byte("{{ .unclosed"), 0o644); err != nil {
		t.Fatalf("write bad template: %v", err)
	}

	writer := NewFileViewWriter()
	_, err := writer.WriteView(context.Background(), col, view, nil, filepath.Join(dir, "output.md"))
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestFileViewWriter_WriteView_TemplateNotFound(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/tmp"}
	view := &ingitdb.ViewDef{
		Template: "nonexistent.md",
		FileName: "output.md",
	}

	writer := NewFileViewWriter()
	_, err := writer.WriteView(context.Background(), col, view, nil, "/tmp/output.md")
	if err == nil {
		t.Fatal("expected error for missing template file")
	}
}

func TestFileViewWriter_WriteView_TemplateExecuteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.exec-fail.md",
		FileName:       "output.md",
		RecordsVarName: "items",
	}

	// Create template that references non-existent field
	templatePath := filepath.Join(dir, ".ingitdb-view.exec-fail.md")
	if err := os.WriteFile(templatePath, []byte("{{ .items.InvalidMethod }}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	records := []ingitdb.IRecordEntry{ingitdb.NewMapRecordEntry("", map[string]any{"title": "Test"})}
	_, err := writer.WriteView(context.Background(), col, view, records, filepath.Join(dir, "output.md"))
	if err == nil {
		t.Fatal("expected error for template execution failure")
	}
}

func TestFileViewWriter_WriteView_MkdirError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template: ".ingitdb-view.test.md",
		FileName: "output.md",
	}

	templatePath := filepath.Join(dir, ".ingitdb-view.test.md")
	if err := os.WriteFile(templatePath, []byte("Test"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	mkdirErr := errors.New("mkdir failed")
	writer := FileViewWriter{
		readFile: os.ReadFile,
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			return nil
		},
		mkdirAll: func(path string, perm os.FileMode) error {
			return mkdirErr
		},
	}

	outPath := filepath.Join(dir, "subdir", "output.md")
	_, err := writer.WriteView(context.Background(), col, view, nil, outPath)
	if err == nil {
		t.Fatal("expected error for mkdir failure")
	}
	if !errors.Is(err, mkdirErr) {
		t.Errorf("expected error to wrap mkdir error, got: %v", err)
	}
}

func TestFileViewWriter_WriteView_WriteFileError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template: ".ingitdb-view.test.md",
		FileName: "output.md",
	}

	templatePath := filepath.Join(dir, ".ingitdb-view.test.md")
	if err := os.WriteFile(templatePath, []byte("Test"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writeErr := errors.New("write failed")
	writer := FileViewWriter{
		readFile: func(path string) ([]byte, error) {
			if strings.HasSuffix(path, ".md") && !strings.Contains(path, ".ingitdb-view") {
				return nil, os.ErrNotExist
			}
			return os.ReadFile(path)
		},
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			return writeErr
		},
		mkdirAll: os.MkdirAll,
	}

	outPath := filepath.Join(dir, "output.md")
	_, err := writer.WriteView(context.Background(), col, view, nil, outPath)
	if err == nil {
		t.Fatal("expected error for write failure")
	}
	if !errors.Is(err, writeErr) {
		t.Errorf("expected error to wrap write error, got: %v", err)
	}
}

func TestFileViewWriter_WriteView_NonMarkdownFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.test.txt",
		FileName:       "output.txt",
		RecordsVarName: "items",
	}

	templatePath := filepath.Join(dir, ".ingitdb-view.test.txt")
	templateContent := "[//]: # (comment)\nContent"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	outPath := filepath.Join(dir, "output.txt")
	_, err := writer.WriteView(context.Background(), col, view, nil, outPath)
	if err != nil {
		t.Fatalf("WriteView: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	// For non-markdown files, comments should NOT be stripped
	if !strings.Contains(string(content), "[//]:") {
		t.Error("expected comments to be preserved in non-markdown file")
	}
}

func TestViewTemplateData_DefaultRecordsVarName(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{RecordsVarName: ""}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("", map[string]any{"title": "A"}),
	}

	data := viewTemplateData(view, records)
	if _, ok := data["records"]; !ok {
		t.Error("expected default var name 'records'")
	}
}

func TestViewTemplateData_CustomRecordsVarName(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{RecordsVarName: "items"}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("", map[string]any{"title": "A"}),
	}

	data := viewTemplateData(view, records)
	if _, ok := data["items"]; !ok {
		t.Error("expected custom var name 'items'")
	}
	if _, ok := data["records"]; ok {
		t.Error("expected 'records' to not exist when custom name is set")
	}
}

func TestViewTemplateData_NilRecordData(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{RecordsVarName: "items"}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("", nil),
		ingitdb.NewMapRecordEntry("", map[string]any{"title": "A"}),
	}

	data := viewTemplateData(view, records)
	items, ok := data["items"].([]map[string]any)
	if !ok {
		t.Fatal("expected items to be []map[string]any")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != nil {
		t.Error("expected first item to be nil")
	}
	if items[1] == nil {
		t.Error("expected second item to not be nil")
	}
}

func TestStripMarkdownComments_MultipleComments(t *testing.T) {
	t.Parallel()

	input := []byte("# Title\n[//]: # (comment 1)\n\nContent\n\n[//]: # (comment 2)\n\nMore content\n")
	output := stripMarkdownComments(input)

	outputStr := string(output)
	if strings.Contains(outputStr, "[//]:") {
		t.Error("expected all comments to be stripped")
	}
	if strings.Contains(outputStr, "comment 1") || strings.Contains(outputStr, "comment 2") {
		t.Error("expected comment text to be removed")
	}
}

func TestStripMarkdownComments_NoComments(t *testing.T) {
	t.Parallel()

	input := []byte("# Title\n\nContent\n")
	output := stripMarkdownComments(input)

	if !bytes.Equal(input, output) {
		t.Error("expected output to match input when no comments present")
	}
}

func TestStripMarkdownComments_CommentAtStart(t *testing.T) {
	t.Parallel()

	input := []byte("[//]: # (comment)\nContent\n")
	output := stripMarkdownComments(input)

	outputStr := string(output)
	if strings.Contains(outputStr, "[//]:") {
		t.Error("expected comment to be stripped")
	}
	if !strings.Contains(outputStr, "Content") {
		t.Error("expected content to be preserved")
	}
}

func TestStripMarkdownComments_CommentAtEnd(t *testing.T) {
	t.Parallel()

	input := []byte("Content\n[//]: # (comment)\n")
	output := stripMarkdownComments(input)

	outputStr := string(output)
	if strings.Contains(outputStr, "[//]:") {
		t.Error("expected comment to be stripped")
	}
	if !strings.Contains(outputStr, "Content") {
		t.Error("expected content to be preserved")
	}
}

func TestStripMarkdownComments_PreservesTrailingNewline(t *testing.T) {
	t.Parallel()

	input := []byte("Content\n")
	output := stripMarkdownComments(input)

	if !bytes.HasSuffix(output, []byte("\n")) {
		t.Error("expected trailing newline to be preserved")
	}
}

func TestStripMarkdownComments_AddsTrailingNewlineWhenNeeded(t *testing.T) {
	t.Parallel()

	input := []byte("Content\n[//]: # (comment)\n")
	output := stripMarkdownComments(input)

	// Input has trailing newline, so output should too
	outputStr := string(output)
	if !strings.HasSuffix(outputStr, "\n") {
		t.Error("expected trailing newline to be preserved")
	}
}

func TestStripMarkdownComments_NoTrailingNewline(t *testing.T) {
	t.Parallel()

	input := []byte("Content")
	output := stripMarkdownComments(input)

	if bytes.HasSuffix(output, []byte("\n")) {
		t.Error("expected no trailing newline when input has none")
	}
}

func TestStripMarkdownComments_WindowsLineEndings(t *testing.T) {
	t.Parallel()

	input := []byte("Content\r\n[//]: # (comment)\r\n")
	output := stripMarkdownComments(input)

	outputStr := string(output)
	if strings.Contains(outputStr, "[//]:") {
		t.Error("expected comment to be stripped with Windows line endings")
	}
}

func TestIsMarkdownCommentLine_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want bool
	}{
		{"standard comment", "[//]: # (comment)", true},
		{"comment with spaces", "  [//]: # (comment)  ", true},
		{"comment with tabs", "\t[//]: # (comment)", true},
		{"comment windows", "[//]: # (comment)\r", true},
		{"not a comment - no hash", "[//]: (comment)", false},
		{"not a comment - missing prefix", "# (comment)", false},
		{"empty line", "", false},
		{"just whitespace", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMarkdownCommentLine(tt.line)
			if got != tt.want {
				t.Errorf("isMarkdownCommentLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsEmptyLine_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want bool
	}{
		{"empty string", "", true},
		{"spaces only", "   ", true},
		{"tabs only", "\t\t", true},
		{"windows newline", "\r", true},
		{"mixed whitespace", " \t \t ", true},
		{"has content", "  text  ", false},
		{"single char", "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEmptyLine(tt.line)
			if got != tt.want {
				t.Errorf("isEmptyLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FuncViewWriter tests
// ---------------------------------------------------------------------------

func TestNewFuncViewWriter(t *testing.T) {
	t.Parallel()

	called := false
	w := NewFuncViewWriter(func(content []byte) error {
		called = true
		return nil
	})
	// write function should be stored; invoke via WriteView to confirm it is wired
	_ = w
	_ = called
}

func TestFuncViewWriter_WriteView_EmptyTemplate(t *testing.T) {
	t.Parallel()

	w := NewFuncViewWriter(func(content []byte) error { return nil })
	col := &ingitdb.CollectionDef{DirPath: "/tmp"}
	view := &ingitdb.ViewDef{Template: ""}
	_, err := w.WriteView(context.Background(), col, view, nil, "/tmp/out.md")
	if err == nil {
		t.Fatal("expected error for empty template, got nil")
	}
}

func TestFuncViewWriter_WriteView_TemplateParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{Template: "bad-tmpl.md"}

	tmplPath := filepath.Join(dir, "bad-tmpl.md")
	if err := os.WriteFile(tmplPath, []byte("{{ .unclosed"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w := NewFuncViewWriter(func(content []byte) error { return nil })
	_, err := w.WriteView(context.Background(), col, view, nil, filepath.Join(dir, "out.md"))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestFuncViewWriter_WriteView_TemplateExecuteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{Template: "exec-fail.md"}

	tmplPath := filepath.Join(dir, "exec-fail.md")
	// Reference an undefined sub-template: valid syntax but fails at Execute time.
	if err := os.WriteFile(tmplPath, []byte(`{{template "nonexistent-sub" .}}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	w := NewFuncViewWriter(func(content []byte) error { return nil })
	_, err := w.WriteView(context.Background(), col, view, nil, filepath.Join(dir, "out.md"))
	if err == nil {
		t.Fatal("expected execute error, got nil")
	}
}

func TestFuncViewWriter_WriteView_WriteFuncError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{Template: "valid.txt", RecordsVarName: "items"}

	tmplPath := filepath.Join(dir, "valid.txt")
	if err := os.WriteFile(tmplPath, []byte("Hello"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writeErr := errors.New("write failed")
	w := NewFuncViewWriter(func(content []byte) error { return writeErr })
	_, err := w.WriteView(context.Background(), col, view, nil, filepath.Join(dir, "out.txt"))
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	if !errors.Is(err, writeErr) {
		t.Errorf("expected writeErr, got: %v", err)
	}
}

func TestFuncViewWriter_WriteView_ValidTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{Template: "greeting.txt", RecordsVarName: "items"}

	tmplPath := filepath.Join(dir, "greeting.txt")
	if err := os.WriteFile(tmplPath, []byte("{{range .items}}{{.name}} {{end}}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var captured []byte
	w := NewFuncViewWriter(func(content []byte) error {
		captured = content
		return nil
	})

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"name": "World"}),
	}
	outPath := filepath.Join(dir, "out.txt")
	outcome, err := w.WriteView(context.Background(), col, view, records, outPath)
	if err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	if outcome != WriteOutcomeCreated {
		t.Errorf("expected WriteOutcomeCreated, got %v", outcome)
	}
	if !strings.Contains(string(captured), "World") {
		t.Errorf("expected 'World' in output, got: %s", string(captured))
	}
}

func TestFuncViewWriter_WriteView_MarkdownOutPath_StripsComments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{Template: "tmpl.md", RecordsVarName: "items"}

	tmplPath := filepath.Join(dir, "tmpl.md")
	// Template containing a markdown comment — should be stripped for .md output.
	if err := os.WriteFile(tmplPath, []byte("# Title\n[//]: # (hidden comment)\nContent\n"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	var captured []byte
	w := NewFuncViewWriter(func(content []byte) error {
		captured = content
		return nil
	})

	outPath := filepath.Join(dir, "output.md") // .md suffix triggers stripMarkdownComments
	outcome, err := w.WriteView(context.Background(), col, view, nil, outPath)
	if err != nil {
		t.Fatalf("WriteView: %v", err)
	}
	if outcome != WriteOutcomeCreated {
		t.Errorf("expected WriteOutcomeCreated, got %v", outcome)
	}
	if strings.Contains(string(captured), "[//]:") {
		t.Error("expected markdown comments to be stripped from .md output")
	}
	if !strings.Contains(string(captured), "Content") {
		t.Error("expected non-comment content to be preserved")
	}
}

// ---------------------------------------------------------------------------
// FileViewWriter — WriteOutcomeUpdated path
// ---------------------------------------------------------------------------

func TestFileViewWriter_WriteView_Updated(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	col := &ingitdb.CollectionDef{DirPath: dir}
	view := &ingitdb.ViewDef{
		Template:       ".ingitdb-view.list.txt",
		FileName:       "list.txt",
		RecordsVarName: "items",
	}

	tmplPath := filepath.Join(dir, ".ingitdb-view.list.txt")
	if err := os.WriteFile(tmplPath, []byte("{{range .items}}{{.name}}\n{{end}}"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	writer := NewFileViewWriter()
	outPath := filepath.Join(dir, "list.txt")

	// First write: file does not exist yet → WriteOutcomeCreated.
	records1 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("a", map[string]any{"name": "Alpha"}),
	}
	outcome1, err := writer.WriteView(context.Background(), col, view, records1, outPath)
	if err != nil {
		t.Fatalf("first WriteView: %v", err)
	}
	if outcome1 != WriteOutcomeCreated {
		t.Errorf("first write: expected WriteOutcomeCreated, got %v", outcome1)
	}

	// Second write with different records: file exists with different content → WriteOutcomeUpdated.
	records2 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("b", map[string]any{"name": "Beta"}),
	}
	outcome2, err := writer.WriteView(context.Background(), col, view, records2, outPath)
	if err != nil {
		t.Fatalf("second WriteView: %v", err)
	}
	if outcome2 != WriteOutcomeUpdated {
		t.Errorf("second write: expected WriteOutcomeUpdated, got %v", outcome2)
	}
}

// ---------------------------------------------------------------------------
// stripMarkdownComments — trailing-newline preservation when only comments remain
// ---------------------------------------------------------------------------

func TestStripMarkdownComments_OnlyCommentWithTrailingNewline(t *testing.T) {
	t.Parallel()

	// Input is only a comment line followed by newline.
	// strings.Split → ["[//]: # (comment)", ""]
	// After filtering: comment is removed, "" remains → filtered = [""]
	// strings.Join([""], "\n") = "" (no trailing newline)
	// Since input ends with "\n" but result doesn't, result += "\n" fires.
	input := []byte("[//]: # (comment)\n")
	output := stripMarkdownComments(input)
	if !bytes.HasSuffix(output, []byte("\n")) {
		t.Error("expected trailing newline to be preserved when input ends with newline and only content was a comment")
	}
}
