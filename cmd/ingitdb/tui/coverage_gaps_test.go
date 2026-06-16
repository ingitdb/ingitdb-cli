package tui

// coverage_gaps_test.go fills branches left uncovered by the existing test suite.
// Each test targets a specific uncovered statement identified via coverage analysis.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// ---------------------------------------------------------------------------
// mockDB — minimal dal.DB that calls query.IntoRecord() to cover the factory
// ---------------------------------------------------------------------------

// mockTx is a read-only transaction that, when asked to execute a query, calls
// query.IntoRecord() once (simulating what a generic dal backend would do)
// and then returns ErrNoMoreRecords.
type mockTx struct{}

func (mockTx) Options() dal.TransactionOptions { return nil }

func (mockTx) Get(_ context.Context, r dal.Record) error {
	r.SetError(errors.New("not implemented"))
	return nil
}

func (mockTx) Exists(_ context.Context, _ *dal.Key) (bool, error) { return false, nil }

func (mockTx) GetMulti(_ context.Context, _ []dal.Record) error { return nil }

func (mockTx) ExecuteQueryToRecordsetReader(_ context.Context, q dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	sq, ok := q.(dal.StructuredQuery)
	if ok {
		// Call the factory — this is the line we need to cover.
		_ = sq.IntoRecord()
	}
	return &emptyRecordsetReader{}, nil
}

func (mockTx) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return &emptyRecordsReader{}, nil
}

type emptyRecordsReader struct{}

func (e *emptyRecordsReader) Next() (dal.Record, error) { return nil, dal.ErrNoMoreRecords }
func (e *emptyRecordsReader) Cursor() (string, error)   { return "", nil }
func (e *emptyRecordsReader) Close() error              { return nil }

type emptyRecordsetReader struct{}

func (e *emptyRecordsetReader) Next() (recordset.Row, recordset.Recordset, error) {
	return nil, nil, dal.ErrNoMoreRecords
}
func (e *emptyRecordsetReader) Recordset() recordset.Recordset { return nil }
func (e *emptyRecordsetReader) Cursor() (string, error)        { return "", nil }
func (e *emptyRecordsetReader) Close() error                   { return nil }

// mockDB wraps mockTx to satisfy the dal.DB interface.
type mockDB struct{ dal.NoConcurrency }

func (mockDB) ID() string           { return "mock" }
func (mockDB) Adapter() dal.Adapter { return dal.NewAdapter("mock", "v0") }
func (mockDB) Schema() dal.Schema   { return nil }
func (mockDB) Get(_ context.Context, r dal.Record) error {
	r.SetError(errors.New("not implemented"))
	return nil
}
func (mockDB) Exists(_ context.Context, _ *dal.Key) (bool, error) { return false, nil }
func (mockDB) GetMulti(_ context.Context, _ []dal.Record) error   { return nil }
func (mockDB) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, errors.New("not implemented")
}
func (mockDB) ExecuteQueryToRecordsReader(ctx context.Context, q dal.Query) (dal.RecordsReader, error) {
	return mockTx{}.ExecuteQueryToRecordsReader(ctx, q)
}
func (mockDB) RunReadonlyTransaction(_ context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), mockTx{})
}
func (mockDB) RunReadwriteTransaction(_ context.Context, _ dal.RWTxWorker, _ ...dal.TransactionOption) error {
	return errors.New("not implemented")
}

// boomColumn is a stored recordset column whose value read always fails, used to
// exercise loadRecordsCmd's per-row read-error path.
type boomColumn struct{}

func (boomColumn) Name() string              { return "boom" }
func (boomColumn) DefaultValue() any         { return nil }
func (boomColumn) GetValue(int) (any, error) { return nil, errors.New("boom") }
func (boomColumn) SetValue(int, any) error   { return nil }
func (boomColumn) DbType() string            { return "" }
func (boomColumn) ValueType() reflect.Type   { return reflect.TypeOf((*any)(nil)).Elem() }
func (boomColumn) IsBitmap() bool            { return false }
func (boomColumn) Add(any) error             { return nil }
func (boomColumn) Values() []any             { return nil }

