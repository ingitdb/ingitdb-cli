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
// Callers must resolve empty format to "ingr" before calling.
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
// format must be one of: "ingr", "tsv", "csv", "json", "jsonl", "yaml".
// An empty or unrecognised format returns an error; callers must pass "ingr" explicitly.
// viewName is used only by INGR to generate the metadata header line.
// opts are applied only by the INGR formatter; all other formats ignore them.
func formatExportBatch(format string, viewName string, headers []string, records []ingitdb.IRecordEntry, opts ...ExportOption) ([]byte, error) {
	switch format {
	case "ingr":
		var cfg ExportOptions
		ApplyOptions(&cfg, opts...)
		return formatINGR(viewName, cfg, headers, records)
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
	default:
		return nil, fmt.Errorf("unknown export format %q", format)
	}
}

func formatTSV(headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(strings.Join(headers, "\t"))
	buf.WriteByte('\n')
	for _, rec := range records {
		for i, h := range headers {
			if i > 0 {
				buf.WriteByte('\t')
			}
			val := ""
			d := rec.GetData()
			if d != nil {
				if v, ok := d[h]; ok && v != nil {
					val = fmt.Sprint(v)
				}
			}
			buf.WriteString(escapeTSV(val))
		}
		buf.WriteByte('\n')
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

func formatCSV(headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, err // untestable: csv.Writer writing to bytes.Buffer never errors
	}
	for _, rec := range records {
		row := make([]string, len(headers))
		d := rec.GetData()
		for i, h := range headers {
			if d != nil {
				if v, ok := d[h]; ok && v != nil {
					row[i] = fmt.Sprint(v)
				}
			}
		}
		if err := w.Write(row); err != nil {
			return nil, err // untestable: csv.Writer writing to bytes.Buffer never errors
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err // untestable: csv.Writer writing to bytes.Buffer never errors
	}
	return buf.Bytes(), nil
}

func formatJSON(headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
	rows := recordsToMaps(headers, records)
	return json.Marshal(rows)
}

func formatJSONL(headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
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

func formatYAML(headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
	rows := recordsToMaps(headers, records)
	return yaml.Marshal(rows)
}

func recordsToMaps(headers []string, records []ingitdb.IRecordEntry) []map[string]any {
	rows := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		row := make(map[string]any, len(headers))
		d := rec.GetData()
		for _, h := range headers {
			if d != nil {
				row[h] = d[h]
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
