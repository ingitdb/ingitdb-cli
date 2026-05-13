package dalgo2ingitdb

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// recordsKey is the key under which a parsed CSV list of rows is exposed
// in the map[string]any returned by ParseRecordContentForCollection.
// Callers reach the rows via data["$records"] (typed []map[string]any).
const recordsKey = "$records"

// parseCSVForCollection reads RFC 4180 CSV bytes, validates the header
// matches colDef.ColumnsOrder exactly (same names, same order), and
// returns the rows as a list of records keyed by column name.
//
// The result is wrapped in map[string]any{"$records": []map[string]any{...}}
// to satisfy the ParseRecordContentForCollection contract (which is
// declared as returning map[string]any) without losing list-of-records
// semantics — the caller unwraps via the recordsKey constant.
func parseCSVForCollection(content []byte, colDef *ingitdb.CollectionDef) (map[string]any, error) {
	if len(colDef.ColumnsOrder) == 0 {
		return nil, fmt.Errorf("csv read requires non-empty columns_order on the collection definition")
	}
	r := csv.NewReader(bytes.NewReader(content))
	header, err := r.Read()
	if err == io.EOF {
		return nil, fmt.Errorf("csv input is empty (expected header row)")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read csv header: %w", err)
	}
	if err = validateCSVHeader(header, colDef.ColumnsOrder); err != nil {
		return nil, err
	}
	var rows []map[string]any
	for {
		fields, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read csv row %d: %w", len(rows)+1, readErr)
		}
		if len(fields) != len(header) {
			return nil, fmt.Errorf("csv row %d has %d columns, header has %d",
				len(rows)+1, len(fields), len(header))
		}
		row := make(map[string]any, len(header))
		for i, col := range header {
			row[col] = fields[i]
		}
		rows = append(rows, row)
	}
	return map[string]any{recordsKey: rows}, nil
}

// validateCSVHeader returns an error when header does not match expected
// exactly (same names in the same order). The error message names the
// first mismatched column and whether it's a missing, extra, or reordered
// column.
func validateCSVHeader(header, expected []string) error {
	if len(header) < len(expected) {
		missing := expected[len(header):]
		return fmt.Errorf("csv header is missing column(s): %v (expected %v, got %v)",
			missing, expected, header)
	}
	if len(header) > len(expected) {
		extra := header[len(expected):]
		return fmt.Errorf("csv header has extra column(s): %v (expected %v, got %v)",
			extra, expected, header)
	}
	for i := range expected {
		if header[i] != expected[i] {
			return fmt.Errorf("csv header column at position %d is %q, expected %q (full order mismatch: got %v, expected %v)",
				i, header[i], expected[i], header, expected)
		}
	}
	return nil
}

// encodeCSVForCollection serializes a list of records as RFC 4180 CSV.
// Header is emitted first, with column names in colDef.ColumnsOrder. Each
// data row's cell values are looked up by column name in the same order.
//
// rows MUST be []map[string]any (a list of records). Keyed maps
// (map[string]map[string]any) are rejected — Go map iteration order is
// non-deterministic and CSV row order matters.
//
// Per-cell values are written via fmt.Sprintf("%v", v) for primitives;
// nested objects (map[...]any) and array values (slices) are rejected
// with a typed error naming the offending field.
func encodeCSVForCollection(value any, colDef *ingitdb.CollectionDef) ([]byte, error) {
	rows, err := coerceToRowList(value)
	if err != nil {
		return nil, err
	}
	if len(colDef.ColumnsOrder) == 0 {
		return nil, fmt.Errorf("csv write requires non-empty columns_order on the collection definition")
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err = w.Write(colDef.ColumnsOrder); err != nil {
		return nil, fmt.Errorf("failed to write csv header: %w", err)
	}
	for i, row := range rows {
		cells := make([]string, len(colDef.ColumnsOrder))
		for j, col := range colDef.ColumnsOrder {
			raw, ok := row[col]
			if !ok {
				cells[j] = ""
				continue
			}
			cell, cellErr := csvCellString(raw, col, i)
			if cellErr != nil {
				return nil, cellErr
			}
			cells[j] = cell
		}
		if err = w.Write(cells); err != nil {
			return nil, fmt.Errorf("failed to write csv row %d: %w", i, err)
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return nil, fmt.Errorf("csv writer error: %w", err)
	}
	return buf.Bytes(), nil
}

// coerceToRowList accepts []map[string]any and rejects every other shape
// — including map[string]map[string]any (keyed input) — with a typed
// error explaining that csv requires a deterministically-ordered list.
func coerceToRowList(value any) ([]map[string]any, error) {
	switch v := value.(type) {
	case []map[string]any:
		return v, nil
	case map[string]map[string]any:
		return nil, fmt.Errorf("csv accepts only []map[string]any (a list of records), not a keyed map (map[string]map[string]any); map iteration order is non-deterministic and csv row order matters")
	case []any:
		// Allow []any of map[string]any rows for caller convenience.
		out := make([]map[string]any, 0, len(v))
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("csv row %d is not a map (got %T)", i, item)
			}
			out = append(out, m)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("csv accepts only []map[string]any (a list of records), got %T", value)
	}
}

// csvCellString converts a single cell value to its CSV string form.
// Nested objects (map[...]any) and array values (slices) are rejected
// with a typed error that identifies the field name and the record's
// position in the input.
func csvCellString(v any, fieldName string, rowIndex int) (string, error) {
	switch val := v.(type) {
	case nil:
		return "", nil
	case string:
		return val, nil
	case map[string]any:
		return "", fmt.Errorf("csv does not support nested or array-valued fields: row %d field %q has value of type %T",
			rowIndex, fieldName, v)
	case []any, []string, []int, []float64, []bool, []map[string]any:
		return "", fmt.Errorf("csv does not support nested or array-valued fields: row %d field %q has value of type %T",
			rowIndex, fieldName, v)
	default:
		return fmt.Sprintf("%v", val), nil
	}
}