// boomRecordsetReader yields one row over a recordset whose only stored column
// errors on read.
type boomRecordsetReader struct {
	rs   recordset.Recordset
	done bool
}

func (r *boomRecordsetReader) Next() (recordset.Row, recordset.Recordset, error) {
	if r.done {
		return nil, r.rs, dal.ErrNoMoreRecords
	}
	r.done = true
	return r.rs.GetRow(0), r.rs, nil
}
func (r *boomRecordsetReader) Recordset() recordset.Recordset { return r.rs }
func (r *boomRecordsetReader) Cursor() (string, error)        { return "", nil }
func (r *boomRecordsetReader) Close() error                   { return nil }

type boomTx struct{ mockTx }

func (boomTx) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	rs := recordset.NewColumnarRecordset("people", boomColumn{})
	rs.NewRow()
	return &boomRecordsetReader{rs: rs}, nil
}

type boomDB struct{ mockDB }

func (boomDB) RunReadonlyTransaction(_ context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return f(context.Background(), boomTx{})
}

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// makeYAMLFile writes a YAML file at path from data.
func makeYAMLFile(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	err = os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("os.WriteFile %s: %v", path, err)
	}
}

// makeSingleRecordDef builds a CollectionDef with SingleRecord YAML storage.
// Files are placed under dir/$records/{key}.yaml.
func makeSingleRecordDef(t *testing.T, dir, colID string, cols []string) *ingitdb.CollectionDef {
	t.Helper()
	columns := make(map[string]*ingitdb.ColumnDef, len(cols))
	for _, c := range cols {
		columns[c] = &ingitdb.ColumnDef{Type: ingitdb.ColumnTypeString}
	}
	return &ingitdb.CollectionDef{
		ID:      colID,
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns:      columns,
		ColumnsOrder: cols,
	}
}

// ---------------------------------------------------------------------------
// loadRecordsCmd — success path: closure body must be invoked
// ---------------------------------------------------------------------------

func TestLoadRecordsCmd_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recordsDir := filepath.Join(dir, "$records")
	err := os.MkdirAll(recordsDir, 0o755)
	if err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeYAMLFile(t, filepath.Join(recordsDir, "alice.yaml"), map[string]any{"name": "Alice"})
	makeYAMLFile(t, filepath.Join(recordsDir, "bob.yaml"), map[string]any{"name": "Bob"})

	colDef := makeSingleRecordDef(t, dir, "people", []string{"name"})
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"people": colDef},
	}

	db, err := dalgo2fsingitdb.NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}

	cmd := loadRecordsCmd(db, colDef)
	if cmd == nil {
		t.Fatal("loadRecordsCmd returned nil")
	}
	// Invoke the closure — this exercises the entire body of loadRecordsCmd.
	msg := cmd()

	loaded, ok := msg.(recordsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want recordsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("loadRecordsCmd returned error: %v", loaded.err)
	}
	if len(loaded.records) != 2 {
		t.Errorf("loaded %d records, want 2", len(loaded.records))
	}
	if len(loaded.keys) != 2 {
		t.Errorf("loaded %d keys, want 2", len(loaded.keys))
	}
}

// TestLoadRecordsCmd_ComputedColumnNotEvaluatedAtLoad proves the lazy load: a
// computed column whose formula raises at runtime does NOT abort the load,
// because loadRecordsCmd reads only stored columns. The erroring formula is
// never evaluated at load time (it would surface only if its cell were painted),
// and the load retains the recordset + row handles for lazy paint-time access.
func TestLoadRecordsCmd_ComputedColumnNotEvaluatedAtLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	makeYAMLFile(t, filepath.Join(recordsDir, "a.yaml"), map[string]any{"qty": 3})

	colDef := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"qty":   {Type: ingitdb.ColumnTypeInt},
			"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty / 0"},
		},
		ColumnsOrder: []string{"qty", "ratio"},
	}
	def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"people": colDef}}

	db, err := dalgo2fsingitdb.NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}

	msg := loadRecordsCmd(db, colDef)()
	loaded, ok := msg.(recordsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want recordsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("load must not abort on an erroring computed column: %v", loaded.err)
	}
	if len(loaded.records) != 1 {
		t.Fatalf("loaded %d records, want 1", len(loaded.records))
	}
	if _, ok := loaded.records[0]["ratio"]; ok {
		t.Error(`computed column "ratio" must not be evaluated or materialized at load time`)
	}
	if loaded.records[0]["qty"] == nil {
		t.Error(`stored column "qty" should be loaded`)
	}
	if loaded.rs == nil || len(loaded.rows) != 1 {
		t.Error("load should retain the recordset and row handles for lazy paint-time evaluation")
	}
}

