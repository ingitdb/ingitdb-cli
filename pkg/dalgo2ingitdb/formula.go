package dalgo2ingitdb

import (
	"fmt"
	"maps"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ApplyFormulasToRead computes the value of every computed column (one with a
// non-empty Formula) and adds it to the returned map, coerced to the column's
// declared type. The stored fields in data are used as the formula's variable
// bindings. The input map is not mutated; a clone is returned.
//
// A runtime evaluation error, or a result that cannot be coerced to the
// declared type, aborts with an error naming the collection, record key, and
// column. No partial result is returned in that case.
func ApplyFormulasToRead(data map[string]any, cols map[string]*ingitdb.ColumnDef, collectionID, recordKey string) (map[string]any, error) {
	if len(cols) == 0 {
		return data, nil
	}
	result := maps.Clone(data)
	for colName, colDef := range cols {
		if colDef.Formula == "" {
			continue
		}
		raw, err := ingitdb.EvaluateFormula(colDef.Formula, data)
		if err != nil {
			return nil, fmt.Errorf("collection %q record %q column %q: %w", collectionID, recordKey, colName, err)
		}
		coerced, err := coerceFormulaResult(raw, colDef.Type)
		if err != nil {
			return nil, fmt.Errorf("collection %q record %q column %q: %w", collectionID, recordKey, colName, err)
		}
		result[colName] = coerced
	}
	return result, nil
}

// coerceFormulaResult coerces a formula evaluation result (string, bool, int64,
// float64, or nil) to the column's declared type. A result whose Go type cannot
// represent the declared type yields an error.
func coerceFormulaResult(v any, colType ingitdb.ColumnType) (any, error) {
	switch colType {
	case ingitdb.ColumnTypeString:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("cannot coerce %T to string", v)
		}
		return s, nil
	case ingitdb.ColumnTypeInt:
		switch n := v.(type) {
		case int64:
			return n, nil
		case float64:
			i := int64(n)
			if float64(i) != n {
				return nil, fmt.Errorf("cannot coerce non-integral float %v to int", n)
			}
			return i, nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to int", v)
		}
	case ingitdb.ColumnTypeFloat:
		switch n := v.(type) {
		case float64:
			return n, nil
		case int64:
			return float64(n), nil
		default:
			return nil, fmt.Errorf("cannot coerce %T to float", v)
		}
	case ingitdb.ColumnTypeBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("cannot coerce %T to bool", v)
		}
		return b, nil
	case ingitdb.ColumnTypeAny:
		return v, nil
	default:
		return nil, fmt.Errorf("computed columns do not support type %q", colType)
	}
}
