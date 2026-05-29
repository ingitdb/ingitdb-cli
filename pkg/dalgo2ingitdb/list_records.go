package dalgo2ingitdb

// specscore: feature/record-format/list-of-records

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// listKeySeparator joins composite primary-key values into a single record key.
const listKeySeparator = "\x1f"

// ParseListOfRecordsContent parses a list-of-records file into ordered row maps.
// It handles a top-level YAML sequence, a top-level JSON array, and a JSONL
// stream (one JSON object per non-empty line). Empty content yields no rows.
// csv and ingr keep their dedicated parsers and are not handled here.
func ParseListOfRecordsContent(content []byte, format ingitdb.RecordFormat) ([]map[string]any, error) {
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		return parseYAMLList(content)
	case ingitdb.RecordFormatJSON:
		return parseJSONList(content)
	case ingitdb.RecordFormatJSONL:
		return parseJSONLList(content)
	default:
		return nil, fmt.Errorf("format %q is not a list-of-records format", format)
	}
}

func parseYAMLList(content []byte) ([]map[string]any, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	if err := yaml.Unmarshal(content, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse YAML list: %w", err)
	}
	return rows, nil
}

func parseJSONList(content []byte) ([]map[string]any, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(content, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse JSON list: %w", err)
	}
	return rows, nil
}

func parseJSONLList(content []byte) ([]map[string]any, error) {
	var rows []map[string]any
	for i, raw := range bytes.Split(content, []byte("\n")) {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("failed to parse JSONL line %d: %w", i+1, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ResolveListRecordKey resolves a list row's record key, in order: the
// collection's declared primary_key (composite values joined), else a "$id"
// field, else an "id" field. ok is false when none is available, meaning the
// list has no usable record key.
func ResolveListRecordKey(row map[string]any, colDef *ingitdb.CollectionDef) (string, bool) {
	if colDef != nil && len(colDef.PrimaryKey) > 0 {
		parts := make([]string, len(colDef.PrimaryKey))
		for i, col := range colDef.PrimaryKey {
			parts[i] = fmt.Sprintf("%v", row[col])
		}
		return strings.Join(parts, listKeySeparator), true
	}
	for _, candidate := range []string{"$id", "id"} {
		if v, ok := row[candidate]; ok {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}
