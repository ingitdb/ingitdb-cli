package tui

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// colDefWithOrder builds a CollectionDef whose Columns map contains exactly
// the keys in order, with all types set to "string".
func colDefWithOrder(id string, colsInOrder []string) *ingitdb.CollectionDef {
	cols := make(map[string]*ingitdb.ColumnDef, len(colsInOrder))
	for _, c := range colsInOrder {
		cols[c] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	}
	return &ingitdb.CollectionDef{
		ID:           id,
		Columns:      cols,
		ColumnsOrder: colsInOrder,
	}
}

// headerLine extracts the column-header line (line index 2) from renderRecords output.
// Line 0: title/locale, line 1: underline, line 2: column headers.
func headerLine(rendered string) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) < 3 {
		return ""
	}
	return stripAnsi(lines[2])
}

// TestRenderRecords_ColumnsOrderRespected verifies that renderRecords displays
// columns in the order defined by ColumnsOrder, not alphabetically.
func TestRenderRecords_ColumnsOrderRespected(t *testing.T) {
	t.Parallel()

	// Define columns_order that is NOT alphabetical.
	colsOrder := []string{"zeta", "alpha", "mu", "beta"}
	colDef := colDefWithOrder("things", colsOrder)

	m := collectionModel{
		colDef:  colDef,
		columns: orderedColumns(colDef),
		records: []map[string]any{
			{"zeta": "z1", "alpha": "a1", "mu": "m1", "beta": "b1"},
		},
	}

	rendered := m.renderRecords(120, 20)
	hdr := headerLine(rendered)

	// Verify every expected column appears.
	for _, col := range colsOrder {
		if !strings.Contains(hdr, col) {
			t.Errorf("column %q not found in header %q; rendered:\n%s", col, hdr, rendered)
		}
	}

	// Verify relative positions match columns_order.
	for i := 0; i < len(colsOrder)-1; i++ {
		a, b := colsOrder[i], colsOrder[i+1]
		pa := strings.Index(hdr, a)
		pb := strings.Index(hdr, b)
		if pa < 0 || pb < 0 {
			continue
		}
		if pa >= pb {
			t.Errorf("column %q (pos %d) should appear before %q (pos %d) per columns_order; header: %q",
				a, pa, b, pb, hdr)
		}
	}
}

// TestRenderRecords_ColumnsOrderWithL10N verifies that columns_order is respected
// even when L10N columns are expanded (e.g. "title" → "title.en").
func TestRenderRecords_ColumnsOrderWithL10N(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID: "countries",
		Columns: map[string]*ingitdb.ColumnDef{
			"flag":       {Type: ingitdb.ColumnTypeString},
			"title":      {Type: ingitdb.ColumnTypeL10N},
			"population": {Type: ingitdb.ColumnTypeFloat},
		},
		ColumnsOrder: []string{"flag", "title", "population"},
	}

	locale := "en"
	m := collectionModel{
		colDef:  colDef,
		locale:  locale,
		columns: buildDisplayColumns(colDef, locale),
		records: []map[string]any{
			{
				"flag":       "US",
				"title":      map[string]any{"en": "United States"},
				"population": 331000000,
			},
		},
	}

	rendered := m.renderRecords(120, 20)
	hdr := headerLine(rendered)

	wantOrder := []string{"flag", "title.en", "population"}
	for _, col := range wantOrder {
		if !strings.Contains(hdr, col) {
			t.Errorf("column %q not found in header %q; rendered:\n%s", col, hdr, rendered)
		}
	}

	// flag must come before title.en, title.en before population.
	for i := 0; i < len(wantOrder)-1; i++ {
		a, b := wantOrder[i], wantOrder[i+1]
		pa := strings.Index(hdr, a)
		pb := strings.Index(hdr, b)
		if pa < 0 || pb < 0 {
			continue
		}
		if pa >= pb {
			t.Errorf("column %q (pos %d) should appear before %q (pos %d) per columns_order; header: %q",
				a, pa, b, pb, hdr)
		}
	}
}
