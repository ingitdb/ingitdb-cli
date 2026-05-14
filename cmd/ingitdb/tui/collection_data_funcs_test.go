package tui

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// computeColOffset
// ---------------------------------------------------------------------------

func TestComputeColOffset_EmptyWidths(t *testing.T) {
	t.Parallel()
	got := computeColOffset(0, 0, nil, 100)
	if got != 0 {
		t.Errorf("computeColOffset with nil widths = %d, want 0", got)
	}
}

func TestComputeColOffset_CursorBeforeOffset(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10, 10, 10}
	// current offset is 3, cursor moves back to 1 → offset snaps to cursor
	got := computeColOffset(1, 3, widths, 50)
	if got != 1 {
		t.Errorf("computeColOffset(cursor=1, offset=3) = %d, want 1", got)
	}
}

func TestComputeColOffset_CursorVisible(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10, 10, 10}
	// offset=0, cursor=1, width=50 → all cols fit, no scroll needed
	got := computeColOffset(1, 0, widths, 50)
	if got != 0 {
		t.Errorf("computeColOffset(cursor=1, offset=0, wide panel) = %d, want 0", got)
	}
}

func TestComputeColOffset_CursorScrollsRight(t *testing.T) {
	t.Parallel()
	// 5 columns of 20 each, panel width 25 → only 1 col visible at a time
	widths := []int{20, 20, 20, 20, 20}
	got := computeColOffset(4, 0, widths, 25)
	// offset should advance until col 4 is visible
	if got < 1 {
		t.Errorf("computeColOffset(cursor=4, offset=0, narrow) = %d, expected ≥1", got)
	}
}

func TestComputeColOffset_CursorNegativeClamped(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10}
	got := computeColOffset(-5, 0, widths, 100)
	if got != 0 {
		t.Errorf("computeColOffset(cursor=-5) = %d, want 0", got)
	}
}

func TestComputeColOffset_CursorBeyondLenClamped(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10}
	got := computeColOffset(99, 0, widths, 100)
	// cursor clamped to len-1 = 2, all visible at width 100 → offset 0
	if got < 0 {
		t.Errorf("computeColOffset(cursor=99) = %d, expected ≥0", got)
	}
}

// ---------------------------------------------------------------------------
// visibleColumns
// ---------------------------------------------------------------------------

func TestVisibleColumns_Empty(t *testing.T) {
	t.Parallel()
	got := visibleColumns(0, nil, 100)
	if got != nil {
		t.Errorf("visibleColumns with nil widths = %v, want nil", got)
	}
}

func TestVisibleColumns_AllFit(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10}
	got := visibleColumns(0, widths, 100)
	if len(got) != 3 {
		t.Errorf("visibleColumns = %v, want [0 1 2]", got)
	}
}

func TestVisibleColumns_OffsetSkipsFirstCols(t *testing.T) {
	t.Parallel()
	widths := []int{10, 10, 10, 10}
	got := visibleColumns(2, widths, 100)
	if len(got) < 1 || got[0] != 2 {
		t.Errorf("visibleColumns(offset=2) = %v, want first element 2", got)
	}
}

func TestVisibleColumns_NarrowWidth_AlwaysAtLeastOne(t *testing.T) {
	t.Parallel()
	widths := []int{100, 100, 100}
	got := visibleColumns(0, widths, 5)
	// Even if col width > panel width, at least one column must be returned.
	if len(got) < 1 {
		t.Errorf("visibleColumns with narrow panel returned %v, want at least 1 entry", got)
	}
	if got[0] != 0 {
		t.Errorf("visibleColumns[0] = %d, want 0", got[0])
	}
}

// ---------------------------------------------------------------------------
// discoverLocales
// ---------------------------------------------------------------------------

func TestDiscoverLocales_NoL10NFields(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	got := discoverLocales([]map[string]any{{"name": "foo"}}, colDef)
	if got != nil {
		t.Errorf("discoverLocales with no L10N fields = %v, want nil", got)
	}
}

