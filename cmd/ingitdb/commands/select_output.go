package commands

// specscore: feature/cli/select

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// writeSingleRecord writes one record's projected fields as a bare
// mapping (yaml) or bare object (json). It does NOT wrap the record in
// a list. csv, md, and ingr formats fall back to a single-row table by
// invoking the tabular helpers with a one-element slice.
func writeSingleRecord(w io.Writer, record map[string]any, format string, columns []string) error {
	switch format {
	case "yaml", "yml", "":
		out, err := yaml.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = w.Write(out)
		return err
	case "json":
		out, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, err = fmt.Fprintf(w, "%s\n", out)
		return err
	case "csv":
		return writeCSV(w, []map[string]any{record}, columns)
	case "md", "markdown":
		return writeMarkdown(w, []map[string]any{record}, columns)
	case "ingr":
		return writeINGR(w, []map[string]any{record}, columns)
	default:
		return fmt.Errorf("unknown format %q, use yaml, json, csv, md, or ingr", format)
	}
}

// writeINGR emits records in the project's native INGR format
// (`# INGR.io | …` header, JSON-encoded cells one per line,
// `# N records` footer). Delegates to materializer.FormatINGR after
// adapting map[string]any rows into ingitdb.RecordEntry values.
//
// columns may be nil — in that case the union of keys across rows is
// used in deterministic order (matching writeCSV's collectColumns).
// The viewName is "select" — a synthetic identifier that flows into
// the INGR header line.
func writeINGR(w io.Writer, rows []map[string]any, columns []string) error {
	if columns == nil {
		columns = collectColumns(rows)
	}
	entries := make([]ingitdb.IRecordEntry, 0, len(rows))
	for _, row := range rows {
		id := ""
		if v, ok := row["$id"]; ok {
			id = fmt.Sprintf("%v", v)
		}
		// Strip $id from the data payload — the INGR header lists
		// columns explicitly, and $id is conveyed via GetID(). Cloning
		// avoids mutating the caller's map.
		data := make(map[string]any, len(row))
		for k, v := range row {
			if k == "$id" {
				continue
			}
			data[k] = v
		}
		entries = append(entries, ingitdb.RecordEntry{ID: id, Data: data})
	}
	// Filter $id out of the columns slice too — it would otherwise
	// produce a duplicate header column. If the caller asked for $id
	// explicitly via --fields, INGR's header already encodes the key
	// position implicitly.
	cleaned := make([]string, 0, len(columns))
	for _, c := range columns {
		if c == "$id" {
			continue
		}
		cleaned = append(cleaned, c)
	}
	out, err := materializer.FormatINGR("select", cleaned, entries)
	if err != nil {
		return fmt.Errorf("format ingr: %w", err)
	}
	_, err = w.Write(out)
	return err
}