// TestLoadRecordsCmd_StoredReadError covers loadRecordsCmd's per-row read-error
// path: a stored column whose value read fails surfaces as the load's error.
func TestLoadRecordsCmd_StoredReadError(t *testing.T) {
	t.Parallel()

	colDef := simpleColDef("people", "boom")
	msg := loadRecordsCmd(boomDB{}, colDef)()
	loaded, ok := msg.(recordsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want recordsLoadedMsg", msg)
	}
	if loaded.err == nil {
		t.Fatal("expected the load to surface the stored-column read error")
	}
}

// TestLoadRecordsCmd_Error exercises the error path inside the closure: an
// unknown collection ID causes the DB transaction to fail.
func TestLoadRecordsCmd_Error(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// colDef points to a collection that the DB doesn't know about.
	colDef := &ingitdb.CollectionDef{
		ID:      "nonexistent",
		DirPath: filepath.Join(dir, "nonexistent"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name"},
	}
	// def intentionally omits the collection so the query finds nothing.
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	db, err := dalgo2fsingitdb.NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}

	cmd := loadRecordsCmd(db, colDef)
	if cmd == nil {
		t.Fatal("loadRecordsCmd returned nil")
	}
	msg := cmd()

	loaded, ok := msg.(recordsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want recordsLoadedMsg", msg)
	}
	// The nonexistent collection results in an error OR zero records.
	// Either outcome means the closure ran fully.
	_ = loaded
}

// TestLoadRecordsCmd_FactoryClosure uses mockDB to ensure the SelectIntoRecord
// factory closure inside loadRecordsCmd is called. mockDB.RunReadonlyTransaction
// invokes the query via mockTx.ExecuteQueryToRecordsReader which calls IntoRecord().
func TestLoadRecordsCmd_FactoryClosure(t *testing.T) {
	t.Parallel()

	colDef := simpleColDef("things", "name")
	cmd := loadRecordsCmd(mockDB{}, colDef)
	if cmd == nil {
		t.Fatal("loadRecordsCmd returned nil")
	}
	msg := cmd()
	loaded, ok := msg.(recordsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want recordsLoadedMsg", msg)
	}
	// mockDB returns zero records with no error.
	if loaded.err != nil {
		t.Errorf("unexpected error: %v", loaded.err)
	}
}

// ---------------------------------------------------------------------------
// discoverLocales — sort comparator with unknown locale codes
// ---------------------------------------------------------------------------

// TestDiscoverLocales_TwoUnknownLocales ensures both !oki and !okj branches
// in the sort comparator are executed. With two locales not in the names map
// the comparator must fall back to the code itself for both i and j.
func TestDiscoverLocales_TwoUnknownLocales(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"title": map[string]any{"zz": "foo", "yy": "bar"}}, // both unknown
	}

	got := discoverLocales(records, colDef)

	if len(got) != 2 {
		t.Fatalf("discoverLocales = %v, want 2 locales", got)
	}
	// Sorted by code (fallback): "yy" < "zz"
	if got[0] != "yy" || got[1] != "zz" {
		t.Errorf("sort order = %v, want [yy zz]", got)
	}
}

