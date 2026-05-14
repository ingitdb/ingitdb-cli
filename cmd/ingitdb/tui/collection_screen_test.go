package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func simpleColDef(id string, cols ...string) *ingitdb.CollectionDef {
	columns := make(map[string]*ingitdb.ColumnDef, len(cols))
	for _, c := range cols {
		columns[c] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	}
	return &ingitdb.CollectionDef{
		ID:           id,
		Columns:      columns,
		ColumnsOrder: cols,
	}
}

// makeRecords creates n records with a single "name" field.
func makeRecords(n int) []map[string]any {
	recs := make([]map[string]any, n)
	for i := range recs {
		recs[i] = map[string]any{"name": "item"}
	}
	return recs
}

// ---------------------------------------------------------------------------
// newCollectionModel
// ---------------------------------------------------------------------------

func TestNewCollectionModel_InitialState(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("users", "name", "email")
	m := newCollectionModel(colDef, nil, 120, 40)
	if !m.loading {
		t.Error("new model should be in loading state")
	}
	if m.colDef != colDef {
		t.Error("colDef not stored")
	}
	if len(m.columns) != 2 {
		t.Errorf("columns = %v, want [name email]", m.columns)
	}
	if !m.panels.IsFocused(1) {
		t.Errorf("initial focus should be panel 1 (data), got %d", m.panels.focus)
	}
}

// ---------------------------------------------------------------------------
// collectionPanelWidths
// ---------------------------------------------------------------------------

func TestCollectionPanelWidths_MinLeft(t *testing.T) {
	t.Parallel()
	left, right := collectionPanelWidths(30) // totalWidth / 4 = 7, below min 24
	if left != 24 {
		t.Errorf("left = %d, want 24 (minimum)", left)
	}
	if right != 30-24 {
		t.Errorf("right = %d, want %d", right, 30-24)
	}
}