func TestDiscoverLocales_L10NFieldFound(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"title": map[string]any{"en": "Hello", "fr": "Bonjour"}},
	}
	got := discoverLocales(records, colDef)
	if len(got) != 2 {
		t.Errorf("discoverLocales = %v, want 2 locales", got)
	}
	// Should be sorted by language name: English before French.
	if got[0] != "en" || got[1] != "fr" {
		t.Errorf("discoverLocales order = %v, want [en fr]", got)
	}
}

func TestDiscoverLocales_SkipsNonMapValues(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"title": "plain string, not a map"},
	}
	got := discoverLocales(records, colDef)
	if len(got) != 0 {
		t.Errorf("discoverLocales with non-map value = %v, want empty", got)
	}
}

func TestDiscoverLocales_MissingField(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"other_field": "value"}, // title not present
	}
	got := discoverLocales(records, colDef)
	if len(got) != 0 {
		t.Errorf("discoverLocales missing field = %v, want empty", got)
	}
}

func TestDiscoverLocales_UnknownLocaleCode(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	records := []map[string]any{
		{"title": map[string]any{"xx": "unknown locale"}},
	}
	got := discoverLocales(records, colDef)
	if len(got) != 1 || got[0] != "xx" {
		t.Errorf("discoverLocales unknown locale = %v, want [xx]", got)
	}
}

// ---------------------------------------------------------------------------
// localeIndex
// ---------------------------------------------------------------------------

func TestLocaleIndex_Found(t *testing.T) {
	t.Parallel()
	locales := []string{"en", "fr", "de"}
	got := localeIndex(locales, "fr")
	if got != 1 {
		t.Errorf("localeIndex(fr) = %d, want 1", got)
	}
}

func TestLocaleIndex_NotFound(t *testing.T) {
	t.Parallel()
	locales := []string{"en", "fr", "de"}
	got := localeIndex(locales, "xx")
	if got != 0 {
		t.Errorf("localeIndex(xx not found) = %d, want 0", got)
	}
}

