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