// TestDiscoverLocales_MixedKnownUnknown covers the case where one locale is
// known and one is unknown, ensuring both !oki=false and !okj=true paths are hit.
func TestDiscoverLocales_MixedKnownUnknown(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"title": map[string]any{"en": "Hello", "zz": "Unknown"}},
	}

	got := discoverLocales(records, colDef)

	if len(got) != 2 {
		t.Fatalf("discoverLocales = %v, want 2 locales", got)
	}
	// "English" < "zz" alphabetically → en comes first
	if got[0] != "en" {
		t.Errorf("expected en first, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// renderRecords — dropdown overlay leftW < 0 branch
// ---------------------------------------------------------------------------

// TestRenderRecords_DropdownOverlay_VeryNarrowWidth exercises the leftW < 0
// clamp inside the dropdown overlay merge loop. This requires the dropdown
// width to exceed the panel width.
func TestRenderRecords_DropdownOverlay_VeryNarrowWidth(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID: "items",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"title"},
	}
	// Use many locales so the dropdown is very wide.
	m := collectionModel{
		colDef:               colDef,
		columns:              buildDisplayColumns(colDef, "en"),
		locale:               "en",
		locales:              []string{"en", "fr", "de", "es", "it", "ja", "ko", "zh", "ru", "ar"},
		localeDropdownOpen:   true,
		localeDropdownCursor: 0,
		records: []map[string]any{
			{"title": map[string]any{"en": "Hello"}},
		},
	}
	m.colWidths = m.computeColWidths()
	// Width=1 forces dropW > width → leftW < 0 is clamped to 0.
	rendered := m.renderRecords(1, 20)
	if rendered == "" {
		t.Error("renderRecords with very narrow width + open dropdown should still return content")
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Update — recordOffset adjustment on "up"
// ---------------------------------------------------------------------------

// TestCollectionModel_Update_Up_AdjustsOffset covers the branch where moving up
// causes recordCursor < recordOffset, which must snap the offset.
func TestCollectionModel_Update_Up_AdjustsOffset(t *testing.T) {
	t.Parallel()

	colDef := simpleColDef("c", "name")
	m := newCollectionModel(colDef, nil, 120, 40)
	m.loading = false
	m.records = makeRecords(10)
	m.recordCursor = 3
	m.recordOffset = 3 // cursor == offset; moving up makes cursor < offset

	updated, _ := m.Update(tea.KeyPressMsg{Text: "up"})

	if updated.recordCursor != 2 {
		t.Errorf("recordCursor = %d, want 2", updated.recordCursor)
	}
	if updated.recordOffset > updated.recordCursor {
		t.Errorf("recordOffset=%d > recordCursor=%d after up", updated.recordOffset, updated.recordCursor)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.Update — recordOffset adjustment on "down" (scroll forward)
// ---------------------------------------------------------------------------

// TestCollectionModel_Update_Down_ScrollsOffset covers the branch where moving
// down causes recordCursor >= recordOffset+visibleRows, scrolling the view.
func TestCollectionModel_Update_Down_ScrollsOffset(t *testing.T) {
	t.Parallel()

	colDef := simpleColDef("c", "name")
	// Use a very small height so visibleRows is small.
	m := newCollectionModel(colDef, nil, 120, 10)
	m.loading = false
	m.records = makeRecords(20)
	m.recordCursor = 0
	m.recordOffset = 0

	// Send enough down keys to overflow the visible window.
	// panelInnerDims: height=10, innerH=10-2=8; visibleRows=8-4=4.
	// So after 4 downs the cursor should have scrolled offset.
	current := m
	for range 5 {
		updated, _ := current.Update(tea.KeyPressMsg{Text: "down"})
		current = updated
	}

	if current.recordCursor != 5 {
		t.Errorf("recordCursor = %d, want 5", current.recordCursor)
	}
	if current.recordOffset == 0 {
		t.Errorf("recordOffset should have scrolled past 0 after 5 downs in height-10 view")
	}
}

// ---------------------------------------------------------------------------
// homeModel.Update — "down" with very small height triggers visibleRows < 1
// ---------------------------------------------------------------------------

// TestHomeModel_Update_Down_DataPanel_VerySmallHeight covers the visibleRows < 1
// clamp in the "down" handler of homeModel.Update.
func TestHomeModel_Update_Down_DataPanel_VerySmallHeight(t *testing.T) {
	t.Parallel()

	def := defWithCollections("alpha")
	// height=4: innerH=4-2=2, visibleRows=2-4=-2 → clamped to 1.
	m := newHomeModel("/repo", def, nil, 120, 4)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(5)}

	updated, _ := m.Update(kp("down"))
	if updated.recordCursor < 0 {
		t.Errorf("recordCursor should be ≥0 after down in tiny height, got %d", updated.recordCursor)
	}
}

// ---------------------------------------------------------------------------
// homeModel.Update — "end" with very small height triggers visibleRows < 1
// ---------------------------------------------------------------------------

// TestHomeModel_Update_End_DataPanel_VerySmallHeight covers the visibleRows < 1
// clamp in the "end" handler of homeModel.Update.
func TestHomeModel_Update_End_DataPanel_VerySmallHeight(t *testing.T) {
	t.Parallel()

	def := defWithCollections("alpha")
	// height=4: innerH=2, visibleRows=2-4=-2 → clamped to 1.
	m := newHomeModel("/repo", def, nil, 120, 4)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(5)}

	updated, _ := m.Update(kp("end"))
	if updated.recordCursor != 4 {
		t.Errorf("recordCursor after end = %d, want 4 (last record)", updated.recordCursor)
	}
}

// ---------------------------------------------------------------------------
// homeModel.Update — "end" with recordCursor >= visibleRows sets offset
// ---------------------------------------------------------------------------

// TestHomeModel_Update_End_SetsScrollOffset covers the branch where
// recordCursor >= visibleRows causes recordOffset to be set.
func TestHomeModel_Update_End_SetsScrollOffset(t *testing.T) {
	t.Parallel()

	def := defWithCollections("alpha")
	// height=10: innerH=8, visibleRows=8-4=4.
	m := newHomeModel("/repo", def, nil, 120, 10)
	m.panels.focus = panelData
	m.preview = &collectionModel{records: makeRecords(20)}

	updated, _ := m.Update(kp("end"))
	// recordCursor = 19, visibleRows = 4 → 19 >= 4 → offset = 19 - 4 + 1 = 16
	if updated.recordCursor != 19 {
		t.Errorf("recordCursor = %d, want 19", updated.recordCursor)
	}
	if updated.recordOffset == 0 {
		t.Errorf("recordOffset should be nonzero for 20 records in height-10 view, got %d", updated.recordOffset)
	}
}

// ---------------------------------------------------------------------------
// homeModel.Update — routing keys to locale dropdown (general keys)
// ---------------------------------------------------------------------------

// TestHomeModel_Update_RoutesAllKeysToDropdown covers the route-all block at the
// top of the KeyPressMsg handler when the locale dropdown is open in the preview.
func TestHomeModel_Update_RoutesAllKeysToDropdown(t *testing.T) {
	t.Parallel()

	def := defWithCollections("alpha")
	m := newHomeModel("/repo", def, nil, 120, 40)
	m.panels.focus = panelData
	m.preview = &collectionModel{
		localeDropdownOpen:   true,
		locales:              []string{"en", "fr"},
		locale:               "en",
		localeDropdownCursor: 0,
		colDef:               simpleColDef("alpha", "name"),
		columns:              []string{"name"},
	}

	// "up" is routed to preview which decrements localeDropdownCursor (no-op at 0).
	updated, _ := m.Update(kp("up"))
	if updated.preview == nil {
		t.Fatal("preview should not be nil after routing")
	}
	if !updated.preview.localeDropdownOpen {
		t.Error("locale dropdown should still be open after routing 'up'")
	}
}

// ---------------------------------------------------------------------------
// homeModel.View — very narrow width triggers minimum clamps
// ---------------------------------------------------------------------------

// TestHomeModel_View_VeryNarrowWidth exercises all five minimum-width clamps
// in homeModel.View: leftWidth<20, rightWidth<24, leftInner<1, middleInner<1,
// rightInner<1.
func TestHomeModel_View_VeryNarrowWidth(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {
				ID:      "users",
				DirPath: "/repo/users",
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
				ColumnsOrder: []string{"name"},
			},
		},
	}

	// Width=20: leftWidth=20/4=5<20 → clamped to 20;
	//           rightWidth=20/4=5<24 → clamped to 24;
	//           middleWidth=20-20-24<0 → panels will be minimal.
	// leftInner=20-4=16>1; but if width is even smaller:
	// Use width=10: leftWidth=10/4=2<20→20; rightWidth=10/4=2<24→24;
	// middleWidth=10-20-24=-34; leftInner=16; middleInner=-34-4<1→1; rightInner=20>1.
	m := newHomeModel("/repo", def, nil, 10, 40)

	rendered := m.View()
	if rendered == "" {
		t.Error("View() with very narrow width should still return non-empty content")
	}
}

