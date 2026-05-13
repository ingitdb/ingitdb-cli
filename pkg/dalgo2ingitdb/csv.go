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
