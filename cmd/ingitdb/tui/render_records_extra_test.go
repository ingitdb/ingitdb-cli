package tui

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// TestRenderRecords_NoRecords covers the empty-records branch.
func TestRenderRecords_NoRecords(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "name")
	m := collectionModel{
		colDef:  colDef,
		columns: orderedColumns(colDef),
		records: nil,
		loading: false,
	}
	rendered := m.renderRecords(80, 20)
	if !strings.Contains(rendered, "no records") {
		t.Errorf("renderRecords empty should say 'no records'; got %q", rendered)
	}
}

// TestRenderRecords_NoColumns covers the empty-columns branch.
func TestRenderRecords_NoColumns(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		Columns: map[string]*ingitdb.ColumnDef{},
	}
	m := collectionModel{
		colDef:  colDef,
		columns: []string{},
		records: []map[string]any{{"x": "y"}},
		loading: false,
	}
	rendered := m.renderRecords(80, 20)
	if !strings.Contains(rendered, "no columns") {
		t.Errorf("renderRecords no columns should say 'no columns'; got %q", rendered)
	}
}

// TestRenderRecords_ColOffsetClamped covers colOffset >= len(cols) branch.
func TestRenderRecords_ColOffsetClamped(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "name")
	m := collectionModel{
		colDef:    colDef,
		columns:   orderedColumns(colDef),
		records:   []map[string]any{{"name": "Alice"}},
		colOffset: 999, // way past end — should be clamped to 0
	}
	m.colWidths = m.computeColWidths()
	rendered := m.renderRecords(80, 20)
	if !strings.Contains(stripAnsi(rendered), "name") {
		t.Errorf("renderRecords clamped offset should still show header; got %q", rendered)
	}
}

// TestRenderRecords_HorizontalScrollIndicators covers colOffset>0 and trailing arrow.
func TestRenderRecords_HorizontalScrollIndicators(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "a", "b", "c", "d", "e")
	m := collectionModel{
		colDef:    colDef,
		columns:   orderedColumns(colDef),
		records:   []map[string]any{{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}},
		colOffset: 1, // scroll past first column → left indicator
	}
	m.colWidths = []int{20, 20, 20, 20, 20}
	// narrow panel so not all columns fit → right indicator too
	rendered := m.renderRecords(30, 20)
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "◀") {
		t.Errorf("renderRecords with offset>0 should show ◀ indicator; got %q", plain)
	}
}

// TestRenderRecords_TruncatesLongValue covers the cell value truncation branch.
func TestRenderRecords_TruncatesLongValue(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "desc")
	m := collectionModel{
		colDef:  colDef,
		columns: orderedColumns(colDef),
		records: []map[string]any{{"desc": strings.Repeat("x", 50)}},
	}
	m.colWidths = []int{10} // force truncation
	rendered := m.renderRecords(80, 20)
	if !strings.Contains(rendered, "…") {
		t.Errorf("renderRecords should truncate long values with ellipsis; got %q", rendered)
	}
}

// TestRenderRecords_LocaleTitleAndDropdownOverlay covers locale label in title
// and dropdown overlay rendering.
func TestRenderRecords_LocaleTitleAndDropdownOverlay(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	m := collectionModel{
		colDef:               colDef,
		columns:              buildDisplayColumns(colDef, "en"),
		locale:               "en",
		locales:              []string{"en", "fr"},
		localeDropdownOpen:   true,
		localeDropdownCursor: 0,
		records: []map[string]any{
			{"title": map[string]any{"en": "Hello", "fr": "Bonjour"}},
		},
	}
	m.colWidths = m.computeColWidths()
	rendered := m.renderRecords(80, 20)
	plain := stripAnsi(rendered)
	// Title row should show locale indicator.
	if !strings.Contains(plain, "en") {
		t.Errorf("renderRecords locale label: should contain 'en'; got %q", plain)
	}
	// Dropdown overlay should appear (border characters).
	if !strings.Contains(plain, "┌") && !strings.Contains(plain, "English") {
		t.Errorf("renderRecords with dropdown open should show dropdown; got %q", plain)
	}
}

// TestRenderRecords_SmallHeight covers visibleRows < 1 clamp.
func TestRenderRecords_SmallHeight(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "name")
	m := collectionModel{
		colDef:  colDef,
		columns: orderedColumns(colDef),
		records: []map[string]any{{"name": "Alice"}},
	}
	m.colWidths = m.computeColWidths()
	// height=3 means visibleRows = 3-4 = -1 which clamps to 1
	rendered := m.renderRecords(80, 3)
	if rendered == "" {
		t.Error("renderRecords with very small height should still return content")
	}
}

// TestRenderRecords_ZeroBorderWidth covers borderW < 1 clamp.
func TestRenderRecords_ZeroBorderWidth(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "name")
	m := collectionModel{
		colDef:  colDef,
		columns: orderedColumns(colDef),
		records: []map[string]any{{"name": "Alice"}},
	}
	m.colWidths = m.computeColWidths()
	// width=0 → borderW clamps to 1
	rendered := m.renderRecords(0, 20)
	if rendered == "" {
		t.Error("renderRecords with zero width should still return content")
	}
}

// TestRenderRecords_LocaleArrowClosed covers upward arrow when dropdown closed.
func TestRenderRecords_LocaleArrowClosed(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	m := collectionModel{
		colDef:             colDef,
		columns:            buildDisplayColumns(colDef, "en"),
		locale:             "en",
		locales:            []string{"en", "fr"},
		localeDropdownOpen: false,
		records: []map[string]any{
			{"title": map[string]any{"en": "Hello"}},
		},
	}
	m.colWidths = m.computeColWidths()
	rendered := m.renderRecords(80, 20)
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "▼") {
		t.Errorf("renderRecords closed dropdown should show ▼; got %q", plain)
	}
}