func TestCollectionPanelWidths_Normal(t *testing.T) {
	t.Parallel()
	left, right := collectionPanelWidths(120)
	if left != 30 {
		t.Errorf("left = %d, want 30 (120/4)", left)
	}
	if left+right != 120 {
		t.Errorf("left+right = %d, want 120", left+right)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.panelInnerDims
// ---------------------------------------------------------------------------

func TestCollectionModel_PanelInnerDims(t *testing.T) {
	t.Parallel()
	m := newCollectionModel(simpleColDef("c", "x"), nil, 120, 40)
	w, h := m.panelInnerDims()
	left, _ := collectionPanelWidths(120)
	wantW := left - 4
	wantH := 40 - 2
	if w != wantW {
		t.Errorf("panelInnerDims width = %d, want %d", w, wantW)
	}
	if h != wantH {
		t.Errorf("panelInnerDims height = %d, want %d", h, wantH)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Update — window resize
// ---------------------------------------------------------------------------

func TestCollectionModel_Update_WindowResize(t *testing.T) {
	t.Parallel()
	m := newCollectionModel(simpleColDef("c", "x"), nil, 120, 40)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	if updated.width != 200 || updated.height != 60 {
		t.Errorf("after resize: width=%d height=%d, want 200 60", updated.width, updated.height)
	}
	if cmd != nil {
		t.Error("window resize should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Update — recordsLoadedMsg
// ---------------------------------------------------------------------------

func TestCollectionModel_Update_RecordsLoaded(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("users", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	records := []map[string]any{{"name": "Alice"}, {"name": "Bob"}}
	keys := []string{"1", "2"}
	updated, _ := m.Update(recordsLoadedMsg{records: records, keys: keys})
	if updated.loading {
		t.Error("after recordsLoadedMsg loading should be false")
	}
	if len(updated.records) != 2 {
		t.Errorf("records = %d, want 2", len(updated.records))
	}
	if updated.recordKeys[0] != "1" {
		t.Errorf("recordKeys[0] = %q, want 1", updated.recordKeys[0])
	}
}

func TestCollectionModel_Update_RecordsLoaded_WithL10N(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	m := newCollectionModel(colDef, nil, 120, 40)
	records := []map[string]any{
		{"title": map[string]any{"en": "Hello", "fr": "Bonjour"}},
	}
	updated, _ := m.Update(recordsLoadedMsg{records: records, keys: []string{"1"}})
	if updated.locale != "en" {
		t.Errorf("locale = %q, want en (preferred)", updated.locale)
	}
	if len(updated.locales) != 2 {
		t.Errorf("locales = %v, want 2 entries", updated.locales)
	}
}

func TestCollectionModel_Update_RecordsLoaded_NoEnLocale(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	m := newCollectionModel(colDef, nil, 120, 40)
	records := []map[string]any{
		{"title": map[string]any{"fr": "Bonjour", "de": "Hallo"}},
	}
	updated, _ := m.Update(recordsLoadedMsg{records: records, keys: []string{"1"}})
	// No "en" → first locale in sorted order selected (German < French by name: "German" < "French"... actually French < German alphabetically)
	if updated.locale == "" {
		t.Error("locale should be set to first sorted locale")
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Update — key navigation
// ---------------------------------------------------------------------------

func TestCollectionModel_Update_PanelNav(t *testing.T) {
	t.Parallel()
	m := newCollectionModel(simpleColDef("c", "x"), nil, 120, 40)
	// Initial focus is panel 1.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "alt+left"})
	if !updated.panels.IsFocused(0) {
		t.Errorf("after alt+left focus = %d, want 0", updated.panels.focus)
	}
}

func TestCollectionModel_Update_CursorDown(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(5)
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.recordCursor != 1 {
		t.Errorf("after down recordCursor = %d, want 1", updated.recordCursor)
	}
}

func TestCollectionModel_Update_CursorDown_AtEnd(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(3)
	m.recordCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.recordCursor != 2 {
		t.Errorf("cursor should not go past last record; got %d", updated.recordCursor)
	}
}

func TestCollectionModel_Update_CursorUp(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(5)
	m.recordCursor = 3
	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})
	if updated.recordCursor != 2 {
		t.Errorf("after up recordCursor = %d, want 2", updated.recordCursor)
	}
}

func TestCollectionModel_Update_CursorUp_AtTop_NoLocales(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(3)
	m.recordCursor = 0
	// No locales → no dropdown opened.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})
	if updated.localeDropdownOpen {
		t.Error("locale dropdown should not open with no locales")
	}
	if updated.recordCursor != 0 {
		t.Errorf("recordCursor should stay 0, got %d", updated.recordCursor)
	}
}

func TestCollectionModel_Update_CursorUp_AtTop_WithLocales(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(3)
	m.recordCursor = 0
	m.locales = []string{"en", "fr"}
	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})
	if !updated.localeDropdownOpen {
		t.Error("locale dropdown should open when at top row with multiple locales")
	}
}

func TestCollectionModel_Update_LocaleDropdown_Open(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(3)
	m.locales = []string{"en", "fr", "de"}
	m.locale = "en"
	// Open dropdown with "l".
	updated, _ := m.Update(tea.KeyPressMsg{Text: "l"})
	if !updated.localeDropdownOpen {
		t.Error("'l' should open locale dropdown when multiple locales available")
	}
}

func TestCollectionModel_Update_LocaleDropdown_Open_Capital_L(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.locales = []string{"en", "fr"}
	m.locale = "en"
	updated, _ := m.Update(tea.KeyPressMsg{Text: "L"})
	if !updated.localeDropdownOpen {
		t.Error("'L' should also open locale dropdown")
	}
}

func TestCollectionModel_Update_LocaleDropdown_NoOpen_SingleLocale(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.locales = []string{"en"} // only one locale
	m.locale = "en"
	updated, _ := m.Update(tea.KeyPressMsg{Text: "l"})
	if updated.localeDropdownOpen {
		t.Error("'l' should NOT open dropdown with only one locale")
	}
}

func TestCollectionModel_Update_LocaleDropdown_Esc(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.localeDropdownOpen = true
	updated, _ := m.Update(tea.KeyPressMsg{Text: "esc"})
	if updated.localeDropdownOpen {
		t.Error("esc should close locale dropdown")
	}
}

func TestCollectionModel_Update_LocaleDropdown_NavigateDown(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.localeDropdownOpen = true
	m.locales = []string{"en", "fr", "de"}
	m.localeDropdownCursor = 0
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.localeDropdownCursor != 1 {
		t.Errorf("dropdown cursor = %d, want 1", updated.localeDropdownCursor)
	}
}

func TestCollectionModel_Update_LocaleDropdown_NavigateUp(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.localeDropdownOpen = true
	m.locales = []string{"en", "fr", "de"}
	m.localeDropdownCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})
	if updated.localeDropdownCursor != 1 {
		t.Errorf("dropdown cursor = %d, want 1", updated.localeDropdownCursor)
	}
}

func TestCollectionModel_Update_LocaleDropdown_NavigateAtBounds(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.localeDropdownOpen = true
	m.locales = []string{"en", "fr"}
	m.localeDropdownCursor = 0
	// Up at top → no change.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})
	if updated.localeDropdownCursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0", updated.localeDropdownCursor)
	}
	// Down at bottom.
	m.localeDropdownCursor = 1
	updated, _ = m.Update(tea.KeyPressMsg{Text: "down"})
	if updated.localeDropdownCursor != 1 {
		t.Errorf("down at bottom: cursor = %d, want 1", updated.localeDropdownCursor)
	}
}

func TestCollectionModel_Update_LocaleDropdown_Enter(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
			"name":  {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name", "title"},
	}
	m := newCollectionModel(colDef, nil, 120, 40)
	m.localeDropdownOpen = true
	m.locales = []string{"en", "fr"}
	m.locale = "en"
	m.localeDropdownCursor = 1 // select "fr"
	m.columns = buildDisplayColumns(colDef, "en")
	updated, _ := m.Update(tea.KeyPressMsg{Text: "enter"})
	if updated.localeDropdownOpen {
		t.Error("enter should close dropdown")
	}
	if updated.locale != "fr" {
		t.Errorf("locale = %q, want fr", updated.locale)
	}
}

func TestCollectionModel_Update_ColRight(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "a", "b", "c")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.columns = orderedColumns(colDef)
	m.colWidths = []int{10, 10, 10}
	m.colCursor = 0
	updated, _ := m.Update(tea.KeyPressMsg{Text: "right"})
	if updated.colCursor != 1 {
		t.Errorf("colCursor = %d, want 1", updated.colCursor)
	}
}

func TestCollectionModel_Update_ColRight_AtEnd(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "a", "b", "c")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.columns = orderedColumns(colDef)
	m.colWidths = []int{10, 10, 10}
	m.colCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Text: "right"})
	if updated.colCursor != 2 {
		t.Errorf("colCursor at end = %d, want 2", updated.colCursor)
	}
}