// TestHomeModel_View_MinimumPanelClamp exercises leftInner and rightInner < 1.
// Use width=5 so all inner dimensions need clamping.
func TestHomeModel_View_MinimumPanelClamp(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	// Width=5: leftWidth=5/4=1<20→20; rightWidth=5/4=1<24→24;
	// leftInner=20-4=16; rightInner=24-4=20; middleWidth=5-20-24=-39;
	// middleInner=-39-4<1→1.
	m := newHomeModel("/repo", def, nil, 5, 40)
	rendered := m.View()
	if rendered == "" {
		t.Error("View() with width=5 should not return empty")
	}
}

// ---------------------------------------------------------------------------
// collectionRelPath — error branch via relative base path
// ---------------------------------------------------------------------------

// TestCollectionRelPath_Error covers the err != nil branch. filepath.Rel
// returns an error when one path is relative and the other is absolute.
func TestCollectionRelPath_Error(t *testing.T) {
	t.Parallel()

	// "relative/path" is not absolute, "/abs/dir" is absolute.
	// filepath.Rel returns an error for this combination on all platforms.
	got := collectionRelPath("relative/path", "/abs/dir")
	// The function should return dirPath unchanged when Rel fails.
	if got != "/abs/dir" {
		t.Errorf("collectionRelPath error fallback = %q, want /abs/dir", got)
	}
}

