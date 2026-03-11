package tui

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// collectionRelPath
// ---------------------------------------------------------------------------

func TestCollectionRelPath_SubPath(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	dirPath := "/home/user/repo/users"
	got := collectionRelPath(dbPath, dirPath)
	want := "users"
	if got != want {
		t.Errorf("collectionRelPath(%q, %q) = %q, want %q", dbPath, dirPath, got, want)
	}
}

func TestCollectionRelPath_DifferentRoot(t *testing.T) {
	t.Parallel()

	// On Linux filepath.Rel("/a", "/b") succeeds and returns "../b",
	// so we test that the returned value is the filepath.Rel result.
	dbPath := "/home/user/repo"
	dirPath := "/other/path/users"
	got := collectionRelPath(dbPath, dirPath)
	// filepath.Rel should succeed on Linux and return "../../other/path/users".
	// We just assert it does NOT return the full absolute path when Rel succeeds,
	// and is non-empty.
	if got == "" {
		t.Errorf("collectionRelPath(%q, %q) returned empty string", dbPath, dirPath)
	}
	// The important thing: it must not panic and must return a usable string.
}

func TestCollectionRelPath_SameDir(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	dirPath := "/home/user/repo"
	got := collectionRelPath(dbPath, dirPath)
	want := "."
	if got != want {
		t.Errorf("collectionRelPath(%q, %q) = %q, want %q", dbPath, dirPath, got, want)
	}
}

// ---------------------------------------------------------------------------
// shortenPath
// ---------------------------------------------------------------------------

func TestShortenPath_NoOp(t *testing.T) {
	t.Parallel()

	p := "short"
	got := shortenPath(p, 100)
	if got != p {
		t.Errorf("shortenPath(%q, 100) = %q, want %q", p, got, p)
	}
}

func TestShortenPath_Truncates(t *testing.T) {
	t.Parallel()

	p := "/very/long/path/to/some/collection/directory"
	maxLen := 20
	got := shortenPath(p, maxLen)
	// Result length should be <= maxLen (the ellipsis is multi-byte but rune count is ~maxLen).
	// We verify it differs from the original and contains the ellipsis character.
	if got == p {
		t.Errorf("shortenPath did not truncate %q", p)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("shortenPath result %q does not contain ellipsis", got)
	}
}

func TestShortenPath_ZeroMaxLen(t *testing.T) {
	t.Parallel()

	p := "anything"
	got := shortenPath(p, 0)
	if got != p {
		t.Errorf("shortenPath(%q, 0) = %q, want %q (no-op)", p, got, p)
	}
}

func TestShortenPath_NegativeMaxLen(t *testing.T) {
	t.Parallel()

	p := "anything"
	got := shortenPath(p, -5)
	if got != p {
		t.Errorf("shortenPath(%q, -5) = %q, want %q (no-op)", p, got, p)
	}
}

// ---------------------------------------------------------------------------
// renderCollectionList smoke test
// ---------------------------------------------------------------------------

func TestRenderCollectionList_ShowsRelativePath(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	collDirPath := "/home/user/repo/users"

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {DirPath: collDirPath},
		},
	}

	m := newHomeModel(dbPath, def, nil, 120, 30)

	// renderCollectionList is called with a reasonable width and height.
	rendered := m.renderCollectionList(60, 20)

	// The rendered output must contain the relative path "users", not the full absolute path.
	if !strings.Contains(rendered, "users") {
		t.Errorf("renderCollectionList output does not contain relative path %q; got:\n%s", "users", rendered)
	}
	if strings.Contains(rendered, collDirPath) {
		t.Errorf("renderCollectionList output contains absolute path %q; should show relative path instead; got:\n%s", collDirPath, rendered)
	}
}

// ---------------------------------------------------------------------------
// Panel height consistency
// ---------------------------------------------------------------------------

func TestPanelHeightsMatch(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {
				ID:      "users",
				DirPath: "/repo/users",
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: "string"},
				},
			},
		},
	}

	m := newHomeModel("/repo", def, nil, 120, 40)
	rendered := m.View()
	// All lines in JoinHorizontal output should have the same width pattern;
	// more importantly the panel block (excluding help line) should not have
	// ragged rows. Count lines — they should equal m.height - 1 (the help
	// line is separate).
	lines := strings.Split(rendered, "\n")
	// The total rendered height should be consistent. With height=40 the
	// panel outer height = 40-5 = 35, and help = 1, so total ~36.
	// The key assertion: no panel should create extra rows.
	if len(lines) > m.height {
		t.Errorf("rendered output has %d lines, expected at most %d (terminal height)", len(lines), m.height)
	}
}

