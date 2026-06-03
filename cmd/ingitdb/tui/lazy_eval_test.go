package tui

// lazy_eval_test.go drives the collection model with a counting recordset
// evaluator (as in pkg/dalgo2ingitdb's recordset tests) to prove that computed
// columns are evaluated only for painted cells — never for off-viewport rows —
// and that per-row memoization survives re-renders and scroll-back.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// countingTUIEvaluator records how many times a computed column is evaluated.
type countingTUIEvaluator struct{ calls *int }

func (e countingTUIEvaluator) Eval(_ map[string]any) (any, error) {
	*e.calls++
	return int64(99), nil
}

// erroringTUIEvaluator always fails, so a painted computed cell surfaces an error.
type erroringTUIEvaluator struct{}

func (erroringTUIEvaluator) Eval(_ map[string]any) (any, error) {
	return nil, fmt.Errorf("boom")
}

// qtyZeroErrorEvaluator counts calls and fails only when the row's stored qty is
// zero — modelling a formula like "100 / qty" that raises on some rows but not
// others, so the error is row-positioned.
type qtyZeroErrorEvaluator struct{ calls *int }

func (e qtyZeroErrorEvaluator) Eval(stored map[string]any) (any, error) {
	*e.calls++
	if stored["qty"] == int64(0) {
		return nil, fmt.Errorf("division by zero")
	}
	return int64(99), nil
}

// lazyTestColDef is a collection with one stored column (qty) and one computed
// column (ratio).
func lazyModelColDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"qty":   {Type: ingitdb.ColumnTypeInt},
			"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty * 2"},
		},
		ColumnsOrder: []string{"qty", "ratio"},
	}
}

// buildModelWithQtys builds a collection model over a recordset with one stored
// "qty" column (one row per supplied value) and a computed "ratio" column bound
// to eval. Row handles are retained on the model exactly as loadRecordsCmd
// retains them, so the tests exercise real per-row memoization.
func buildModelWithQtys(qtys []int64, eval recordset.Evaluator) collectionModel {
	colDef := lazyModelColDef()
	rs := recordset.NewColumnarRecordset(colDef.ID,
		recordset.NewColumn[int64]("qty", 0),
		recordset.NewComputedColumn("ratio", eval),
	)

	rows := make([]recordset.Row, len(qtys))
	records := make([]map[string]any, len(qtys))
	keys := make([]string, len(qtys))
	for i, q := range qtys {
		row := rs.NewRow()
		_ = row.SetValueByName("qty", q, rs)
		rows[i] = row
		records[i] = map[string]any{"qty": q}
		keys[i] = fmt.Sprintf("r%d", i)
	}

	m := collectionModel{
		colDef:     colDef,
		columns:    []string{"qty", "ratio"},
		rs:         rs,
		rows:       rows,
		records:    records,
		recordKeys: keys,
	}
	m.colWidths = m.computeColWidths()
	return m
}

// newLazyModel builds a model of `total` records (qty = 1..total) whose computed
// column is bound to the counting evaluator.
func newLazyModel(total int, calls *int) collectionModel {
	qtys := make([]int64, total)
	for i := range qtys {
		qtys[i] = int64(i + 1)
	}
	return buildModelWithQtys(qtys, countingTUIEvaluator{calls})
}

// AC: off-viewport-rows-not-evaluated — rendering without scrolling evaluates
// the computed column only for the V visible records, never for off-viewport
// records; and load + width sizing evaluate nothing.
func TestLazy_OffViewportRowsNotEvaluated(t *testing.T) {
	t.Parallel()
	calls := 0
	const total, height = 10, 7 // renderRecords: visibleRows = height-4 = 3
	const wantVisible = 3
	m := newLazyModel(total, &calls)

	if calls != 0 {
		t.Fatalf("load and width sizing evaluated %d computed cells, want 0", calls)
	}

	_ = m.renderRecords(80, height)

	if calls != wantVisible {
		t.Errorf("evaluator invoked %d times, want %d (one per visible record, zero for the %d off-viewport records)",
			calls, wantVisible, total-wantVisible)
	}
}

