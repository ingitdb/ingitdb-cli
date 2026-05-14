package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defWithCollections(ids ...string) *ingitdb.Definition {
	cols := make(map[string]*ingitdb.CollectionDef, len(ids))
	for _, id := range ids {
		cols[id] = &ingitdb.CollectionDef{
			ID:      id,
			DirPath: "/repo/" + id,
			Columns: map[string]*ingitdb.ColumnDef{
				"name": {Type: ingitdb.ColumnTypeString},
			},
			ColumnsOrder: []string{"name"},
		}
	}
	return &ingitdb.Definition{Collections: cols}
}

func kp(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: text}
}

// ---------------------------------------------------------------------------
// homeModel.Init
// ---------------------------------------------------------------------------

func TestHomeModel_Init_NoPreview(t *testing.T) {
	t.Parallel()
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{}}
	m := newHomeModel("/repo", def, nil, 120, 40)
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() with no preview should return nil")
	}
}

func TestHomeModel_Init_WithPreview(t *testing.T) {
	t.Parallel()
	// newDB always fails → preview will not be built (buildPreview exits on db error)
	def := defWithCollections("users")
	newDB := func(dbPath string, d *ingitdb.Definition) (dal.DB, error) {
		return nil, nil // returns nil db, no error
	}
	m := newHomeModel("/repo", def, newDB, 120, 40)
	// preview should have been built (newDB returned nil db without error)
	// Init() returns the preview's Init() cmd, which calls loadRecordsCmd with nil db.
	cmd := m.Init()
	// cmd may be non-nil (a function wrapping loadRecordsCmd) or nil depending on preview build.
	// We just assert no panic.
	_ = cmd
}

// ---------------------------------------------------------------------------
// homeModel.applyFilter
// ---------------------------------------------------------------------------

func TestApplyFilter_Empty(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta", "gamma")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = ""
	got := m.applyFilter()
	if len(got) != 3 {
		t.Errorf("applyFilter empty = %d, want 3", len(got))
	}
}

func TestApplyFilter_Match(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta", "gamma")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "al"
	got := m.applyFilter()
	if len(got) != 1 || got[0].id != "alpha" {
		t.Errorf("applyFilter 'al' = %v, want [alpha]", got)
	}
}

func TestApplyFilter_NoMatch(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "zzz"
	got := m.applyFilter()
	if len(got) != 0 {
		t.Errorf("applyFilter 'zzz' = %v, want empty", got)
	}
}

func TestApplyFilter_CaseInsensitive(t *testing.T) {
	t.Parallel()
	def := defWithCollections("Users", "products")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "users"
	got := m.applyFilter()
	if len(got) != 1 {
		t.Errorf("applyFilter case-insensitive = %v, want 1 match", got)
	}
}

// ---------------------------------------------------------------------------
// homeModel.SelectedCollection
// ---------------------------------------------------------------------------

func TestSelectedCollection_OnCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users", "orders")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	got := m.SelectedCollection()
	if got == nil {
		t.Fatal("SelectedCollection() = nil, want a CollectionDef")
	}
}

func TestSelectedCollection_OnAddButton(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = len(m.filteredCollections) // add-button row
	got := m.SelectedCollection()
	if got != nil {
		t.Errorf("SelectedCollection() on add-button = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// homeModel.buildPreview
// ---------------------------------------------------------------------------

func TestBuildPreview_OutOfBounds(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.buildPreview(99) // index out of bounds → no-op
	if m.preview != nil {
		t.Error("buildPreview out of bounds should leave preview nil")
	}
}

func TestBuildPreview_NilNewDB(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40) // newDB = nil
	m.buildPreview(0)
	if m.preview != nil {
		t.Error("buildPreview with nil newDB should leave preview nil")
	}
}

func TestBuildPreview_NilColDef(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	// Inject a filteredCollections entry whose id is not in def.Collections.
	m.filteredCollections = []collectionEntry{{id: "ghost", dirPath: "/repo/ghost"}}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }
	m.newDB = newDB
	m.buildPreview(0)
	if m.preview != nil {
		t.Error("buildPreview with missing colDef should leave preview nil")
	}
}

// ---------------------------------------------------------------------------
// homeModel.refreshPreview
// ---------------------------------------------------------------------------

func TestRefreshPreview_CursorOnAddButton(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = len(m.filteredCollections) // add-button
	cmd := m.refreshPreview()
	if cmd != nil {
		t.Error("refreshPreview on add-button cursor should return nil cmd")
	}
	if m.preview != nil {
		t.Error("refreshPreview on add-button should clear preview")
	}
}

func TestRefreshPreview_SameCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	// Manually set previewID to match current cursor entry.
	if len(m.filteredCollections) > 0 {
		m.previewID = m.filteredCollections[0].id
		// Place a non-nil preview sentinel.
		col := collectionModel{}
		m.preview = &col
	}
	cmd := m.refreshPreview()
	if cmd != nil {
		t.Error("refreshPreview for already-loaded collection should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// homeModel.Update — key navigation
// ---------------------------------------------------------------------------

func TestHomeModel_Update_Down_CollectionPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta", "gamma")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	updated, _ := m.Update(kp("down"))
	if updated.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", updated.cursor)
	}
}

func TestHomeModel_Update_Down_AtEnd(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = len(m.filteredCollections) // already on add-button
	updated, _ := m.Update(kp("down"))
	// cursor should not exceed len(filteredCollections)
	if updated.cursor > len(updated.filteredCollections) {
		t.Errorf("cursor past add-button: %d", updated.cursor)
	}
}

func TestHomeModel_Update_Up_CollectionPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 1
	updated, _ := m.Update(kp("up"))
	if updated.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", updated.cursor)
	}
}