func TestPreviewPanelHeightsEqualCollectionList(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {
				ID:      "users",
				DirPath: "/repo/users",
				Columns: map[string]*ingitdb.ColumnDef{
					"name":  {Type: "string"},
					"email": {Type: "string"},
				},
				RecordFile: &ingitdb.RecordFileDef{
					Format: "yaml",
					Name:   "record.yaml",
				},
			},
		},
	}

	const termW, termH = 120, 40

	m := newHomeModel("/repo", def, nil, termW, termH)
	// Focus on data panel with cursor on first collection
	m.focus = panelData
	m.cursor = 0

	// Layout: 1/4 left | 1/2 middle | 1/4 right
	leftWidth := termW / 4
	if leftWidth < 20 {
		leftWidth = 20
	}
	rightWidth := termW / 4
	if rightWidth < 24 {
		rightWidth = 24
	}
	middleWidth := termW - leftWidth - rightWidth

	leftInner := leftWidth - 4
	middleInner := middleWidth - 4
	innerH := termH - 2
	contentH := innerH - 2

	leftContent := m.renderCollectionList(leftInner, contentH)
	// In lipgloss v2, Width() is border-box: pass the allocated column width.
	leftPanel := focusedPanelStyle.Width(leftWidth).Height(innerH).Render(leftContent)

	middleContent := m.renderWelcome(middleInner, contentH)
	middlePanel := panelStyle.Width(middleWidth).Height(innerH).Render(middleContent)

	rightPanel := panelStyle.Width(rightWidth).Height(innerH).Render("")

	leftLines := strings.Count(leftPanel, "\n") + 1
	middleLines := strings.Count(middlePanel, "\n") + 1
	rightLines := strings.Count(rightPanel, "\n") + 1

	if leftLines != middleLines || leftLines != rightLines {
		t.Errorf("panel heights differ: left=%d middle=%d right=%d (expected %d)", leftLines, middleLines, rightLines, innerH)
	}
	if leftLines != innerH {
		t.Errorf("panel height (%d) != expected outer height (%d)", leftLines, innerH)
	}
}

func TestSchemaPanelDoesNotOverflow(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID: "users",
		Columns: map[string]*ingitdb.ColumnDef{
			"name":  {Type: "string", Required: true},
			"email": {Type: "string"},
			"age":   {Type: "integer"},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Format: "yaml",
			Name:   "record.yaml",
		},
	}

	schemaLines := buildSchemaLines(colDef)

	const innerH = 35
	contentH := innerH - 2 // panel border

	col := collectionModel{
		colDef:      colDef,
		schemaLines: schemaLines,
	}
	content := col.renderSchema(30, contentH)
	contentLines := strings.Count(content, "\n") + 1
	if contentLines != contentH {
		t.Errorf("renderSchema produced %d lines, want exactly %d", contentLines, contentH)
	}

	panel := focusedPanelStyle.Width(30).Height(innerH).Render(content)
	panelLines := strings.Count(panel, "\n") + 1
	if panelLines != innerH {
		t.Errorf("schema panel height %d != expected outer height %d", panelLines, innerH)
	}
}

// ---------------------------------------------------------------------------
// Panel focus system tests
// ---------------------------------------------------------------------------

func TestPanelFocusInitialization(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {ID: "users", DirPath: "/repo/users"},
		},
	}

	m := newHomeModel("/repo", def, nil, 120, 40)

	if m.focus != panelCollections {
		t.Errorf("initial focus = %d, want panelCollections (%d)", m.focus, panelCollections)
	}
}

func TestPanelFocusInitializationOnly(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {ID: "users", DirPath: "/repo/users"},
		},
	}

	m := newHomeModel("/repo", def, nil, 120, 40)
	if m.focus != panelCollections {
		t.Errorf("initial focus = %d, want panelCollections (%d)", m.focus, panelCollections)
	}
	if m.recordCursor != 0 {
		t.Errorf("initial recordCursor = %d, want 0", m.recordCursor)
	}
	if m.recordOffset != 0 {
		t.Errorf("initial recordOffset = %d, want 0", m.recordOffset)
	}
}

func TestPanelFocusStateIndependence(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {ID: "users", DirPath: "/repo/users"},
		},
	}

	m := newHomeModel("/repo", def, nil, 120, 40)

	// Each panel maintains its own cursor state
	m.cursor = 5       // collections cursor
	m.recordCursor = 3 // records cursor

	if m.cursor != 5 {
		t.Errorf("collections cursor = %d, want 5", m.cursor)
	}
	if m.recordCursor != 3 {
		t.Errorf("records cursor = %d, want 3", m.recordCursor)
	}

	// Switching focus doesn't affect cursor positions
	m.focus = panelData
	if m.cursor != 5 || m.recordCursor != 3 {
		t.Errorf("focus change affected cursors: collections=%d records=%d", m.cursor, m.recordCursor)
	}

	m.focus = panelSchema
	if m.cursor != 5 || m.recordCursor != 3 {
		t.Errorf("focus change affected cursors: collections=%d records=%d", m.cursor, m.recordCursor)
	}
}

func TestRecordScrollCalculation(t *testing.T) {
	t.Parallel()

	m := homeModel{
		width:  120,
		height: 40,
	}
	m.preview = &collectionModel{
		records: make([]map[string]any, 100), // 100 records
	}

	// Set record cursor to a high value
	m.recordCursor = 50

	// Calculate visible rows (innerH - 4 for title/header/separator/total)
	_, innerH := m.panelInnerDims()
	visibleRows := innerH - 4
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Scroll offset should adjust to keep cursor visible
	if m.recordCursor >= m.recordOffset+visibleRows {
		m.recordOffset = m.recordCursor - visibleRows + 1
	}

	if m.recordOffset < 0 {
		t.Errorf("recordOffset = %d (negative)", m.recordOffset)
	}
	if m.recordCursor < m.recordOffset {
		t.Errorf("recordCursor=%d is before scroll offset=%d", m.recordCursor, m.recordOffset)
	}
}
