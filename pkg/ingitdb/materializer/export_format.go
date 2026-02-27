package materializer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// defaultViewFormatExtension returns the file extension for a given format string.
// Empty string or unknown -> "ingr", "tsv" -> "tsv", "ingr" -> "ingr", "csv" -> "csv", etc.
func defaultViewFormatExtension(format string) string {
	switch strings.ToLower(format) {
	case "tsv":
		return "tsv"
	case "ingr":
		return "ingr"
	case "csv":
		return "csv"
	case "json":
		return "json"
	case "jsonl":
		return "jsonl"
	case "yaml":
		return "yaml"
	default: // "", unknown
		return "ingr"
	}
}

// formatBatchFileName returns the output file name for a batch.
// If totalBatches <= 1, returns base+"."+ext.
// Otherwise returns base-NNNNNN.ext (zero-padded 6-digit batch number).
func formatBatchFileName(base, ext string, batchNum, totalBatches int) string {
	if totalBatches <= 1 {
		return base + "." + ext
	}
	return fmt.Sprintf("%s-%06d.%s", base, batchNum, ext)
}

// formatExportBatch serializes a batch of records into the given format.
// format must already be lowercased (or empty for ingr default).
// viewName is used only by INGR to generate the metadata header line.
func formatExportBatch(format string, viewName string, headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	switch format {
	case "tsv":
		return formatTSV(headers, records)
	case "csv":
		return formatCSV(headers, records)
	case "json":
		return formatJSON(headers, records)
	case "jsonl":
		return formatJSONL(headers, records)
	case "yaml":
		return formatYAML(headers, records)
	default: // "", "ingr"
		return formatINGR(viewName, headers, records)
	}
}

func formatTSV(headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(strings.Join(headers, "\t"))
	buf.WriteByte('\n')
	for _, rec := range records {
		for i, h := range headers {
			if i > 0 {
				buf.WriteByte('\t')
			}
			val := ""
			if rec.Data != nil {
				if v, ok := rec.Data[h]; ok && v != nil {
					val = fmt.Sprint(v)
				}
			}
			buf.WriteString(escapeTSV(val))
		}
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// formatINGR serializes records in INGR format.
// The first line is a metadata header: "# {viewName}: $ID, col2, col3, ..."
// where "id" is represented as "$ID". Subsequent lines are N lines per record
// (one JSON-encoded field value per line). N equals len(headers).
func formatINGR(viewName string, headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	// Write metadata header line
	buf.WriteString("# ")
	buf.WriteString(viewName)
	buf.WriteString(": ")
	for i, h := range headers {
		if i > 0 {
			buf.WriteString(", ")
		}
		if h == "id" {
			buf.WriteString("$ID")
		} else {
			buf.WriteString(h)
		}
	}
	buf.WriteByte('\n')
	for _, rec := range records {
		for _, h := range headers {
			var val any
			if rec.Data != nil {
				val = rec.Data[h]
			}
			b, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("ingr: failed to marshal field %q: %w", h, err)
			}
			buf.Write(b)
			buf.WriteByte('\n')
		}
	}
	// Write footer line without trailing newline
	n := len(records)
	if n == 1 {
		buf.WriteString("# 1 record")
	} else {
		fmt.Fprintf(&buf, "# %d records", n)
	}
	return buf.Bytes(), nil
}

func escapeTSV(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

func formatCSV(headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, rec := range records {
		row := make([]string, len(headers))
		for i, h := range headers {
			if rec.Data != nil {
				if v, ok := rec.Data[h]; ok && v != nil {
					row[i] = fmt.Sprint(v)
				}
			}
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatJSON(headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	rows := recordsToMaps(headers, records)
	return json.Marshal(rows)
}

func formatJSONL(headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	rows := recordsToMaps(headers, records)
	var buf bytes.Buffer
	for _, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func formatYAML(headers []string, records []ingitdb.RecordEntry) ([]byte, error) {
	rows := recordsToMaps(headers, records)
	return yaml.Marshal(rows)
}

func recordsToMaps(headers []string, records []ingitdb.RecordEntry) []map[string]any {
	rows := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		row := make(map[string]any, len(headers))
		for _, h := range headers {
			if rec.Data != nil {
				row[h] = rec.Data[h]
			} else {
				row[h] = nil
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// determineColumns returns the ordered list of column names to export.
// Priority:
//  1. view.Columns if non-empty (used as-is, in the order specified)
//  2. col.ColumnsOrder if non-empty
//  3. keys of col.Columns sorted alphabetically
//
// "id" is always prepended if it is not already at index 0.
func determineColumns(col *ingitdb.CollectionDef, view *ingitdb.ViewDef) []string {
	var cols []string
	if len(view.Columns) > 0 {
		cols = make([]string, len(view.Columns))
		copy(cols, view.Columns)
	} else if len(col.ColumnsOrder) > 0 {
		cols = make([]string, len(col.ColumnsOrder))
		copy(cols, col.ColumnsOrder)
	} else {
		cols = make([]string, 0, len(col.Columns))
		for k := range col.Columns {
			cols = append(cols, k)
		}
		sort.Strings(cols)
	}

	// Ensure "id" is at index 0
	if len(cols) == 0 || cols[0] != "id" {
		// Remove "id" from wherever it is (if present)
		filtered := cols[:0]
		for _, c := range cols {
			if c != "id" {
				filtered = append(filtered, c)
			}
		}
		cols = append([]string{"id"}, filtered...)
	}
	return cols
}
