package dalgo2ingitdb

import (
	"fmt"

	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// AccessValue reads colName from a recordset row — the single coerce-on-access
// path every read consumer (select/delete/update/TUI) uses.
//
// For a computed (formula) column the raw evaluator result is coerced to the
// column's declared ColumnType via coerceFormulaResult, so typed results stay
// identical to the eager ApplyFormulasToRead pipeline. A stored column's value
// (colDef nil or no Formula) is returned unchanged — the eager pipeline never
// coerced stored values, so neither do we.
//
// Because computation is lazy, evaluation happens here, on access. Any evaluator
// or coercion error is wrapped with the collection, record key, and column, so
// the failure is fail-loud and names its source (matching the eager pipeline's
// error format).
func AccessValue(row recordset.Row, rs recordset.Recordset, collectionID, recordKey, colName string, colDef *ingitdb.ColumnDef) (any, error) {
	raw, err := row.GetValueByName(colName, rs)
	if err != nil {
		return nil, fmt.Errorf("collection %q record %q column %q: %w", collectionID, recordKey, colName, err)
	}
	if colDef == nil || colDef.Formula == "" {
		return raw, nil
	}
	coerced, err := coerceFormulaResult(raw, colDef.Type)
	if err != nil {
		return nil, fmt.Errorf("collection %q record %q column %q: %w", collectionID, recordKey, colName, err)
	}
	return coerced, nil
}

// RowData reads the named columns of a recordset row through AccessValue and
// returns them as a map, omitting nil values so the result matches the ragged
// record map the eager pipeline produced (absent fields were never keys). Only
// the requested columns are read, so an unreferenced computed column is never
// evaluated; a referenced computed column that errors surfaces fail-loud.
func RowData(row recordset.Row, rs recordset.Recordset, collectionID, recordKey string, colDef *ingitdb.CollectionDef, names []string) (map[string]any, error) {
	data := make(map[string]any, len(names))
	for _, name := range names {
		v, err := AccessValue(row, rs, collectionID, recordKey, name, colDef.Columns[name])
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		data[name] = v
	}
	return data, nil
}

// AllColumnNames returns every column name of rs except the reserved IDColumn.
func AllColumnNames(rs recordset.Recordset) []string {
	cols := rs.Columns()
	names := make([]string, 0, len(cols))
	for _, col := range cols {
		if col.Name() == IDColumn {
			continue
		}
		names = append(names, col.Name())
	}
	return names
}

// StoredColumnNames returns the stored (non-computed) column names of rs except
// the reserved IDColumn. Used by write consumers that must persist stored fields
// only, never computed values.
func StoredColumnNames(rs recordset.Recordset) []string {
	cols := rs.Columns()
	names := make([]string, 0, len(cols))
	for _, col := range cols {
		if col.Name() == IDColumn {
			continue
		}
		if _, computed := col.(recordset.ComputedColumn); computed {
			continue
		}
		names = append(names, col.Name())
	}
	return names
}