// AC: scroll-evaluates-only-newly-visible — after scrolling so a previously
// off-viewport record becomes visible, only that newly-visible record is
// evaluated; repainted records are not evaluated a second time (retained row
// handles preserve dalgo's per-row memoization).
func TestLazy_ScrollEvaluatesOnlyNewlyVisible(t *testing.T) {
	t.Parallel()
	calls := 0
	const total, height = 10, 7 // V = 3
	m := newLazyModel(total, &calls)

	_ = m.renderRecords(80, height) // paints records 0,1,2
	if calls != 3 {
		t.Fatalf("first render evaluated %d computed cells, want 3", calls)
	}

	// Scroll down by one row: records 1,2,3 are now visible. Record 3 is newly
	// visible; records 1 and 2 are repainted from their retained, memoized row
	// handles and must not be evaluated again.
	m.recordOffset = 1
	_ = m.renderRecords(80, height)

	if calls != 4 {
		t.Errorf("after scroll evaluator invoked %d times total, want 4 (only newly-visible record 3 evaluated; repainted records 1,2 memoized)", calls)
	}
}

// cellValueAt resolves a computed cell through AccessValue and propagates an
// evaluation error to its caller (the render path decides how to surface it).
func TestCellValueAt_ComputedErrorPropagated(t *testing.T) {
	t.Parallel()
	colDef := lazyModelColDef()
	rs := recordset.NewColumnarRecordset(colDef.ID,
		recordset.NewColumn[int64]("qty", 0),
		recordset.NewComputedColumn("ratio", erroringTUIEvaluator{}),
	)
	row := rs.NewRow()
	_ = row.SetValueByName("qty", int64(1), rs)
	m := collectionModel{
		colDef:     colDef,
		columns:    []string{"qty", "ratio"},
		rs:         rs,
		rows:       []recordset.Row{row},
		records:    []map[string]any{{"qty": int64(1)}},
		recordKeys: []string{"r0"},
	}

	got, err := m.cellValueAt(0, "ratio")
	if err == nil {
		t.Fatalf("cellValueAt(ratio) error = nil, want an error; value=%q", got)
	}
	if got != "" {
		t.Errorf("cellValueAt(ratio) value = %q on error, want empty", got)
	}

	// A stored column on the same row resolves without error.
	if v, err := m.cellValueAt(0, "qty"); err != nil || v != "1" {
		t.Errorf("cellValueAt(qty) = (%q, %v), want (\"1\", nil)", v, err)
	}
}

// AC: width-sizing-does-not-evaluate — computeColWidths never invokes the
// evaluator, and a computed column's width derives from its header label.
func TestLazy_WidthSizingDoesNotEvaluate(t *testing.T) {
	t.Parallel()
	calls := 0
	m := newLazyModel(5, &calls)

	widths := m.computeColWidths()

	if calls != 0 {
		t.Fatalf("computeColWidths invoked the evaluator %d times, want 0", calls)
	}
	// columns: [qty, ratio]; ratio has no declared length, so its width is its
	// header label width.
	if want := len("ratio"); widths[1] != want {
		t.Errorf("computed column width = %d, want %d (header-derived, no value sampling)", widths[1], want)
	}
}

// AC: width-sizing-does-not-evaluate (declared-length path) — a computed
// column's width honors its declared length without sampling any value.
func TestLazy_ComputedWidthFromDeclaredLength(t *testing.T) {
	t.Parallel()
	calls := 0
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"x":     {Type: ingitdb.ColumnTypeString},
			"label": {Type: ingitdb.ColumnTypeString, Formula: "x", Length: 12},
		},
		ColumnsOrder: []string{"x", "label"},
	}
	rs := recordset.NewColumnarRecordset(colDef.ID,
		recordset.NewColumn[string]("x", ""),
		recordset.NewComputedColumn("label", countingTUIEvaluator{&calls}),
	)
	row := rs.NewRow()
	_ = row.SetValueByName("x", "hi", rs)
	m := collectionModel{
		colDef:     colDef,
		columns:    []string{"x", "label"},
		rs:         rs,
		rows:       []recordset.Row{row},
		records:    []map[string]any{{"x": "hi"}},
		recordKeys: []string{"r0"},
	}

	widths := m.computeColWidths()

	if calls != 0 {
		t.Fatalf("computeColWidths invoked the evaluator %d times, want 0", calls)
	}
	if widths[1] != 12 {
		t.Errorf("computed column width = %d, want 12 (declared length)", widths[1])
	}
}