func TestHomeModel_Update_Up_AtTop(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	updated, _ := m.Update(kp("up"))
	if updated.cursor != 0 {
		t.Errorf("cursor at top after up = %d, want 0", updated.cursor)
	}
}

func TestHomeModel_Update_K_Navigation(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 1
	updated, _ := m.Update(kp("k"))
	if updated.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", updated.cursor)
	}
}

func TestHomeModel_Update_J_Navigation(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	updated, _ := m.Update(kp("j"))
	if updated.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", updated.cursor)
	}
}

func TestHomeModel_Update_AltLeft_SwitchesPanelFocus(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	updated, _ := m.Update(kp("alt+left"))
	if !updated.panels.IsFocused(panelCollections) {
		t.Errorf("focus after alt+left = %d, want panelCollections", updated.panels.focus)
	}
}

func TestHomeModel_Update_AltRight_SwitchesPanelFocus(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelCollections
	updated, _ := m.Update(kp("alt+right"))
	if !updated.panels.IsFocused(panelData) {
		t.Errorf("focus after alt+right = %d, want panelData", updated.panels.focus)
	}
}

func TestHomeModel_Update_FilterTyping(t *testing.T) {
	t.Parallel()
	// "xfoo" is unique among collection names; filtering by "x" leaves only it.
	def := defWithCollections("xfoo", "bar", "baz")
	m := newHomeModel("/repo", def, nil, 120, 40)
	updated, _ := m.Update(kp("x"))
	if updated.filterValue != "x" {
		t.Errorf("filterValue = %q, want 'x'", updated.filterValue)
	}
	if len(updated.filteredCollections) != 1 {
		t.Errorf("filteredCollections after 'x' = %d, want 1 (xfoo)", len(updated.filteredCollections))
	}
}

func TestHomeModel_Update_Backspace_ClearsFilter(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "al"
	m.filteredCollections = m.applyFilter()
	updated, _ := m.Update(kp("backspace"))
	if updated.filterValue != "a" {
		t.Errorf("filterValue after backspace = %q, want 'a'", updated.filterValue)
	}
}

func TestHomeModel_Update_Backspace_EmptyFilter_NoOp(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = ""
	updated, _ := m.Update(kp("backspace"))
	if updated.filterValue != "" {
		t.Errorf("backspace on empty filter should not change filterValue; got %q", updated.filterValue)
	}
}

func TestHomeModel_Update_Backspace_NonCollectionPanel_NoOp(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.filterValue = "al"
	m.panels.focus = panelData // focus not on collections panel
	updated, _ := m.Update(kp("backspace"))
	if updated.filterValue != "al" {
		t.Errorf("backspace on non-collections panel should not clear filter; got %q", updated.filterValue)
	}
}

func TestHomeModel_Update_FilterTruncatesCursor(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha", "beta", "gamma")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 3                    // past all collections
	updated, _ := m.Update(kp("z")) // "z" filters to nothing
	if updated.cursor > len(updated.filteredCollections) {
		t.Errorf("cursor=%d exceeds filteredCollections len=%d after filter", updated.cursor, len(updated.filteredCollections))
	}
}