// TestRenderRecords_GapClampedToOne covers the locale gap < 1 branch.
func TestRenderRecords_GapClampedToOne(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	m := collectionModel{
		colDef:  colDef,
		columns: buildDisplayColumns(colDef, "en"),
		locale:  "en",
		locales: []string{"en", "fr"},
		records: []map[string]any{
			{"title": map[string]any{"en": "Hello"}},
		},
	}
	m.colWidths = m.computeColWidths()
	// Very narrow width forces gap < 1.
	rendered := m.renderRecords(5, 20)
	if rendered == "" {
		t.Error("renderRecords with very narrow width should still return content")
	}
}

// TestRenderSchema_StartClamped covers start > len(physical) branch.
func TestRenderSchema_StartClamped(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("things", "name")
	m := collectionModel{
		colDef:       colDef,
		schemaLines:  buildSchemaLines(colDef),
		schemaOffset: 9999, // way past end
	}
	rendered := m.renderSchema(40, 10)
	// Should return 10 empty lines rather than panicking.
	lines := strings.Split(rendered, "\n")
	if len(lines) != 10 {
		t.Errorf("renderSchema clamped start: got %d lines, want 10", len(lines))
	}
}

// TestHomeModel_View_PreviewWithData covers the middle+right panel paths in homeModel.View.
func TestHomeModel_View_PreviewWithData(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	colDef := def.Collections["users"]
	m.cursor = 0
	m.preview = &collectionModel{
		colDef:      colDef,
		schemaLines: buildSchemaLines(colDef),
		columns:     []string{"name"},
		records:     []map[string]any{{"name": "Alice"}},
		loading:     false,
	}
	m.preview.colWidths = m.preview.computeColWidths()
	rendered := m.View()
	if !strings.Contains(rendered, "users") {
		t.Errorf("View() with preview data should contain collection id; got first 200:\n%s", rendered[:min(200, len(rendered))])
	}
}

// TestHomeModel_Update_FilterCursorTruncate covers cursor > filteredCollections after filter.
func TestHomeModel_Update_FilterCursorTruncate(t *testing.T) {
	t.Parallel()
	def := defWithCollections("xfoo", "bar")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 2 // past all 2 collections
	// Type "x" → only xfoo remains (len=1); cursor must be clamped to 1 (add-button)
	updated, _ := m.Update(kp("x"))
	if updated.cursor > len(updated.filteredCollections) {
		t.Errorf("cursor=%d exceeds len(filteredCollections)=%d after filter", updated.cursor, len(updated.filteredCollections))
	}
}

// TestHomeModel_Update_Backspace_CursorTruncate covers cursor > filteredCollections after backspace.
func TestHomeModel_Update_Backspace_CursorTruncate(t *testing.T) {
	t.Parallel()
	def := defWithCollections("xfoo", "bar")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "xf"
	m.filteredCollections = m.applyFilter() // [xfoo]
	m.cursor = 1                            // add-button in filtered view
	// Backspace removes "f" → filter="x", still only xfoo; cursor stays valid.
	updated, _ := m.Update(kp("backspace"))
	if updated.cursor > len(updated.filteredCollections) {
		t.Errorf("cursor=%d exceeds len(filteredCollections)=%d after backspace", updated.cursor, len(updated.filteredCollections))
	}
}

// TestHomeModel_Init_PreviewBuilt covers the m.preview != nil branch of Init.
func TestHomeModel_Init_PreviewBuilt(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	// Manually inject a preview with a colDef so Init() returns its Init().
	col := collectionModel{
		colDef:  def.Collections["users"],
		columns: []string{"name"},
		loading: true,
	}
	m.preview = &col
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() with non-nil preview should return non-nil cmd")
	}
}

// TestHomeModel_RefreshPreview_NewCollection covers building a new preview.
func TestHomeModel_RefreshPreview_NewCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	m.previewID = "" // force rebuild
	m.preview = nil
	// With nil newDB the build will exit early.
	cmd := m.refreshPreview()
	_ = cmd // may be nil; just assert no panic
}

// TestRenderCollectionList_ScrollOffset covers start > 0 branch in renderCollectionList.
func TestRenderCollectionList_ScrollOffset(t *testing.T) {
	t.Parallel()
	// Create enough collections to force scrolling (maxVisible = height-4).
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = "col" + string(rune('a'+i))
	}
	def := defWithCollections(ids...)
	m := newHomeModel("/repo", def, nil, 120, 30)
	// Set cursor to a high value to force scroll offset.
	m.cursor = 15
	rendered := m.renderCollectionList(30, 10)
	if rendered == "" {
		t.Error("renderCollectionList with scroll offset should return content")
	}
}

// TestRenderCollectionList_RelPathError covers the filepath.Rel error branch
// (simulated by injecting a mismatched entry whose dirPath produces a relative path).
func TestRenderCollectionList_RelPath_Fallback(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {ID: "users", DirPath: "/other/path/users"},
		},
	}
	m := newHomeModel("/repo", def, nil, 120, 30)
	rendered := m.renderCollectionList(60, 20)
	// On Unix filepath.Rel succeeds and returns a relative path; no panic.
	if rendered == "" {
		t.Error("renderCollectionList should return content even with cross-tree paths")
	}
}
