package commands

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// isFdTTY reports whether f's file descriptor is a terminal.
func isFdTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runWithEditor writes a schema-derived template to a temp file, opens it in
// the configured editor, and returns the parsed record data. Returns
// (nil, true, nil) when the file was not modified (no-op edit).
func runWithEditor(colDef *ingitdb.CollectionDef, openEditor func(string) error) (map[string]any, bool, error) {
	if colDef.RecordFile == nil {
		return nil, false, fmt.Errorf("collection %q has no record_file definition", colDef.ID)
	}

	template := buildRecordTemplate(colDef)

	ext := recordFormatExt(colDef.RecordFile.Format)
	tmpFile, err := os.CreateTemp("", "ingitdb-*."+ext)
	if err != nil {
		return nil, false, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err = tmpFile.Write(template); err != nil {
		_ = tmpFile.Close()
		return nil, false, fmt.Errorf("write template: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return nil, false, fmt.Errorf("close temp file: %w", err)
	}

	if err = openEditor(tmpPath); err != nil {
		return nil, false, fmt.Errorf("editor: %w", err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, false, fmt.Errorf("read edited file: %w", err)
	}

	if bytes.Equal(content, template) {
		return nil, true, nil
	}

	data, err := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	return data, false, err
}

// buildRecordTemplate returns a byte slice pre-filled with an empty record
// template for the given collection. For markdown the template has YAML
// frontmatter delimiters; for other formats it is a bare YAML skeleton.
func buildRecordTemplate(colDef *ingitdb.CollectionDef) []byte {
	keys := orderedColumnKeys(colDef)
	var buf bytes.Buffer
	if colDef.RecordFile != nil && colDef.RecordFile.Format == ingitdb.RecordFormatMarkdown {
		buf.WriteString("---\n")
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
		buf.WriteString("---\n\n")
	} else {
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
	}
	return buf.Bytes()
}

// orderedColumnKeys returns the column names in canonical order:
// ColumnsOrder entries first (skipping absent columns), then remaining
// columns alphabetically.
func orderedColumnKeys(colDef *ingitdb.CollectionDef) []string {
	seen := make(map[string]bool)
	var ordered []string
	for _, k := range colDef.ColumnsOrder {
		if _, ok := colDef.Columns[k]; !ok || seen[k] {
			continue
		}
		seen[k] = true
		ordered = append(ordered, k)
	}
	var rest []string
	for k := range colDef.Columns {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// recordFormatExt returns a file extension for the given record format,
// used when naming the editor temp file.
func recordFormatExt(format ingitdb.RecordFormat) string {
	switch format {
	case ingitdb.RecordFormatMarkdown:
		return "md"
	case ingitdb.RecordFormatJSON:
		return "json"
	case ingitdb.RecordFormatTOML:
		return "toml"
	default:
		return "yaml"
	}
}

// parseEditorCommand splits the EDITOR env value into a program name and
// flag arguments. Empty input defaults to "vi". Values like "code --wait"
// or "emacs -nw" tokenize correctly so editor flags work without invoking
// a shell.
func parseEditorCommand(editor string) (string, []string) {
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	return parts[0], parts[1:]
}

// defaultOpenEditor opens tmpPath in $EDITOR (falling back to vi).
// Uses exec.Command (never a shell) to avoid injection.
func defaultOpenEditor(tmpPath string) error {
	prog, flags := parseEditorCommand(os.Getenv("EDITOR"))
	c := exec.Command(prog, append(flags, tmpPath)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
