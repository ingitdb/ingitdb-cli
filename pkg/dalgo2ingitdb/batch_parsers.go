package dalgo2ingitdb

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsedRecord is one record extracted from a batch stream.
type ParsedRecord struct {
	// Position is 1-based: line number for jsonl/csv, document index
	// for yaml/ingr. For csv with a header row, Position 2 is the
	// first data record.
	Position int
	// Key is the resolved record key (from $id, id, or --key-column).
	Key string
	// Data is the record's structured fields with the key field stripped.
	Data map[string]any
}

// ParseBatchJSONL reads NDJSON from r and returns one ParsedRecord per
// non-blank line. Each record MUST have a top-level $id; the $id is
// stripped from the returned Data map. Blank lines are skipped but
// counted for the Position field.
func ParseBatchJSONL(r io.Reader) ([]ParsedRecord, error) {
	scanner := bufio.NewScanner(r)
	// Allow large records — default 64KiB is too small for realistic batches.
	const maxLine = 1 << 22 // 4 MiB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLine)

	var records []ParsedRecord
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Bytes()
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}
		var data map[string]any
		err := json.Unmarshal(raw, &data)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", lineNo, err)
		}
		idRaw, ok := data["$id"]
		if !ok {
			return nil, fmt.Errorf("line %d: record missing required $id field", lineNo)
		}
		key, ok := idRaw.(string)
		if !ok {
			return nil, fmt.Errorf("line %d: $id must be a string, got %T", lineNo, idRaw)
		}
		if key == "" {
			return nil, fmt.Errorf("line %d: $id is empty", lineNo)
		}
		delete(data, "$id")
		records = append(records, ParsedRecord{
			Position: lineNo,
			Key:      key,
			Data:     data,
		})
	}
	err := scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("read jsonl stream: %w", err)
	}
	return records, nil
}

// ParseBatchYAMLStream reads a YAML multi-document stream from r and
// returns one ParsedRecord per non-nil document. Each record MUST have
// a top-level $id; $id is stripped from the returned Data map.
// Position is the 1-based document index.
func ParseBatchYAMLStream(r io.Reader) ([]ParsedRecord, error) {
	dec := yaml.NewDecoder(r)
	var records []ParsedRecord
	docNo := 0
	for {
		var data map[string]any
		err := dec.Decode(&data)
		if err == io.EOF {
			break
		}
		docNo++
		if err != nil {
			return nil, fmt.Errorf("document %d: invalid YAML: %w", docNo, err)
		}
		if data == nil {
			// Skip empty documents (e.g. trailing "---\n").
			continue
		}
		idRaw, ok := data["$id"]
		if !ok {
			return nil, fmt.Errorf("document %d: record missing required $id field", docNo)
		}
		key, ok := idRaw.(string)
		if !ok {
			return nil, fmt.Errorf("document %d: $id must be a string, got %T", docNo, idRaw)
		}
		if key == "" {
			return nil, fmt.Errorf("document %d: $id is empty", docNo)
		}
		delete(data, "$id")
		records = append(records, ParsedRecord{
			Position: docNo,
			Key:      key,
			Data:     data,
		})
	}
	return records, nil
}
