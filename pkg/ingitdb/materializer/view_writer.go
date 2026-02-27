package materializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type FuncViewWriter struct {
	write func(content []byte) error
}

func NewFuncViewWriter(write func(content []byte) error) FuncViewWriter {
	return FuncViewWriter{write: write}
}

func (w FuncViewWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.RecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	if view.Template == "" {
		return WriteOutcomeUnchanged, fmt.Errorf("view template is required")
	}
	templatePath := filepath.Join(col.DirPath, view.Template)
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}
	data := viewTemplateData(view, records)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to render template %s: %w", templatePath, err)
	}
	content := buf.Bytes()
	if strings.HasSuffix(strings.ToLower(outPath), ".md") {
		content = stripMarkdownComments(content)
	}
	if err := w.write(content); err != nil {
		return WriteOutcomeUnchanged, err
	}
	return WriteOutcomeCreated, nil
}

// FileViewWriter renders a view template and writes it to a file.
type FileViewWriter struct {
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	mkdirAll  func(string, os.FileMode) error
}

func NewFileViewWriter() FileViewWriter {
	return FileViewWriter{
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
	}
}

func (w FileViewWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.RecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	if view.Template == "" {
		return WriteOutcomeUnchanged, fmt.Errorf("view template is required")
	}
	templatePath := filepath.Join(col.DirPath, view.Template)
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}
	data := viewTemplateData(view, records)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to render template %s: %w", templatePath, err)
	}
	content := buf.Bytes()
	if strings.HasSuffix(strings.ToLower(outPath), ".md") {
		content = stripMarkdownComments(content)
	}
	existing, readErr := w.readFile(outPath)
	if readErr == nil {
		if bytes.Equal(existing, content) {
			return WriteOutcomeUnchanged, nil
		}
	}
	if err := w.mkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to create directory for %s: %w", outPath, err)
	}
	if err := w.writeFile(outPath, content, 0o644); err != nil {
		return WriteOutcomeUnchanged, fmt.Errorf("failed to write view output %s: %w", outPath, err)
	}
	if readErr == nil {
		return WriteOutcomeUpdated, nil
	}
	return WriteOutcomeCreated, nil
}

func viewTemplateData(view *ingitdb.ViewDef, records []ingitdb.RecordEntry) map[string]any {
	varName := view.RecordsVarName
	if varName == "" {
		varName = "records"
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, record.Data)
	}
	return map[string]any{
		varName: items,
	}
}

func stripMarkdownComments(content []byte) []byte {
	text := string(content)
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if isMarkdownCommentLine(line) {
			if len(filtered) > 0 && isEmptyLine(filtered[len(filtered)-1]) {
				if i+1 < len(lines) && isEmptyLine(lines[i+1]) {
					filtered = filtered[:len(filtered)-1]
				}
			}
			continue
		}
		filtered = append(filtered, line)
	}
	result := strings.Join(filtered, "\n")
	if strings.HasSuffix(text, "\n") && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return []byte(result)
}

func isMarkdownCommentLine(line string) bool {
	trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if !strings.HasPrefix(trimmed, "[//]:") {
		return false
	}
	return strings.Contains(trimmed, "#")
}

func isEmptyLine(line string) bool {
	return strings.TrimSpace(strings.TrimSuffix(line, "\r")) == ""
}
