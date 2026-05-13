package dalgo2ingitdb

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ingr-io/ingr-go/ingr"
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

// ParseBatchINGR reads an INGR multi-record stream from r and returns
// one ParsedRecord per record. The key is read from the reserved $ID
// column (INGR's key field; note the uppercase). $ID is stripped from
// the returned Data map. Position is the 1-based record index.
func ParseBatchINGR(r io.Reader) ([]ParsedRecord, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read ingr stream: %w", err)
	}
	if len(content) == 0 {
		return nil, nil
	}
	var rows []map[string]any
	if err := ingr.Unmarshal(content, &rows); err != nil {
		return nil, fmt.Errorf("parse ingr stream: %w", err)
	}
	records := make([]ParsedRecord, 0, len(rows))
	for i, row := range rows {
		pos := i + 1
		idRaw, ok := row["$ID"]
		if !ok {
			return nil, fmt.Errorf("record %d: missing required $ID column", pos)
		}
		key, ok := idRaw.(string)
		if !ok {
			return nil, fmt.Errorf("record %d: $ID must be a string, got %T", pos, idRaw)
		}
		if key == "" {
			return nil, fmt.Errorf("record %d: $ID is empty", pos)
		}
		delete(row, "$ID")
		records = append(records, ParsedRecord{
			Position: pos,
			Key:      key,
			Data:     row,
		})
	}
	return records, nil
}

// CSVParseOptions controls CSV-specific behavior.
type CSVParseOptions struct {
	// KeyColumn, if non-empty, names the column to use as the record
	// key (overrides $id/id auto-resolution).
	KeyColumn string
	// Fields, if non-empty, replaces the header row: the first stdin
	// line is treated as data and these names are used for column
	// mapping.
	Fields []string
}

// ParseBatchCSV reads RFC 4180 CSV from r and returns one ParsedRecord
// per data row. Key resolution precedence is:
//  1. opts.KeyColumn if set (rejected before reading rows if column missing).
//  2. column named "$id" if present.
//  3. column named "id" if present (auto-mapped).
//  4. otherwise error.
//
// When both "$id" and "id" columns exist without opts.KeyColumn, "$id"
// wins; "id" is kept as a data field. The resolved key column's value
// is stripped from Data.
//
// If opts.Fields is non-empty, those names override the header row:
// the first stdin line is treated as data, and Position is 1-based
// against data rows. Otherwise Position is 1-based against source
// lines, so the header is line 1 and the first data row is line 2.
func ParseBatchCSV(r io.Reader, opts CSVParseOptions) ([]ParsedRecord, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // we validate manually so we can error per line

	var header []string
	var firstDataLine int
	if len(opts.Fields) > 0 {
		header = append([]string(nil), opts.Fields...)
		firstDataLine = 1
	} else {
		h, err := cr.Read()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read csv header: %w", err)
		}
		header = h
		firstDataLine = 2
	}
	if len(header) == 0 {
		return nil, fmt.Errorf("csv header is empty")
	}

	// Resolve which column is the key.
	keyCol, keyColIdx, err := resolveCSVKeyColumn(header, opts.KeyColumn)
	if err != nil {
		return nil, err
	}

	var records []ParsedRecord
	lineNo := firstDataLine - 1
	for {
		fields, readErr := cr.Read()
		lineNo++
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("line %d: csv parse error: %w", lineNo, readErr)
		}
		if len(fields) != len(header) {
			return nil, fmt.Errorf("line %d: row has %d columns, header has %d", lineNo, len(fields), len(header))
		}
		keyVal := fields[keyColIdx]
		if keyVal == "" {
			return nil, fmt.Errorf("line %d: key column %q is empty", lineNo, keyCol)
		}
		data := make(map[string]any, len(header)-1)
		for i, col := range header {
			if i == keyColIdx {
				continue
			}
			data[col] = fields[i]
		}
		records = append(records, ParsedRecord{
			Position: lineNo,
			Key:      keyVal,
			Data:     data,
		})
	}
	return records, nil
}

func resolveCSVKeyColumn(header []string, override string) (string, int, error) {
	if override != "" {
		for i, h := range header {
			if h == override {
				return override, i, nil
			}
		}
		return "", -1, fmt.Errorf("--key-column=%q not found in CSV header %v", override, header)
	}
	// Look for $id first (wins precedence over id).
	for i, h := range header {
		if h == "$id" {
			return "$id", i, nil
		}
	}
	for i, h := range header {
		if h == "id" {
			return "id", i, nil
		}
	}
	return "", -1, fmt.Errorf("no key column found in CSV header %v; use --key-column, or include a $id or id column", header)
}
