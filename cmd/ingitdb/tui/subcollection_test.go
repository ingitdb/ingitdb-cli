package tui

// specscore: feature/subcollection-addressing

import (
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"

	"github.com/ingitdb/ingitdb-cli/internal/testutil"
)

var (
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyEsc   = tea.KeyPressMsg{Code: tea.KeyEsc}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
)

// press sends msg to m, then runs the returned command and feeds every message
// it produces back into the model, as the bubbletea runtime would.
func press(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	pending := []tea.Msg{msg}
	for i := 0; len(pending) > 0; i++ {
		if i > 20 {
			t.Fatal("too many follow-up messages")
		}
		next := pending[0]
		pending = pending[1:]
		updated, cmd := m.Update(next)
		m = updated.(Model)
		if cmd == nil {
			continue
		}
		out := cmd()
		if batch, ok := out.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c != nil {
					pending = append(pending, c())
				}
			}
			continue
		}
		if out != nil {
			pending = append(pending, out)
		}
	}
	return m
}

func newNestedListsModel(t *testing.T) Model {
	t.Helper()
	dir := testutil.WriteNestedListsDB(t)
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	return New(dir, def, newDB, 120, 40)
}

// recordTitles returns the screen's record titles sorted; the screen lists
// records in the driver's key order.
func recordTitles(m *collectionModel) []string {
	titles := make([]string, 0, len(m.records))
	for _, r := range m.records {
		title, _ := r["title"].(string)
		titles = append(titles, title)
	}
	sort.Strings(titles)
	return titles
}

func TestModel_Subcollection_OpenAndLeave(t *testing.T) {
	t.Parallel()
	m := newNestedListsModel(t)

	m = press(t, m, keyEnter) // open lists
	if m.currentScreen != screenCollection || m.collection.colDef.ID != "lists" {
		t.Fatalf("want lists collection screen, got screen %d", m.currentScreen)
	}
	if got := strings.Join(m.collection.recordKeys, ","); got != "to-buy,to-watch" {
		t.Fatalf("lists records = %s, want to-buy,to-watch", got)
	}

	m = press(t, m, keyEnter) // open to-buy's items
	if m.collection.colDef.ID != "items" {
		t.Fatalf("want items screen, got %q", m.collection.colDef.ID)
	}
	if got := strings.Join(recordTitles(m.collection), ","); got != "Bananas,Coffee,Milk" {
		t.Errorf("items titles = %s, want Bananas,Coffee,Milk", got)
	}
	for i, r := range m.collection.records {
		if done, ok := r["done"].(bool); !ok || done {
			t.Errorf("item %s: done = %v, want false", m.collection.recordKeys[i], r["done"])
		}
	}
	view := m.View().Content
	if !strings.Contains(view, "lists › to-buy › items") {
		t.Errorf("header should show the path lists › to-buy › items; got:\n%s", view)
	}
	for _, want := range []string{"Milk", "done", "added_at"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q; got:\n%s", want, view)
		}
	}

	m = press(t, m, keyEsc) // back to lists
	if m.currentScreen != screenCollection || m.collection.colDef.ID != "lists" {
		t.Fatalf("esc should return to lists, got screen %d", m.currentScreen)
	}
	if m.collection.recordCursor != 0 || len(m.collection.records) != 2 {
		t.Errorf("lists state not kept: cursor %d, %d records", m.collection.recordCursor, len(m.collection.records))
	}
	if strings.Contains(m.View().Content, "›  lists › ") {
		t.Error("header should show only lists after going back")
	}

	m = press(t, m, keyEsc) // back home
	if m.currentScreen != screenHome || m.collection != nil || len(m.parents) != 0 {
		t.Errorf("second esc should return home, got screen %d", m.currentScreen)
	}
}

