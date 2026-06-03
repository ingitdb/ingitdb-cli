package dalgo2ingitdb

import (
	"fmt"
	"reflect"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// idColumnName is the reserved recordset column that carries each record's key.
// It is not a declared schema column; the Starlark evaluator strips it from the
// stored field map so formula inputs match the eager ApplyFormulasToRead
// pipeline exactly. The "$id" spelling mirrors the dal pseudo-field convention
// and cannot collide with a real column (Starlark identifiers cannot contain
// "$", so no formula can reference it).
const idColumnName = "$id"

// formulaEvaluator is a recordset.Evaluator that computes a computed column's
// value by delegating to ingitdb.EvaluateFormula. dalgo carries no scripting
// dependency: the Starlark engine, its sandbox, and schema-load-time validation
// remain entirely in ingitdb.
type formulaEvaluator struct {
	formula string
}

var _ recordset.Evaluator = formulaEvaluator{}

// Eval evaluates the formula against the row's stored sibling values. The
// reserved idColumnName entry is removed first so the field set handed to
// Starlark is identical to the eager pipeline's (which never carried "$id").
func (e formulaEvaluator) Eval(stored map[string]any) (any, error) {
	fields := stored
	if _, ok := stored[idColumnName]; ok {
		fields = make(map[string]any, len(stored))
		for k, v := range stored {
			if k == idColumnName {
				continue
			}
			fields[k] = v
		}
	}
	return ingitdb.EvaluateFormula(e.formula, fields)
}

// anyColumn is a recordset.Column[any] that stores arbitrary per-row values,
// including nil. dalgo's UntypedColWrapper[any] cannot back a stored column here
// because its Add rejects a nil interface value (the assertion value.(any) is
// false for an untyped nil), so a nil/absent stored field would fail to grow the
// column on NewRow. anyColumn appends unconditionally.
type anyColumn struct {
	name   string
	values []any
}

var _ recordset.Column[any] = (*anyColumn)(nil)

func (c *anyColumn) Name() string      { return c.name }
func (c *anyColumn) DefaultValue() any { return nil }
func (c *anyColumn) DbType() string    { return "" }
func (c *anyColumn) IsBitmap() bool    { return false }
func (c *anyColumn) ValueType() reflect.Type {
	return reflect.TypeFor[any]()
}

func (c *anyColumn) Add(value any) error {
	c.values = append(c.values, value)
	return nil
}

func (c *anyColumn) GetValue(row int) (any, error) {
	if row < 0 || row >= len(c.values) {
		return nil, fmt.Errorf("row %d out of range for %d rows", row, len(c.values))
	}
	return c.values[row], nil
}

func (c *anyColumn) SetValue(row int, value any) error {
	if row < 0 || row >= len(c.values) {
		return fmt.Errorf("row %d out of range for %d rows", row, len(c.values))
	}
	c.values[row] = value
	return nil
}

func (c *anyColumn) Values() []any {
	out := make([]any, len(c.values))
	copy(out, c.values)
	return out
}

// KeyedStored pairs a record key with its locale-normalized stored fields. The
// stored map holds only stored (non-computed) values; computed columns are
// resolved lazily through the recordset, never baked in here.
type KeyedStored struct {
	Key    string
	Stored map[string]any
}

// BuildRecordset assembles a recordset.Recordset for a collection: a reserved
// "$id" column carrying each record key, one ordinary column per stored
// (non-formula) column carrying the per-record stored value, and one
// recordset.NewComputedColumn per formula column bound to a Starlark-backed
// evaluator. Computed values are never evaluated here — they resolve lazily,
// at most once per row, when a consumer reads them.
func BuildRecordset(colDef *ingitdb.CollectionDef, records []KeyedStored) recordset.Recordset {
	return buildRecordset(colDef, records, func(formula string) recordset.Evaluator {
		return formulaEvaluator{formula: formula}
	})
}

// buildRecordset is BuildRecordset with an injectable evaluator factory so tests
// can substitute a counting evaluator to prove lazy, once-per-row resolution.
func buildRecordset(colDef *ingitdb.CollectionDef, records []KeyedStored, evalFor func(formula string) recordset.Evaluator) recordset.Recordset {
	storedNames := orderedColumns(colDef, func(c *ingitdb.ColumnDef) bool { return c.Formula == "" })
	computedNames := orderedComputedColumns(colDef)

	cols := make([]recordset.Column[any], 0, len(storedNames)+len(computedNames)+1)
	cols = append(cols, &anyColumn{name: idColumnName})
	storedSet := make(map[string]bool, len(storedNames))
	for _, name := range storedNames {
		cols = append(cols, &anyColumn{name: name})
		storedSet[name] = true
	}
	for _, name := range computedNames {
		cols = append(cols, recordset.NewComputedColumn(name, evalFor(colDef.Columns[name].Formula)))
	}

	rs := recordset.NewColumnarRecordset(colDef.ID, cols...)
	for _, rec := range records {
		row := rs.NewRow()
		// SetValueByName on a stored column cannot fail here: every column was
		// added above and the column value type is any, so the type assertion
		// inside the recordset always succeeds.
		_ = row.SetValueByName(idColumnName, rec.Key, rs)
		for name, value := range rec.Stored {
			if !storedSet[name] {
				continue
			}
			_ = row.SetValueByName(name, value, rs)
		}
	}
	return rs
}

// recordsetReader is a slice-backed dal.RecordsetReader over a prebuilt
// recordset. Rows are yielded in insertion order.
type recordsetReader struct {
	rs    recordset.Recordset
	index int
}

var _ dal.RecordsetReader = (*recordsetReader)(nil)

// NewRecordsetReader returns a dal.RecordsetReader that walks the rows of rs.
func NewRecordsetReader(rs recordset.Recordset) dal.RecordsetReader {
	return &recordsetReader{rs: rs}
}

// Recordset returns the underlying recordset.
func (r *recordsetReader) Recordset() recordset.Recordset {
	return r.rs
}

// Next yields the next row, or dal.ErrNoMoreRecords when the rows are exhausted.
func (r *recordsetReader) Next() (recordset.Row, recordset.Recordset, error) {
	if r.index >= r.rs.RowsCount() {
		return nil, r.rs, dal.ErrNoMoreRecords
	}
	row := r.rs.GetRow(r.index)
	r.index++
	return row, r.rs, nil
}

// Cursor is not supported; the reader returns the full result set eagerly.
func (r *recordsetReader) Cursor() (string, error) {
	return "", nil
}

// Close is a no-op: the reader holds an in-memory recordset.
func (r *recordsetReader) Close() error {
	return nil
}