func TestLocaleIndex_FirstElement(t *testing.T) {
	t.Parallel()
	locales := []string{"en", "fr", "de"}
	got := localeIndex(locales, "en")
	if got != 0 {
		t.Errorf("localeIndex(en) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// localeLanguageNames
// ---------------------------------------------------------------------------

func TestLocaleLanguageNames_ContainsKnownLocales(t *testing.T) {
	t.Parallel()
	names := localeLanguageNames()
	tests := []struct {
		code string
		want string
	}{
		{"en", "English"},
		{"fr", "French"},
		{"de", "German"},
		{"zh", "Chinese"},
	}
	for _, tc := range tests {
		got, ok := names[tc.code]
		if !ok {
			t.Errorf("localeLanguageNames missing %q", tc.code)
			continue
		}
		if got != tc.want {
			t.Errorf("localeLanguageNames[%q] = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// truncateToWidth
// ---------------------------------------------------------------------------

func TestTruncateToWidth_Empty(t *testing.T) {
	t.Parallel()
	got := truncateToWidth("", 10)
	if got != "" {
		t.Errorf("truncateToWidth empty = %q, want empty", got)
	}
}

func TestTruncateToWidth_FitsExactly(t *testing.T) {
	t.Parallel()
	got := truncateToWidth("hello", 5)
	if got != "hello" {
		t.Errorf("truncateToWidth fits exactly = %q, want hello", got)
	}
}

func TestTruncateToWidth_Truncates(t *testing.T) {
	t.Parallel()
	got := truncateToWidth("hello world", 5)
	if got != "hello" {
		t.Errorf("truncateToWidth truncates = %q, want hello", got)
	}
}

func TestTruncateToWidth_Zero(t *testing.T) {
	t.Parallel()
	got := truncateToWidth("hello", 0)
	if got != "" {
		t.Errorf("truncateToWidth zero = %q, want empty", got)
	}
}

func TestTruncateToWidth_MultibyteChars(t *testing.T) {
	t.Parallel()
	// "日本語" each char is width 2, total width 6; truncate to 4 → "日本"
	got := truncateToWidth("日本語", 4)
	if got != "日本" {
		t.Errorf("truncateToWidth multibyte = %q, want 日本", got)
	}
}

// ---------------------------------------------------------------------------
// replaceRegionalIndicators
// ---------------------------------------------------------------------------

func TestReplaceRegionalIndicators_NoIndicators(t *testing.T) {
	t.Parallel()
	got := replaceRegionalIndicators("hello")
	if got != "hello" {
		t.Errorf("replaceRegionalIndicators no-op = %q, want hello", got)
	}
}

func TestReplaceRegionalIndicators_FlagEmoji(t *testing.T) {
	t.Parallel()
	// 🇺🇸 = U+1F1FA U+1F1F8 → "US"
	input := "\U0001F1FA\U0001F1F8"
	got := replaceRegionalIndicators(input)
	if got != "US" {
		t.Errorf("replaceRegionalIndicators(🇺🇸) = %q, want US", got)
	}
}

func TestReplaceRegionalIndicators_LoneIndicator(t *testing.T) {
	t.Parallel()
	// Single regional indicator (no pair) → should map to letter
	input := "\U0001F1FA" // 🇺 alone
	got := replaceRegionalIndicators(input)
	if got != "U" {
		t.Errorf("replaceRegionalIndicators lone indicator = %q, want U", got)
	}
}

func TestReplaceRegionalIndicators_MixedContent(t *testing.T) {
	t.Parallel()
	// "flag: 🇩🇪" → "flag: DE"
	input := "flag: \U0001F1E9\U0001F1EA"
	got := replaceRegionalIndicators(input)
	if got != "flag: DE" {
		t.Errorf("replaceRegionalIndicators mixed = %q, want 'flag: DE'", got)
	}
}

// ---------------------------------------------------------------------------
// padLeft
// ---------------------------------------------------------------------------

func TestPadLeft_Pads(t *testing.T) {
	t.Parallel()
	got := padLeft("42", 6)
	if got != "    42" {
		t.Errorf("padLeft = %q, want '    42'", got)
	}
}

func TestPadLeft_NoOp(t *testing.T) {
	t.Parallel()
	got := padLeft("hello", 3)
	if got != "hello" {
		t.Errorf("padLeft already wide = %q, want hello", got)
	}
}

// ---------------------------------------------------------------------------
// buildDisplayColumns
// ---------------------------------------------------------------------------

func TestBuildDisplayColumns_L10NExpanded(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"name":  {Type: ingitdb.ColumnTypeString},
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
		ColumnsOrder: []string{"name", "title"},
	}
	got := buildDisplayColumns(colDef, "fr")
	if len(got) != 2 {
		t.Fatalf("buildDisplayColumns = %v, want 2 elements", got)
	}
	if got[0] != "name" {
		t.Errorf("got[0] = %q, want name", got[0])
	}
	if got[1] != "title.fr" {
		t.Errorf("got[1] = %q, want title.fr", got[1])
	}
}

// ---------------------------------------------------------------------------
// collectionModel.cellValue
// ---------------------------------------------------------------------------

func TestCellValue_PlainColumn(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	m := collectionModel{colDef: colDef}
	row := map[string]any{"name": "Alice"}
	got := m.cellValue(row, "name")
	if got != "Alice" {
		t.Errorf("cellValue plain = %q, want Alice", got)
	}
}

func TestCellValue_L10NColumn_Present(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	m := collectionModel{colDef: colDef}
	row := map[string]any{"title": map[string]any{"en": "Hello", "fr": "Bonjour"}}
	got := m.cellValue(row, "title.en")
	if got != "Hello" {
		t.Errorf("cellValue L10N en = %q, want Hello", got)
	}
}

func TestCellValue_L10NColumn_MissingLocale(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	m := collectionModel{colDef: colDef}
	row := map[string]any{"title": map[string]any{"en": "Hello"}}
	got := m.cellValue(row, "title.de")
	if got != "" {
		t.Errorf("cellValue L10N missing locale = %q, want empty", got)
	}
}

func TestCellValue_L10NColumn_FieldMissing(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	m := collectionModel{colDef: colDef}
	row := map[string]any{} // no "title" key
	got := m.cellValue(row, "title.en")
	if got != "" {
		t.Errorf("cellValue L10N field missing = %q, want empty", got)
	}
}

func TestCellValue_L10NColumn_WrongType(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	m := collectionModel{colDef: colDef}
	row := map[string]any{"title": "plain string"} // should be map[string]any
	got := m.cellValue(row, "title.en")
	if got != "" {
		t.Errorf("cellValue L10N wrong type = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.isL10NDisplayCol
// ---------------------------------------------------------------------------

func TestIsL10NDisplayCol_True(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeL10N},
		},
	}
	m := collectionModel{colDef: colDef}
	if !m.isL10NDisplayCol("title.en") {
		t.Error("isL10NDisplayCol(title.en) should be true")
	}
}

func TestIsL10NDisplayCol_False_NoL10N(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	m := collectionModel{colDef: colDef}
	if m.isL10NDisplayCol("name") {
		t.Error("isL10NDisplayCol(name) should be false — no dot")
	}
}

func TestIsL10NDisplayCol_False_StringColumnWithDot(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"some": {Type: ingitdb.ColumnTypeString},
		},
	}
	m := collectionModel{colDef: colDef}
	if m.isL10NDisplayCol("some.thing") {
		t.Error("isL10NDisplayCol(some.thing) should be false — base col is string not L10N")
	}
}

func TestIsL10NDisplayCol_False_UnknownBase(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		Columns: map[string]*ingitdb.ColumnDef{},
	}
	m := collectionModel{colDef: colDef}
	if m.isL10NDisplayCol("unknown.en") {
		t.Error("isL10NDisplayCol(unknown.en) should be false — base col not in def")
	}
}

// ---------------------------------------------------------------------------
// collectionModel.buildLocaleDropdownLines
// ---------------------------------------------------------------------------

func TestBuildLocaleDropdownLines_Basic(t *testing.T) {
	t.Parallel()
	m := collectionModel{
		locales:              []string{"en", "fr"},
		localeDropdownCursor: 0,
	}
	lines := m.buildLocaleDropdownLines()
	// expect: top border + 2 rows + bottom border = 4 lines
	if len(lines) != 4 {
		t.Fatalf("buildLocaleDropdownLines = %d lines, want 4", len(lines))
	}
	if !strings.HasPrefix(lines[0], "┌") {
		t.Errorf("first line should be top border, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Errorf("last line should be bottom border, got %q", lines[len(lines)-1])
	}
}

func TestBuildLocaleDropdownLines_CursorMarked(t *testing.T) {
	t.Parallel()
	m := collectionModel{
		locales:              []string{"en", "fr", "de"},
		localeDropdownCursor: 1,
	}
	lines := m.buildLocaleDropdownLines()
	// row 1 (index 1 in lines, which is "en") → plain, row 2 (fr) → selected
	// Strip ANSI to check content.
	row1Plain := stripAnsi(lines[1])
	row2Plain := stripAnsi(lines[2])
	if !strings.Contains(row1Plain, "  ") { // prefix "  " (not ►)
		t.Errorf("row1 not-selected prefix wrong: %q", row1Plain)
	}
	if !strings.Contains(row2Plain, "► ") {
		t.Errorf("row2 cursor marker missing in %q", row2Plain)
	}
}

func TestBuildLocaleDropdownLines_UnknownLocale(t *testing.T) {
	t.Parallel()
	m := collectionModel{
		locales:              []string{"xx"}, // not in language names map
		localeDropdownCursor: 0,
	}
	lines := m.buildLocaleDropdownLines()
	if len(lines) != 3 {
		t.Fatalf("buildLocaleDropdownLines unknown locale = %d lines, want 3", len(lines))
	}
	// The row should contain "xx" since that's the fallback name.
	row := stripAnsi(lines[1])
	if !strings.Contains(row, "xx") {
		t.Errorf("row should contain locale code xx, got %q", row)
	}
}

// ---------------------------------------------------------------------------
// collectionModel.computeColWidths
// ---------------------------------------------------------------------------

func TestComputeColWidths_EmptyColumns(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		Columns: map[string]*ingitdb.ColumnDef{},
	}
	m := collectionModel{colDef: colDef, columns: []string{}}
	got := m.computeColWidths()
	if got != nil {
		t.Errorf("computeColWidths empty = %v, want nil", got)
	}
}

func TestComputeColWidths_CapAt30(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "things",
		Columns: map[string]*ingitdb.ColumnDef{
			"desc": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"desc"},
	}
	m := collectionModel{
		colDef:  colDef,
		columns: []string{"desc"},
		records: []map[string]any{
			{"desc": strings.Repeat("x", 50)}, // 50 chars > cap 30
		},
	}
	got := m.computeColWidths()
	if len(got) != 1 || got[0] != 30 {
		t.Errorf("computeColWidths cap = %v, want [30]", got)
	}
}

// ---------------------------------------------------------------------------
// buildSchemaLines
// ---------------------------------------------------------------------------

func TestBuildSchemaLines_WithRecordFile(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "users",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString, Required: true},
		},
		RecordFile: &ingitdb.RecordFileDef{
			Format: "yaml",
			Name:   "record.yaml",
		},
	}
	lines := buildSchemaLines(colDef)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(stripAnsi(joined), "format") {
		t.Errorf("buildSchemaLines should include 'format' when RecordFile set; got %q", joined)
	}
	if !strings.Contains(stripAnsi(joined), "required") {
		t.Errorf("buildSchemaLines should include 'required' for required columns; got %q", joined)
	}
}

