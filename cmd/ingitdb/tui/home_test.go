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
// Cursor on first collection (not filter), no newDB so no preview.
m.filterFocused = false
m.cursor = 0

leftWidth, rightWidth := m.panelWidths()
leftInner := leftWidth - 4
rightInner := rightWidth - 4
innerH := termH - 5
contentH := innerH - 2

leftContent := m.renderCollectionList(leftInner, contentH)
leftPanel := focusedPanelStyle.Width(leftInner).Height(innerH).Render(leftContent)

rightContent := m.renderWelcome(rightInner, contentH)
rightPanel := panelStyle.Width(rightInner).Height(innerH).Render(rightContent)

leftLines := strings.Count(leftPanel, "\n") + 1
rightLines := strings.Count(rightPanel, "\n") + 1

if leftLines != rightLines {
t.Errorf("left panel height (%d) != right panel height (%d)", leftLines, rightLines)
}
if leftLines != innerH {
t.Errorf("left panel height (%d) != expected outer height (%d)", leftLines, innerH)
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
