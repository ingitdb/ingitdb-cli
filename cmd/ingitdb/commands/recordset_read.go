package commands

// specscore: feature/computed-columns-via-dalgo

import (
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// rowData reads the named columns of a recordset row through the shared
// coerce-on-access accessor, returning a map equivalent to the eager
// ApplyFormulasToRead result for those columns: stored values plus computed
// values resolved lazily on access. Only the requested columns are read, so an
// unreferenced computed column is never evaluated. Nil values are omitted so the
// map matches the ragged record map the eager pipeline produced (absent fields
// were never keys). A referenced computed column that errors surfaces fail-loud.
func rowData(row recordset.Row, rs recordset.Recordset, collectionID, recKey string, colDef *ingitdb.CollectionDef, names []string) (map[string]any, error) {
	data := make(map[string]any, len(names))
	for _, name := range names {
		v, err := dalgo2ingitdb.AccessValue(row, rs, collectionID, recKey, name, colDef.Columns[name])
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

// allColumnNames returns every column name of rs except the reserved $id column.
func allColumnNames(rs recordset.Recordset) []string {
	cols := rs.Columns()
	names := make([]string, 0, len(cols))
	for _, col := range cols {
		if col.Name() == dalgo2ingitdb.IDColumn {
			continue
		}
		names = append(names, col.Name())
	}
	return names
}