func TestBuildSchemaLines_WithSubCollections(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "users",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"addresses": {ID: "addresses"},
		},
	}
	lines := buildSchemaLines(colDef)
	joined := stripAnsi(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "addresses") {
		t.Errorf("buildSchemaLines should include sub-collection 'addresses'; got %q", joined)
	}
	if !strings.Contains(joined, "Sub-collections") {
		t.Errorf("buildSchemaLines should include 'Sub-collections' header; got %q", joined)
	}
}

func TestBuildSchemaLines_NoRecordFile(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "users",
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	// Should not panic when RecordFile is nil.
	lines := buildSchemaLines(colDef)
	if len(lines) == 0 {
		t.Error("buildSchemaLines should return at least schema header lines")
	}
}

// ---------------------------------------------------------------------------
// wordWrap
// ---------------------------------------------------------------------------

func TestWordWrap_NoWrap(t *testing.T) {
	t.Parallel()
	got := wordWrap("hello world", 100)
	if got != "hello world" {
		t.Errorf("wordWrap no-wrap = %q, want 'hello world'", got)
	}
}

func TestWordWrap_Wraps(t *testing.T) {
	t.Parallel()
	got := wordWrap("hello world foo bar", 10)
	if !strings.Contains(got, "\n") {
		t.Errorf("wordWrap should insert newline; got %q", got)
	}
}

func TestWordWrap_ZeroWidth(t *testing.T) {
	t.Parallel()
	got := wordWrap("hello", 0)
	if got != "hello" {
		t.Errorf("wordWrap zero width = %q, want hello", got)
	}
}

func TestWordWrap_Empty(t *testing.T) {
	t.Parallel()
	got := wordWrap("", 10)
	if got != "" {
		t.Errorf("wordWrap empty = %q, want empty", got)
	}
}
