package ingitdb

import (
	"fmt"
	"slices"

	"go.starlark.net/syntax"
)

// computedColumnTypes lists the declared column types supported for computed
// (formula) columns in this Feature.
var computedColumnTypes = []ColumnType{
	ColumnTypeString,
	ColumnTypeInt,
	ColumnTypeFloat,
	ColumnTypeBool,
	ColumnTypeAny,
}

// validateComputedColumn validates a column whose Formula is non-empty.
// It enforces that:
//   - the declared Type is one of the supported computed-column types,
//   - the Formula parses as a single Starlark expression, and
//   - the Formula references only stored (non-computed) sibling columns.
//
// Errors name the collection and the column to aid diagnosis.
func validateComputedColumn(collectionID, colName string, col *ColumnDef, columns map[string]*ColumnDef) error {
	if !slices.Contains(computedColumnTypes, col.Type) {
		return fmt.Errorf("collection '%s': computed column '%s' has unsupported type '%s': computed columns support only string, int, float, bool, and any",
			collectionID, colName, col.Type)
	}

	var opts syntax.FileOptions
	expr, err := opts.ParseExpr(colName, col.Formula, 0)
	if err != nil {
		return fmt.Errorf("collection '%s': invalid formula for column '%s': %w", collectionID, colName, err)
	}

	var refErr error
	syntax.Walk(expr, func(n syntax.Node) bool {
		ident, ok := n.(*syntax.Ident)
		if !ok {
			return true
		}
		sibling, exists := columns[ident.Name]
		if exists && sibling.Formula != "" {
			refErr = fmt.Errorf("collection '%s': formula for column '%s' references computed column '%s': a formula may reference only stored fields",
				collectionID, colName, ident.Name)
			return false
		}
		return true
	})
	return refErr
}