func TestHomeModel_Update_Home_DataPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(10)}
	m.recordCursor = 7
	m.recordOffset = 5
	updated, _ := m.Update(kp("home"))
	if updated.recordCursor != 0 || updated.recordOffset != 0 {
		t.Errorf("after home: cursor=%d offset=%d, want 0 0", updated.recordCursor, updated.recordOffset)
	}
}

func TestHomeModel_Update_End_DataPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(20)}
	updated, _ := m.Update(kp("end"))
	if updated.recordCursor != 19 {
		t.Errorf("after end: cursor=%d, want 19", updated.recordCursor)
	}
}

func TestHomeModel_Update_Down_DataPanel_ScrollsOffset(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(50)}
	// Send many down keys to force scrolling.
	current := m
	for i := 0; i < 40; i++ {
		updated, _ := current.Update(kp("down"))
		current = updated
	}
	if current.recordCursor != 40 {
		t.Errorf("recordCursor after 40 downs = %d, want 40", current.recordCursor)
	}
}

func TestHomeModel_Update_Up_DataPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(10)}
	m.recordCursor = 5
	m.recordOffset = 3
	updated, _ := m.Update(kp("up"))
	if updated.recordCursor != 4 {
		t.Errorf("recordCursor after up = %d, want 4", updated.recordCursor)
	}
}

func TestHomeModel_Update_Up_DataPanel_ScrollsOffset(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(10)}
	m.recordCursor = 3
	m.recordOffset = 3 // cursor == offset, moving up should adjust offset
	updated, _ := m.Update(kp("up"))
	if updated.recordOffset > updated.recordCursor {
		t.Errorf("offset=%d > cursor=%d after up", updated.recordOffset, updated.recordCursor)
	}
}

func TestHomeModel_Update_Up_DataPanel_AtTop_MultipleLocales(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{
		records: makeRecords(5),
		locales: []string{"en", "fr"},
		locale:  "en",
		colDef:  simpleColDef("alpha", "name"),
		columns: []string{"name"},
	}
	m.recordCursor = 0
	// up at top with multiple locales → should delegate to preview (opens locale dropdown)
	updated, _ := m.Update(kp("up"))
	_ = updated // just assert no panic
}

func TestHomeModel_Update_L_DataPanel(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{
		records: makeRecords(1),
		locales: []string{"en", "fr"},
		locale:  "en",
		colDef:  simpleColDef("alpha", "name"),
		columns: []string{"name"},
	}
	updated, _ := m.Update(kp("l"))
	// Locale dropdown should have been opened in the preview.
	if updated.preview != nil && !updated.preview.localeDropdownOpen {
		t.Error("'l' with data panel focused and multiple locales should open locale dropdown in preview")
	}
}

func TestHomeModel_Update_Esc_ClosesLocaleDropdown(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{
		localeDropdownOpen: true,
		locales:            []string{"en", "fr"},
		colDef:             simpleColDef("alpha", "name"),
		columns:            []string{"name"},
	}
	updated, _ := m.Update(kp("esc"))
	if updated.preview != nil && updated.preview.localeDropdownOpen {
		t.Error("esc should close locale dropdown in preview")
	}
}

func TestHomeModel_Update_Enter_LocaleDropdownOpen(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{
		localeDropdownOpen:   true,
		locales:              []string{"en", "fr"},
		locale:               "en",
		localeDropdownCursor: 1,
		colDef: &ingitdb.CollectionDef{
			ID: "alpha",
			Columns: map[string]*ingitdb.ColumnDef{
				"title": {Type: ingitdb.ColumnTypeL10N},
			},
			ColumnsOrder: []string{"title"},
		},
		columns: []string{"title.en"},
	}
	updated, _ := m.Update(kp("enter"))
	if updated.preview != nil && updated.preview.localeDropdownOpen {
		t.Error("enter should close locale dropdown")
	}
}

func TestHomeModel_Update_RecordsLoaded_WithPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	col := collectionModel{
		colDef:  simpleColDef("alpha", "name"),
		columns: []string{"name"},
		loading: true,
	}
	m.preview = &col
	records := []map[string]any{{"name": "Alice"}}
	updated, _ := m.Update(recordsLoadedMsg{records: records, keys: []string{"1"}})
	if updated.preview == nil {
		t.Fatal("preview should not be nil after recordsLoadedMsg")
	}
	if updated.preview.loading {
		t.Error("preview should not be loading after recordsLoadedMsg")
	}
}

