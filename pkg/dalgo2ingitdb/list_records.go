package dalgo2ingitdb

// specscore: feature/record-format/list-of-records

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
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

// EncodeListOfRecordsContent serializes list rows back to the declared format,
// preserving record (insertion) order. Within each record, keys are emitted in
// columnsOrder first, then remaining keys alphabetically. JSONL writes one
// compact JSON object per line; JSON writes a pretty array; YAML writes a
// top-level sequence. csv and ingr keep their dedicated encoders.
func EncodeListOfRecordsContent(rows []map[string]any, format ingitdb.RecordFormat, columnsOrder []string) ([]byte, error) {
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		return encodeYAMLList(rows, columnsOrder)
	case ingitdb.RecordFormatJSON:
		return encodeJSONList(rows, columnsOrder)
	case ingitdb.RecordFormatJSONL:
		return encodeJSONLList(rows, columnsOrder)
	default:
		return nil, fmt.Errorf("format %q is not a list-of-records format", format)
	}
}

// orderRecordKeys returns a record's keys with columnsOrder entries first (in
// order, only those present), then the remaining keys sorted alphabetically.
func orderRecordKeys(rec map[string]any, columnsOrder []string) []string {
	seen := make(map[string]bool, len(rec))
	ordered := make([]string, 0, len(rec))
	for _, k := range columnsOrder {
		if _, ok := rec[k]; ok && !seen[k] {
			ordered = append(ordered, k)
			seen[k] = true
		}
	}
	rest := make([]string, 0, len(rec))
	for k := range rec {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// jsonObjectOrdered marshals one record as a compact JSON object with keys in
// the resolved order.
func jsonObjectOrdered(rec map[string]any, columnsOrder []string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range orderRecordKeys(rec, columnsOrder) {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyBytes, _ := json.Marshal(k) // a string key never fails to marshal
		valBytes, err := json.Marshal(rec[k])
		if err != nil {
			return nil, fmt.Errorf("failed to encode field %q: %w", k, err)
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(valBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func encodeJSONList(rows []map[string]any, columnsOrder []string) ([]byte, error) {
	if len(rows) == 0 {
		return []byte("[]\n"), nil
	}
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, rec := range rows {
		obj, err := jsonObjectOrdered(rec, columnsOrder)
		if err != nil {
			return nil, err
		}
		buf.WriteString("  ")
		buf.Write(obj)
		if i < len(rows)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n")
	return buf.Bytes(), nil
}

func encodeJSONLList(rows []map[string]any, columnsOrder []string) ([]byte, error) {
	var buf bytes.Buffer
	for _, rec := range rows {
		obj, err := jsonObjectOrdered(rec, columnsOrder)
		if err != nil {
			return nil, err
		}
		buf.Write(obj)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func encodeYAMLList(rows []map[string]any, columnsOrder []string) ([]byte, error) {
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, rec := range rows {
		m := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range orderRecordKeys(rec, columnsOrder) {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			valNode := &yaml.Node{}
			// yaml signals unencodable values by panicking (matching the rest of
			// the codebase), so the returned error is never meaningful here.
			_ = valNode.Encode(rec[k])
			m.Content = append(m.Content, keyNode, valNode)
		}
		seq.Content = append(seq.Content, m)
	}
	return yaml.Marshal(seq)
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
