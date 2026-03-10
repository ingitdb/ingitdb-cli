package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// writeCSV writes records as a CSV table with a header row.
// columns defines the column order; if empty it is derived from the first record.
// Cell values are formatted by formatCSVCell: maps and slices are JSON-encoded so
// the output is machine-readable; scalars are rendered with fmt.Sprintf.
// Quoting and escaping is handled by encoding/csv.
func writeCSV(w io.Writer, records []map[string]any, columns []string) error {
	if len(columns) == 0 {
		columns = collectColumns(records)
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}
	for _, rec := range records {
		row := make([]string, len(columns))
		for i, col := range columns {
			if v, ok := rec[col]; ok {
				row[i] = formatCSVCell(v)
			}
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}
	cw.Flush()
	return cw.Error()
}

// formatCSVCell converts a field value to a CSV cell string.
// Maps and slices are JSON-encoded to produce a compact, machine-readable
// representation. All other types are formatted with fmt.Sprintf.
func formatCSVCell(v any) string {
	if v == nil {
		return ""
	}
	switch v.(type) {
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// writeJSON writes records as an indented JSON array.
func writeJSON(w io.Writer, records []map[string]any) error {
	out, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", out)
	return err
}

// writeYAML writes records as a YAML list.
func writeYAML(w io.Writer, records []map[string]any) error {
	out, err := yaml.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	_, err = w.Write(out)
	return err
}

// writeMarkdown writes records as a GitHub-flavoured Markdown table.
// columns defines the column order; if empty it is derived from the first record.
func writeMarkdown(w io.Writer, records []map[string]any, columns []string) error {
	if len(columns) == 0 {
		columns = collectColumns(records)
	}
	// Header row.
	_, err := fmt.Fprintf(w, "| %s |\n", strings.Join(columns, " | "))
	if err != nil {
		return err
	}
	// Separator row.
	separators := make([]string, len(columns))
	for i := range separators {
		separators[i] = "---"
	}
	_, err = fmt.Fprintf(w, "| %s |\n", strings.Join(separators, " | "))
	if err != nil {
		return err
	}
	// Data rows.
	for _, rec := range records {
		cells := make([]string, len(columns))
		for i, col := range columns {
			if v, ok := rec[col]; ok {
				cells[i] = fmt.Sprintf("%v", v)
			}
		}
		_, err = fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | "))
		if err != nil {
			return err
		}
	}
	return nil
}

// collectColumns returns a sorted, deduplicated list of keys found across all records.
// "$id" is placed first when present.
func collectColumns(records []map[string]any) []string {
	seen := make(map[string]bool)
	var cols []string
	// Always put $id first if present in any record.
	for _, rec := range records {
		if _, ok := rec["$id"]; ok {
			if !seen["$id"] {
				cols = append(cols, "$id")
				seen["$id"] = true
			}
			break
		}
	}
	for _, rec := range records {
		for k := range rec {
			if k == "$id" {
				continue
			}
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols
}
