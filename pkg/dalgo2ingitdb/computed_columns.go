package dalgo2ingitdb

import (
	"fmt"
	"sort"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// validateNoStoredComputedValues rejects any record that supplies a value for a
// computed column (a column with a non-empty Formula). A computed column's value
// is derived solely from its formula and must never be stored. The presence of
// the column's key in data is a rejection regardless of the value.
func (r readwriteTx) validateNoStoredComputedValues(collectionID string, colDef *ingitdb.CollectionDef, recordKey string, data map[string]any) error {
	for _, field := range orderedComputedColumns(colDef) {
		if _, ok := data[field]; ok {
			return fmt.Errorf("dalgo2ingitdb: collection %q record %q column %q is a computed column and cannot be stored", collectionID, recordKey, field)
		}
	}
	return nil
}

// orderedComputedColumns returns the names of computed columns (Formula != "")
// in a deterministic order: columns named in ColumnsOrder first, then any
// remaining computed columns sorted lexicographically.
func orderedComputedColumns(colDef *ingitdb.CollectionDef) []string {
	if colDef == nil || len(colDef.Columns) == 0 {
		return nil
	}
	fields := make([]string, 0, len(colDef.Columns))
	seen := make(map[string]bool, len(colDef.Columns))
	for _, field := range colDef.ColumnsOrder {
		column, ok := colDef.Columns[field]
		if !ok || column == nil || column.Formula == "" {
			continue
		}
		fields = append(fields, field)
		seen[field] = true
	}
	var remaining []string
	for field, column := range colDef.Columns {
		if seen[field] || column == nil || column.Formula == "" {
			continue
		}
		remaining = append(remaining, field)
	}
	sort.Strings(remaining)
	fields = append(fields, remaining...)
	return fields
}