func TestModel_Subcollection_KeepsSelectedParentRecord(t *testing.T) {
	t.Parallel()
	m := newNestedListsModel(t)
	m = press(t, m, keyEnter)
	m = press(t, m, keyDown) // to-watch
	m = press(t, m, keyEnter)
	if got := strings.Join(recordTitles(m.collection), ","); got != "Interstellar,The Matrix" {
		t.Errorf("to-watch items = %s, want Interstellar,The Matrix", got)
	}
	m = press(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.collection == nil || m.collection.colDef.ID != "lists" || m.collection.recordCursor != 1 {
		t.Fatalf("backspace should return to lists with to-watch selected")
	}
}

func TestModel_Subcollection_StaleLoadIgnoredByParent(t *testing.T) {
	t.Parallel()
	m := newNestedListsModel(t)
	m = press(t, m, keyEnter)
	// Open items without letting its load finish, then go back.
	updated, openCmd := m.Update(keyEnter)
	m = updated.(Model)
	updated, loadCmd := m.Update(openCmd())
	m = updated.(Model)
	m = press(t, m, keyEsc)
	stale := loadCmd()
	m = press(t, m, stale)
	if got := strings.Join(m.collection.recordKeys, ","); got != "to-buy,to-watch" {
		t.Errorf("stale subcollection load replaced parent records: %s", got)
	}
}

func TestModel_Subcollection_WindowResizeReachesParents(t *testing.T) {
	t.Parallel()
	m := newNestedListsModel(t)
	m = press(t, m, keyEnter)
	m = press(t, m, keyEnter)
	m = press(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = press(t, m, keyEsc)
	if m.collection.width != 100 || m.collection.height != 30 {
		t.Errorf("parent size = %dx%d, want 100x30", m.collection.width, m.collection.height)
	}
}

// twoSubcollectionsModel returns a loaded collection screen for a collection
// declaring subcollections "notes" and "items", with records r1 and r2.
func twoSubcollectionsModel() collectionModel {
	colDef := simpleColDef("lists", "title")
	colDef.SubCollections = map[string]*ingitdb.CollectionDef{
		"notes": simpleColDef("notes", "text"),
		"items": simpleColDef("items", "title"),
	}
	m := newCollectionModel(colDef, nil, 120, 40)
	updated, _ := m.Update(recordsLoadedMsg{
		path:    "lists",
		records: []map[string]any{{"title": "One"}, {"title": "Two"}},
		keys:    []string{"r1", "r2"},
	})
	return updated
}

func TestCollectionModel_Subcollection_ChooseAmongSeveral(t *testing.T) {
	t.Parallel()
	m := twoSubcollectionsModel()
	m, _ = m.Update(keyDown) // select r2
	m, cmd := m.Update(keyEnter)
	if cmd != nil || !m.subDropdownOpen {
		t.Fatalf("enter with two subcollections should open the chooser")
	}
	view := m.View()
	if !strings.Contains(view, "items") || !strings.Contains(view, "notes") {
		t.Errorf("chooser should list items and notes; got:\n%s", view)
	}
	if !strings.Contains(view, "► items") {
		t.Errorf("chooser cursor should start on items (sorted first); got:\n%s", view)
	}
	m, _ = m.Update(keyDown)
	m, _ = m.Update(keyDown) // stays at the last entry
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = m.Update(keyDown)
	m, cmd = m.Update(keyEnter)
	if m.subDropdownOpen || cmd == nil {
		t.Fatalf("enter in the chooser should close it and open a subcollection")
	}
	msg, ok := cmd().(openSubcollectionMsg)
	if !ok {
		t.Fatalf("want openSubcollectionMsg")
	}
	if msg.colDef.ID != "notes" || strings.Join(msg.path, "/") != "lists/r2/notes" {
		t.Errorf("opened %q at %v, want notes at lists/r2/notes", msg.colDef.ID, msg.path)
	}
	wantParent := record.NewKeyWithID("lists", "r2")
	if msg.parent == nil || msg.parent.String() != wantParent.String() {
		t.Errorf("parent = %v, want %v", msg.parent, wantParent)
	}
}

func TestCollectionModel_Subcollection_ChooserEscClosesOnly(t *testing.T) {
	t.Parallel()
	m := twoSubcollectionsModel()
	m, _ = m.Update(keyEnter)
	m, _ = m.Update(tea.KeyPressMsg{Text: "l"}) // locale dropdown must not open over the chooser
	if m.localeDropdownOpen {
		t.Error("locale dropdown opened while the chooser is open")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp}) // stays at the first entry
	m, _ = m.Update(keyEsc)
	if m.subDropdownOpen {
		t.Error("esc should close the chooser")
	}
}

func TestModel_Subcollection_EscClosesChooserBeforeLeaving(t *testing.T) {
	t.Parallel()
	m := newTestModel("lists")
	col := twoSubcollectionsModel()
	col.subDropdownOpen = true
	m.collection = &col
	m.currentScreen = screenCollection
	m = press(t, m, keyEsc)
	if m.currentScreen != screenCollection || m.collection.subDropdownOpen {
		t.Errorf("esc should only close the chooser, got screen %d", m.currentScreen)
	}
}

func TestCollectionModel_Subcollection_EnterWithoutSubcollectionsDoesNothing(t *testing.T) {
	t.Parallel()
	m := newCollectionModel(simpleColDef("users", "name"), nil, 120, 40)
	m, _ = m.Update(recordsLoadedMsg{records: []map[string]any{{"name": "A"}}, keys: []string{"a"}})
	m, cmd := m.Update(keyEnter)
	if cmd != nil || m.subDropdownOpen {
		t.Error("enter on a collection without subcollections should do nothing")
	}
	empty := twoSubcollectionsModel()
	empty.records, empty.recordKeys = nil, nil
	empty, cmd = empty.Update(keyEnter)
	if cmd != nil || empty.subDropdownOpen {
		t.Error("enter with no records should do nothing")
	}
}

func TestModel_OpenSubcollectionMsg_IgnoredOnHome(t *testing.T) {
	t.Parallel()
	m := newTestModel("lists")
	updated, cmd := m.Update(openSubcollectionMsg{colDef: simpleColDef("items", "title"), path: []string{"lists", "a", "items"}})
	m = updated.(Model)
	if cmd != nil || m.currentScreen != screenHome || m.collection != nil {
		t.Error("openSubcollectionMsg on the home screen should be ignored")
	}
}

func TestCollectionModel_Subcollection_NestedParentChain(t *testing.T) {
	t.Parallel()
	items := simpleColDef("items", "title")
	items.SubCollections = map[string]*ingitdb.CollectionDef{"tags": simpleColDef("tags", "name")}
	m := newCollectionModel(items, nil, 120, 40)
	m.parent = record.NewKeyWithID("lists", "to-buy")
	m.path = []string{"lists", "to-buy", "items"}
	m, _ = m.Update(recordsLoadedMsg{path: "lists/to-buy/items", records: []map[string]any{{"title": "Milk"}}, keys: []string{"milk"}})
	_, cmd := m.Update(keyEnter)
	if cmd == nil {
		t.Fatal("enter with one declared subcollection should open it directly")
	}
	msg := cmd().(openSubcollectionMsg)
	want := record.NewKeyWithParentAndID(record.NewKeyWithID("lists", "to-buy"), "items", "milk")
	if msg.parent.String() != want.String() || strings.Join(msg.path, "/") != "lists/to-buy/items/milk/tags" {
		t.Errorf("parent %v path %v, want %v lists/to-buy/items/milk/tags", msg.parent, msg.path, want)
	}
}

func TestBuildSubcollectionDropdownLines_WidensForLongIDs(t *testing.T) {
	t.Parallel()
	colDef := simpleColDef("lists", "title")
	colDef.SubCollections = map[string]*ingitdb.CollectionDef{"attachments": simpleColDef("attachments", "name")}
	m := newCollectionModel(colDef, nil, 120, 40)
	lines := m.buildSubcollectionDropdownLines()
	if len(lines) != 3 || !strings.Contains(lines[1], "attachments") {
		t.Fatalf("lines = %q", lines)
	}
	if lines[0] != "┌"+strings.Repeat("─", 13)+"┐" {
		t.Errorf("top border %q should fit the widest entry", lines[0])
	}
}