func TestCollectionModel_Update_ColLeft(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "a", "b", "c")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.columns = orderedColumns(colDef)
	m.colWidths = []int{10, 10, 10}
	m.colCursor = 2
	updated, _ := m.Update(tea.KeyPressMsg{Text: "left"})
	if updated.colCursor != 1 {
		t.Errorf("colCursor = %d, want 1", updated.colCursor)
	}
}

func TestCollectionModel_Update_ColLeft_AtStart(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "a", "b")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(1)
	m.columns = orderedColumns(colDef)
	m.colWidths = []int{10, 10}
	m.colCursor = 0
	updated, _ := m.Update(tea.KeyPressMsg{Text: "left"})
	if updated.colCursor != 0 {
		t.Errorf("colCursor at start = %d, want 0", updated.colCursor)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.View
// ---------------------------------------------------------------------------

func TestCollectionModel_View_Loading(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("users", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	// loading = true by default
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("View() while loading should contain 'Loading'; got:\n%s", view)
	}
}

func TestCollectionModel_View_WithRecords(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("users", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = []map[string]any{{"name": "Alice"}}
	m.columns = orderedColumns(colDef)
	m.colWidths = m.computeColWidths()
	view := m.View()
	if !strings.Contains(view, "name") {
		t.Errorf("View() should contain column header 'name'; got:\n%s", view)
	}
}

func TestCollectionModel_View_Help(t *testing.T) {
	t.Parallel()
	m := newCollectionModel(simpleColDef("c", "x"), nil, 120, 40)
	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Errorf("View() should contain help text with 'quit'; got first 200 chars:\n%s", view[:min(200, len(view))])
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Init
// ---------------------------------------------------------------------------

func TestCollectionModel_Init_WithNilDB(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("c", "x")
	m := newCollectionModel(colDef, nil, 120, 40)
	// Init returns a tea.Cmd. With nil db, it will return a function.
	// We just verify Init() doesn't panic.
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
