package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestModel(ids ...string) Model {
	def := defWithCollections(ids...)
	return New("/repo", def, nil, 120, 40)
}

// newDBOK returns a newDB func that succeeds but returns a nil DB.
func newDBOK() func(string, *ingitdb.Definition) (dal.DB, error) {
	return func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }
}

// newDBFail returns a newDB func that always errors.
func newDBFail() func(string, *ingitdb.Definition) (dal.DB, error) {
	return func(string, *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db open failed")
	}
}

// ---------------------------------------------------------------------------
// New / Init
// ---------------------------------------------------------------------------

func TestModel_New(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	if m.currentScreen != screenHome {
		t.Errorf("currentScreen = %d, want screenHome", m.currentScreen)
	}
	if m.width != 120 || m.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", m.width, m.height)
	}
	if m.collection != nil {
		t.Error("collection should be nil at start")
	}
}

func TestModel_Init(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	// Init delegates to home.Init(); with no collections and no preview, it returns nil.
	cmd := m.Init()
	_ = cmd // may be nil or non-nil, must not panic
}

// ---------------------------------------------------------------------------
// Model.Update — quit keys
// ---------------------------------------------------------------------------

func TestModel_Update_Quit_q(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	_, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if cmd == nil {
		t.Error("'q' should return a non-nil quit command")
	}
}

func TestModel_Update_Quit_CtrlC(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Error("ctrl+c should return a non-nil quit command")
	}
}

// ---------------------------------------------------------------------------
// Model.Update — window resize
// ---------------------------------------------------------------------------

func TestModel_Update_WindowResize(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	m2 := updated.(Model)
	if m2.width != 200 || m2.height != 60 {
		t.Errorf("after resize: %dx%d, want 200x60", m2.width, m2.height)
	}
	if cmd != nil {
		t.Error("window resize should return nil cmd")
	}
}

func TestModel_Update_WindowResize_WithCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m2 := updated.(Model)
	if m2.collection == nil {
		t.Fatal("collection should not be nil after resize")
	}
	if m2.collection.width != 160 || m2.collection.height != 50 {
		t.Errorf("collection size after resize: %dx%d, want 160x50", m2.collection.width, m2.collection.height)
	}
}

// ---------------------------------------------------------------------------
// Model.Update — backspace navigation
// ---------------------------------------------------------------------------

func TestModel_Update_Backspace_OnHome_NoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	m.currentScreen = screenHome
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("backspace on home should stay on home; got screen %d", m2.currentScreen)
	}
}

func TestModel_Update_Backspace_OnCollection_GoesHome(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("backspace on collection should go home; got screen %d", m2.currentScreen)
	}
	if m2.collection != nil {
		t.Error("collection should be nil after backspace from collection screen")
	}
}

// ---------------------------------------------------------------------------
// Model.Update — esc navigation
// ---------------------------------------------------------------------------

func TestModel_Update_Esc_OnHome_RoutesToHome(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	m.currentScreen = screenHome
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("esc on home screen should stay home; got %d", m2.currentScreen)
	}
}

func TestModel_Update_Esc_OnCollection_GoesHome(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("esc on collection should go home; got screen %d", m2.currentScreen)
	}
}

func TestModel_Update_Esc_OnCollection_WithDropdownOpen(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	col.localeDropdownOpen = true
	col.locales = []string{"en", "fr"}
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m2 := updated.(Model)
	// Should close the dropdown rather than navigate back.
	if m2.currentScreen != screenCollection {
		t.Errorf("esc with dropdown open should stay on collection screen; got %d", m2.currentScreen)
	}
	if m2.collection != nil && m2.collection.localeDropdownOpen {
		t.Error("esc should close locale dropdown")
	}
}

// ---------------------------------------------------------------------------
// Model.Update — enter to open collection
// ---------------------------------------------------------------------------

func TestModel_Update_Enter_OnHome_NoSelection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	// Cursor on add-button (no collection selected).
	m.home.cursor = len(m.home.filteredCollections)
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("enter with no collection selected should stay home; got screen %d", m2.currentScreen)
	}
}

func TestModel_Update_Enter_OnHome_WithDBError(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, newDBFail(), 120, 40)
	m.home.cursor = 0
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := updated.(Model)
	// DB failed to open → stay on home, no crash.
	if m2.currentScreen != screenHome {
		t.Errorf("enter with DB error should stay home; got screen %d", m2.currentScreen)
	}
}

func TestModel_Update_Enter_OnHome_OpenCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, newDBOK(), 120, 40)
	m.home.cursor = 0
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := updated.(Model)
	if m2.currentScreen != screenCollection {
		t.Errorf("enter with valid collection should open collection screen; got screen %d", m2.currentScreen)
	}
	if m2.collection == nil {
		t.Error("collection should not be nil after opening")
	}
}

func TestModel_Update_Enter_OnHome_LocaleDropdownOpen(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	m.home.panels.focus = panelData
	m.home.preview = &collectionModel{
		localeDropdownOpen:   true,
		locales:              []string{"en", "fr"},
		locale:               "en",
		localeDropdownCursor: 1,
		colDef: &ingitdb.CollectionDef{
			ID: "users",
			Columns: map[string]*ingitdb.ColumnDef{
				"title": {Type: ingitdb.ColumnTypeL10N},
			},
			ColumnsOrder: []string{"title"},
		},
		columns: []string{"title.en"},
	}
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m2 := updated.(Model)
	// Enter should close the dropdown, not open collection.
	if m2.currentScreen != screenHome {
		t.Errorf("enter with dropdown open should stay home; got screen %d", m2.currentScreen)
	}
}

// ---------------------------------------------------------------------------
// Model.Update — recordsLoadedMsg routing
// ---------------------------------------------------------------------------

