package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ordersColDef has stored qty/price (ints) plus computed columns: total
// (qty*price, int), ratio (qty/0, int — raises at runtime) and bad (a string
// literal declared as int — coercion mismatch).
func ordersColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID: "orders",
		Columns: map[string]*ingitdb.ColumnDef{
			"qty":   {Type: ingitdb.ColumnTypeInt},
			"price": {Type: ingitdb.ColumnTypeInt},
			"total": {Type: ingitdb.ColumnTypeInt, Formula: "qty * price"},
			"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty / 0"},
			"bad":   {Type: ingitdb.ColumnTypeInt, Formula: `"not an int"`},
		},
		ColumnsOrder: []string{"qty", "price", "total", "ratio", "bad"},
	}
}

// AC: type-coercion-preserved — an int computed column reads back as int64, not
// float, exactly like the eager pipeline.
func TestAccessValue_TypeCoercionPreserved(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int64(3), "price": int64(4)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	got, err := AccessValue(row, rs, "orders", "o1", "total", ordersColDef().Columns["total"])
	if err != nil {
		t.Fatalf("AccessValue(total): %v", err)
	}
	if got != int64(12) {
		t.Errorf("total = %#v, want int64(12)", got)
	}
}

// AC: referenced-erroring-column-fails-loud — reading a computed column whose
// formula raises aborts with an error naming collection, record key, and column.
func TestAccessValue_ReferencedErroringColumnFailsLoud(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int64(3), "price": int64(4)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	_, err := AccessValue(row, rs, "orders", "o1", "ratio", ordersColDef().Columns["ratio"])
	if err == nil {
		t.Fatal("want fail-loud error for erroring computed column")
	}
	for _, want := range []string{`collection "orders"`, `record "o1"`, `column "ratio"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// A computed result that cannot be coerced to the declared type fails loud too.
func TestAccessValue_CoercionMismatchFailsLoud(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int64(3), "price": int64(4)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	_, err := AccessValue(row, rs, "orders", "o1", "bad", ordersColDef().Columns["bad"])
	if err == nil {
		t.Fatal("want coercion-mismatch error")
	}
	if !strings.Contains(err.Error(), `column "bad"`) {
		t.Errorf("error %q should name the bad column", err.Error())
	}
}

// A stored column's value is returned unchanged — never run through
// coerceFormulaResult (which would reject a Go int for an int column).
func TestAccessValue_StoredValuePassesThroughUncoerced(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int(3)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	got, err := AccessValue(row, rs, "orders", "o1", "qty", ordersColDef().Columns["qty"])
	if err != nil {
		t.Fatalf("AccessValue(qty): %v", err)
	}
	if got != int(3) {
		t.Errorf("qty = %#v, want int(3) unchanged", got)
	}
}

// A nil colDef (e.g. the reserved $id column or an undeclared stored field) is
// passed through without coercion.
func TestAccessValue_NilColDefPassesThrough(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int64(3)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	got, err := AccessValue(row, rs, "orders", "o1", idColumnName, nil)
	if err != nil {
		t.Fatalf("AccessValue($id): %v", err)
	}
	if got != "o1" {
		t.Errorf("$id = %#v, want \"o1\"", got)
	}
}

// An unknown column name surfaces a wrapped, fail-loud error.
func TestAccessValue_UnknownColumnFailsLoud(t *testing.T) {
	t.Parallel()
	records := []KeyedStored{{Key: "o1", Stored: map[string]any{"qty": int64(3)}}}
	rs := BuildRecordset(ordersColDef(), records)
	row := rs.GetRow(0)
	_, err := AccessValue(row, rs, "orders", "o1", "nope", nil)
	if err == nil {
		t.Fatal("want error for unknown column")
	}
	if !strings.Contains(err.Error(), `column "nope"`) {
		t.Errorf("error %q should name the unknown column", err.Error())
	}
}