func TestHomeModel_Update_RecordsLoaded_NilPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.preview = nil
	// Should not panic.
	updated, cmd := m.Update(recordsLoadedMsg{records: nil, keys: nil})
	_ = updated
	_ = cmd
}

func TestHomeModel_Update_WindowResize(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	if updated.width != 200 || updated.height != 60 {
		t.Errorf("after resize: w=%d h=%d, want 200 60", updated.width, updated.height)
	}
}

func TestHomeModel_Update_WindowResize_WithPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.preview = &collectionModel{colDef: simpleColDef("alpha", "name"), columns: []string{"name"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	if updated.preview == nil {
		t.Fatal("preview should not be nil after resize")
	}
	if updated.preview.width != 150 || updated.preview.height != 50 {
		t.Errorf("preview size after resize: w=%d h=%d, want 150 50", updated.preview.width, updated.preview.height)
	}
}

func TestHomeModel_Update_UnknownMsg(t *testing.T) {
	t.Parallel()
	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	// An unhandled message type — should fall through without panic.
	type customMsg struct{}
	updated, cmd := m.Update(customMsg{})
	_ = updated
	_ = cmd
}

// ---------------------------------------------------------------------------
// homeModel.View — preview path
// ---------------------------------------------------------------------------

func TestHomeModel_View_WithPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = 0
	m.preview = &collectionModel{
		colDef:  def.Collections["users"],
		columns: []string{"name"},
		loading: false,
		records: []map[string]any{{"name": "Alice"}},
	}
	m.preview.colWidths = m.preview.computeColWidths()
	rendered := m.View()
	if !strings.Contains(rendered, "users") {
		t.Errorf("View() with preview should contain 'users'; got first 300 chars:\n%s", rendered[:min(300, len(rendered))])
	}
}

func TestHomeModel_View_CursorOnAddButton(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.cursor = len(m.filteredCollections) // add-button
	rendered := m.View()
	if !strings.Contains(rendered, addBtnText) {
		t.Errorf("View() cursor on add-button should show add button text; got:\n%s", rendered)
	}
}

// ---------------------------------------------------------------------------
// home_panel: renderSchema and renderRecords delegation
// ---------------------------------------------------------------------------

func TestHomePanel_RenderSchema_WithPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	colDef := def.Collections["users"]
	m.preview = &collectionModel{
		colDef:      colDef,
		schemaLines: buildSchemaLines(colDef),
	}
	rendered := m.renderSchema(40, 20)
	if rendered == "" {
		t.Error("renderSchema with preview should return non-empty content")
	}
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "users") {
		t.Errorf("renderSchema should contain collection id 'users'; got %q", plain)
	}
}

func TestHomePanel_RenderSchema_NilPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.preview = nil
	got := m.renderSchema(40, 20)
	if got != "" {
		t.Errorf("renderSchema nil preview = %q, want empty", got)
	}
}

func TestHomePanel_RenderRecords_WithPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	colDef := def.Collections["users"]
	m.preview = &collectionModel{
		colDef:  colDef,
		columns: []string{"name"},
		records: []map[string]any{{"name": "Alice"}, {"name": "Bob"}},
	}
	m.preview.colWidths = m.preview.computeColWidths()
	rendered := m.renderRecords(80, 20)
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "name") {
		t.Errorf("renderRecords should contain 'name' header; got %q", plain)
	}
}

func TestHomePanel_RenderRecords_NilPreview(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.preview = nil
	got := m.renderRecords(80, 20)
	if got != "" {
		t.Errorf("renderRecords nil preview = %q, want empty", got)
	}
}

func TestHomePanel_RenderRecords_OverridesCursorTemporarily(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := newHomeModel("/repo", def, nil, 120, 40)
	colDef := def.Collections["users"]
	m.preview = &collectionModel{
		colDef:       colDef,
		columns:      []string{"name"},
		records:      []map[string]any{{"name": "Alice"}, {"name": "Bob"}},
		recordCursor: 0, // preview's own cursor
	}
	m.preview.colWidths = m.preview.computeColWidths()
	m.recordCursor = 1 // home's cursor overrides
	m.renderRecords(80, 20)
	// After renderRecords, preview's cursor must be restored.
	if m.preview.recordCursor != 0 {
		t.Errorf("preview.recordCursor after renderRecords = %d, want 0 (restored)", m.preview.recordCursor)
	}
}