func TestModel_Update_RecordsLoaded_ToHome(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	m.currentScreen = screenHome
	m.home.preview = &collectionModel{
		colDef:  def.Collections["users"],
		columns: []string{"name"},
		loading: true,
	}
	updated, _ := m.Update(recordsLoadedMsg{
		records: []map[string]any{{"name": "Alice"}},
		keys:    []string{"1"},
	})
	m2 := updated.(Model)
	if m2.home.preview == nil {
		t.Fatal("home preview should not be nil")
	}
	if m2.home.preview.loading {
		t.Error("home preview should not be loading after recordsLoadedMsg")
	}
}

func TestModel_Update_RecordsLoaded_ToCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(recordsLoadedMsg{
		records: []map[string]any{{"name": "Bob"}},
		keys:    []string{"1"},
	})
	m2 := updated.(Model)
	if m2.collection == nil {
		t.Fatal("collection should not be nil")
	}
	if m2.collection.loading {
		t.Error("collection should not be loading after recordsLoadedMsg")
	}
	if len(m2.collection.records) != 1 {
		t.Errorf("records = %d, want 1", len(m2.collection.records))
	}
}

func TestModel_Update_RecordsLoaded_NilCollection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	m.currentScreen = screenCollection
	m.collection = nil
	// Should not panic.
	updated, _ := m.Update(recordsLoadedMsg{records: nil, keys: nil})
	_ = updated
}

// ---------------------------------------------------------------------------
// Model.Update — delegate unhandled messages
// ---------------------------------------------------------------------------

func TestModel_Update_Delegate_Home(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	m.currentScreen = screenHome
	// An unhandled key on home screen gets delegated to homeModel.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	m2 := updated.(Model)
	if m2.currentScreen != screenHome {
		t.Errorf("unhandled key on home should stay home; got %d", m2.currentScreen)
	}
}

func TestModel_Update_Delegate_Collection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	m2 := updated.(Model)
	if m2.currentScreen != screenCollection {
		t.Errorf("unhandled key on collection should stay on collection; got %d", m2.currentScreen)
	}
}

func TestModel_Update_Delegate_CollectionNil(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	m.currentScreen = screenCollection
	m.collection = nil
	// Should not panic.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "down"})
	_ = updated
}

// ---------------------------------------------------------------------------
// Model.View
// ---------------------------------------------------------------------------

func TestModel_View_HomeScreen(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	v := m.View()
	content := v.Content
	if !strings.Contains(content, "inGitDB") {
		t.Errorf("View() on home screen should contain 'inGitDB'; got first 200:\n%s", content[:min(200, len(content))])
	}
}

func TestModel_View_CollectionScreen(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	v := m.View()
	content := v.Content
	if !strings.Contains(content, "users") {
		t.Errorf("View() on collection screen should contain 'users'; got first 200:\n%s", content[:min(200, len(content))])
	}
}

func TestModel_View_CollectionScreen_NilCollection(t *testing.T) {
	t.Parallel()
	m := newTestModel("users")
	m.currentScreen = screenCollection
	m.collection = nil
	// Should not panic.
	v := m.View()
	_ = v
}

// ---------------------------------------------------------------------------
// Model.renderHeader
// ---------------------------------------------------------------------------

func TestModel_RenderHeader_Home(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	header := m.renderHeader()
	plain := stripAnsi(header)
	if !strings.Contains(plain, "inGitDB") {
		t.Errorf("renderHeader home should contain 'inGitDB'; got %q", plain)
	}
}

func TestModel_RenderHeader_Collection(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, nil, 120, 40)
	col := newCollectionModel(def.Collections["users"], nil, 120, 40)
	m.collection = &col
	m.currentScreen = screenCollection
	header := m.renderHeader()
	plain := stripAnsi(header)
	if !strings.Contains(plain, "users") {
		t.Errorf("renderHeader collection should contain 'users'; got %q", plain)
	}
}

func TestModel_RenderHeader_Collection_NilCollection(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	m.currentScreen = screenCollection
	m.collection = nil
	// Should not panic; colID will be empty.
	header := m.renderHeader()
	plain := stripAnsi(header)
	if !strings.Contains(plain, "inGitDB") {
		t.Errorf("renderHeader nil collection should still contain 'inGitDB'; got %q", plain)
	}
}

// ---------------------------------------------------------------------------
// openCollection
// ---------------------------------------------------------------------------

func TestModel_OpenCollection_Success(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, newDBOK(), 120, 40)
	colDef := def.Collections["users"]
	m2, cmd := m.openCollection(colDef)
	if m2.currentScreen != screenCollection {
		t.Errorf("openCollection screen = %d, want screenCollection", m2.currentScreen)
	}
	if m2.collection == nil {
		t.Error("openCollection should set collection")
	}
	if cmd == nil {
		t.Error("openCollection should return a non-nil init cmd")
	}
}

func TestModel_OpenCollection_DBError(t *testing.T) {
	t.Parallel()
	def := defWithCollections("users")
	m := New("/repo", def, newDBFail(), 120, 40)
	colDef := def.Collections["users"]
	m2, cmd := m.openCollection(colDef)
	if m2.currentScreen != screenHome {
		t.Errorf("openCollection with DB error screen = %d, want screenHome", m2.currentScreen)
	}
	if cmd != nil {
		t.Error("openCollection with DB error should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// collectionRelPath — error branch
// ---------------------------------------------------------------------------

func TestCollectionRelPath_RelSucceeds(t *testing.T) {
	t.Parallel()
	got := collectionRelPath("/a/b", "/a/b/c")
	if got != "c" {
		t.Errorf("collectionRelPath = %q, want c", got)
	}
}