// AC: numeric-alignment-does-not-evaluate — an int computed column is
// right-aligned (numeric) from its declared type, with no evaluator call.
func TestLazy_NumericAlignmentFromTypeDoesNotEvaluate(t *testing.T) {
	t.Parallel()
	calls := 0
	m := newLazyModel(5, &calls)

	numeric := m.numericColumns(m.columns)

	if calls != 0 {
		t.Fatalf("alignment determination invoked the evaluator %d times, want 0", calls)
	}
	// columns: [qty (stored int), ratio (computed int)] — both numeric.
	if !numeric[1] {
		t.Error("int computed column should be right-aligned (numeric) from its declared type")
	}
}

// A non-numeric computed column is not right-aligned, decided from its declared
// type without evaluation.
func TestNumericColumns_ComputedStringNotNumeric(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"x":    {Type: ingitdb.ColumnTypeString},
			"name": {Type: ingitdb.ColumnTypeString, Formula: "x"},
		},
		ColumnsOrder: []string{"x", "name"},
	}
	m := collectionModel{
		colDef:  colDef,
		columns: []string{"x", "name"},
		records: []map[string]any{{"x": "hi"}},
	}

	numeric := m.numericColumns(m.columns)

	if numeric[1] {
		t.Error("string computed column must not be right-aligned")
	}
}

// AC: stored-locale-discovery-unchanged — locale keys present only on an
// off-viewport record still appear; locale discovery scans every record's
// stored values, exactly as before this Feature.
func TestLazy_StoredLocaleDiscoveryScansAllRecords(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	// "fr" appears only on the second record — the one that would sit outside a
	// short visible window.
	records := []map[string]any{
		{"title": map[string]any{"en": "Hello"}},
		{"title": map[string]any{"en": "Hi", "fr": "Bonjour"}},
	}

	got := discoverLocales(records, colDef)

	if len(got) != 2 || got[0] != "en" || got[1] != "fr" {
		t.Errorf("discoverLocales = %v, want [en fr] (off-viewport locale must be discovered)", got)
	}
}

// AC: visible-computed-error-non-fatal — a painted computed cell whose
// evaluation fails renders a bounded error indicator, and the rest of the screen
// keeps rendering (no crash, no aborted load).
func TestLazy_VisibleComputedErrorNonFatal(t *testing.T) {
	t.Parallel()
	calls := 0
	// Record 0's computed cell errors (qty=0); records 1 and 2 compute fine. All
	// three are within the visible window (visibleRows = height-4 = 3).
	m := buildModelWithQtys([]int64{0, 2, 3}, qtyZeroErrorEvaluator{&calls})

	out := m.renderRecords(80, 7)

	if !strings.Contains(out, computedCellError) {
		t.Errorf("expected bounded error indicator %q in render output, got:\n%s", computedCellError, out)
	}
	// The screen keeps rendering its other cells — the non-erroring computed
	// values (99) and the stored qty values are all present.
	if !strings.Contains(out, "99") {
		t.Error("screen should keep rendering the non-erroring rows around the erroring cell")
	}
}

// AC: off-viewport-error-never-evaluated — an erroring computed column on
// off-viewport records is never evaluated, so the screen renders normally and no
// error surfaces.
func TestLazy_OffViewportErrorNeverEvaluated(t *testing.T) {
	t.Parallel()
	calls := 0
	// Visible window (3 rows) shows records 0..2 with qty 1,2,3 (compute fine).
	// Records 3..5 have qty=0 and would error — but they are off-viewport.
	m := buildModelWithQtys([]int64{1, 2, 3, 0, 0, 0}, qtyZeroErrorEvaluator{&calls})

	out := m.renderRecords(80, 7)

	if strings.Contains(out, computedCellError) {
		t.Errorf("off-viewport erroring rows must not surface an error indicator; output:\n%s", out)
	}
	if calls != 3 {
		t.Errorf("evaluator invoked %d times, want 3 (only the visible non-erroring rows; off-viewport erroring rows never evaluated)", calls)
	}
}