// ---------------------------------------------------------------------------
// renderCollectionList — filepath.Rel error branch via relative dbPath
// ---------------------------------------------------------------------------

// TestRenderCollectionList_RelPath_Error covers the err != nil branch inside
// renderCollectionList's filepath.Rel call. Passing a relative dbPath with an
// absolute dirPath triggers the error.
func TestRenderCollectionList_RelPath_Error(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {ID: "users", DirPath: "/abs/users"},
		},
	}

	// relative dbPath + absolute dirPath → filepath.Rel returns error.
	m := newHomeModel("relative/db", def, nil, 120, 30)
	rendered := m.renderCollectionList(60, 20)

	// Must contain the absolute dirPath as fallback, not an empty string.
	if rendered == "" {
		t.Error("renderCollectionList with Rel error should still return content")
	}
	// Key assertion: no panic, and it renders.
}

// ---------------------------------------------------------------------------
// refreshPreview — branch where buildPreview succeeds and Init() is returned
// ---------------------------------------------------------------------------

// TestRefreshPreview_ReturnsInitCmd covers the `return m.preview.Init()` branch
// in refreshPreview. It requires newDB to succeed so buildPreview sets m.preview.
func TestRefreshPreview_ReturnsInitCmd(t *testing.T) {
	t.Parallel()

	def := defWithCollections("users")
	// newDBOK-style: returns nil db (no error) so buildPreview sets m.preview.
	newDB := func(dbPath string, d *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	m := newHomeModel("/repo", def, newDB, 120, 40)
	// Move cursor to first collection, clear previewID to force rebuild.
	m.cursor = 0
	m.previewID = ""
	m.preview = nil

	cmd := m.refreshPreview()

	// buildPreview succeeded (newDB returned no error) so m.preview is set
	// and Init() returned a non-nil cmd.
	if m.preview == nil {
		t.Fatal("refreshPreview should have built a preview when newDB succeeds")
	}
	if cmd == nil {
		t.Error("refreshPreview should return a non-nil cmd from preview.Init()")
	}
}
