package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dal-go/dalgo/dal"
	"github.com/rivo/uniseg"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// recordsLoadedMsg carries all records loaded from disk for a collection.
type recordsLoadedMsg struct {
	records []map[string]any
	keys    []string
	err     error
}

// collectionModel renders the collection detail screen:
// narrow left column (schema) + wide right panel (records).
type collectionModel struct {
	colDef  *ingitdb.CollectionDef
	db      dal.DB
	width   int
	height  int
	loading bool

	// left panel: schema lines + scroll
	schemaLines  []string
	schemaOffset int

	// right panel: records + scroll
	records      []map[string]any
	recordKeys   []string
	recordCursor int
	recordOffset int
	columns      []string

	// locale handling for map[locale]string columns
	locale  string   // currently selected locale (e.g. "en")
	locales []string // available locales sorted by full language name

	// locale dropdown state
	localeDropdownOpen   bool
	localeDropdownCursor int
}

func newCollectionModel(colDef *ingitdb.CollectionDef, db dal.DB, width, height int) collectionModel {
	return collectionModel{
		colDef:      colDef,
		db:          db,
		width:       width,
		height:      height,
		loading:     true,
		schemaLines: buildSchemaLines(colDef),
		columns:     orderedColumns(colDef),
	}
}

func (m collectionModel) Init() tea.Cmd {
	return loadRecordsCmd(m.db, m.colDef)
}

func (m collectionModel) Update(msg tea.Msg) (collectionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case recordsLoadedMsg:
		m.loading = false
		m.records = msg.records
		m.recordKeys = msg.keys
		m.locales = discoverLocales(m.records, m.colDef)
		if len(m.locales) > 0 {
			m.locale = m.locales[0]
			for _, loc := range m.locales {
				if loc == "en" {
					m.locale = "en"
					break
				}
			}
			m.columns = buildDisplayColumns(m.colDef, m.locale)
		}

	case tea.KeyPressMsg:
		_, innerH := m.panelInnerDims()
		_ = innerH
		switch msg.String() {
		case "up", "k":
			if m.localeDropdownOpen {
				if m.localeDropdownCursor > 0 {
					m.localeDropdownCursor--
				}
			} else if m.recordCursor > 0 {
				m.recordCursor--
				if m.recordCursor < m.recordOffset {
					m.recordOffset = m.recordCursor
				}
			} else if len(m.locales) > 1 {
				// At top row: give focus to locale selector dropdown.
				m.localeDropdownOpen = true
				m.localeDropdownCursor = localeIndex(m.locales, m.locale)
			}
		case "down", "j":
			if m.localeDropdownOpen {
				if m.localeDropdownCursor < len(m.locales)-1 {
					m.localeDropdownCursor++
				}
			} else if m.recordCursor < len(m.records)-1 {
				m.recordCursor++
				visibleRows := innerH - 4 // title + header + separator + total line
				if m.recordCursor >= m.recordOffset+visibleRows {
					m.recordOffset = m.recordCursor - visibleRows + 1
				}
			}
		case "l", "L":
			if len(m.locales) > 1 && !m.localeDropdownOpen {
				m.localeDropdownOpen = true
				m.localeDropdownCursor = localeIndex(m.locales, m.locale)
			}
		case "enter":
			if m.localeDropdownOpen {
				m.locale = m.locales[m.localeDropdownCursor]
				m.columns = buildDisplayColumns(m.colDef, m.locale)
				m.localeDropdownOpen = false
			}
		case "esc":
			if m.localeDropdownOpen {
				m.localeDropdownOpen = false
			}
		}
	}
	return m, nil
}

func (m collectionModel) View() string {
	leftW, rightW := collectionPanelWidths(m.width)
	leftInner := leftW - 4
	rightInner := rightW - 4
	_, innerH := m.panelInnerDims()

	contentH := innerH - 2 // panel border consumes 2 rows
	leftContent := m.renderSchema(leftInner, contentH)
	rightContent := m.renderRecords(rightInner, contentH)

	// In lipgloss v2, Width() is the total outer (border-box) width.
	// Content area = Width - border(2) - padding(2) = leftW/rightW - 4 = inner widths.
	// Collection screen: data panel (right) focused by default, schema panel (left) unfocused.
	left := panelStyle.Width(leftW).Height(innerH).Render(leftContent)
	right := focusedPanelStyle.Width(rightW).Height(innerH).Render(rightContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := helpStyle.Render(" ↑/↓ navigate  l locale  enter select  esc back  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

func (m collectionModel) panelInnerDims() (width, height int) {
	leftW, _ := collectionPanelWidths(m.width)
	return leftW - 4, m.height - 2 // header + help bar
}

func (m collectionModel) renderSchema(width, height int) string {
	// Expand logical lines: sectionTitleStyle embeds \n for its
	// bottom border; split to physical lines so we slice exactly height rows.
	var physical []string
	for _, l := range m.schemaLines {
		physical = append(physical, strings.Split(l, "\n")...)
	}
	start := m.schemaOffset
	if start > len(physical) {
		start = len(physical)
	}
	end := start + height
	if end > len(physical) {
		end = len(physical)
	}
	visible := physical[start:end]
	for len(visible) < height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

func (m collectionModel) renderRecords(width, height int) string {
	if m.loading {
		return mutedStyle.Render("Loading records…")
	}
	if len(m.records) == 0 {
		return mutedStyle.Render("(no records)")
	}
	cols := m.columns
	if len(cols) == 0 {
		return mutedStyle.Render("(no columns defined)")
	}

	colWidths := make([]int, len(cols))
	for i, c := range cols {
		colWidths[i] = uniseg.StringWidth(c)
	}
	for _, row := range m.records {
		for i, c := range cols {
			raw := m.cellValue(row, c)
			v := replaceRegionalIndicators(raw)
			if w := uniseg.StringWidth(v); w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}
	for i := range colWidths {
		if colWidths[i] > 30 {
			colWidths[i] = 30
		}
	}

	// Build all content lines as a slice so the dropdown can be overlaid.
	var lines []string

	// Line 0: collection name (top-left) + locale selector (top-right).
	// Use a plain bold style here — sectionTitleStyle has BorderBottom which
	// would push the locale label onto the underline row, not the title row.
	titleStr := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Render(m.colDef.ID)
	if m.locale != "" {
		arrow := " ▼"
		if m.localeDropdownOpen {
			arrow = " ▲"
		}
		localeLabel := columnKeyStyle.Render("[ " + m.locale + arrow + " ]")
		titleW := lipgloss.Width(titleStr)
		localeW := lipgloss.Width(localeLabel)
		gap := width - titleW - localeW
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, titleStr+strings.Repeat(" ", gap)+localeLabel)
	} else {
		lines = append(lines, titleStr)
	}

	// Line 1: title underline (drawn manually, matching sectionTitleStyle).
	borderW := width
	if borderW < 1 {
		borderW = 1
	}
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", borderW)))

	// Line 2: column headers.
	headerCells := make([]string, len(cols))
	for i, c := range cols {
		headerCells[i] = columnKeyStyle.Render(padRight(c, colWidths[i]))
	}
	lines = append(lines, strings.Join(headerCells, " │ "))

	// Line 3: separator.
	seps := make([]string, len(cols))
	for i, w := range colWidths {
		seps[i] = strings.Repeat("─", w)
	}
	lines = append(lines, mutedStyle.Render(strings.Join(seps, "─┼─")))

	// Data rows (lines 4+).
	visibleRows := height - 4 // title + border + header + separator
	if visibleRows < 1 {
		visibleRows = 1
	}
	end := m.recordOffset + visibleRows
	if end > len(m.records) {
		end = len(m.records)
	}
	numericCol := make([]bool, len(cols))
	if len(m.records) > 0 {
		for i, c := range cols {
			if m.isL10NDisplayCol(c) {
				numericCol[i] = false
			} else {
				numericCol[i] = isNumeric(m.records[0][c])
			}
		}
	}
	for ri := m.recordOffset; ri < end; ri++ {
		row := m.records[ri]
		cells := make([]string, len(cols))
		for i, c := range cols {
			raw := m.cellValue(row, c)
			v := replaceRegionalIndicators(raw)
			if uniseg.StringWidth(v) > colWidths[i] {
				v = truncateToWidth(v, colWidths[i]-1) + "…"
			}
			if numericCol[i] {
				cells[i] = padLeft(v, colWidths[i])
			} else {
				cells[i] = padRight(v, colWidths[i])
			}
		}
		line := strings.Join(cells, " │ ")
		if ri == m.recordCursor {
			line = selectedItemStyle.Render(line)
		}
		lines = append(lines, line)
	}

	// Blank line + record count (matches the old "\n%d record(s)" pattern).
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d record(s)", len(m.records))))

	// Build dropdown lines (if open); they will overlay the table, not push it down.
	var dropLines []string
	if m.localeDropdownOpen && len(m.locales) > 1 {
		dropLines = m.buildLocaleDropdownLines()
	}

	// Merge: overlay the dropdown on the right side starting at line 1
	// (just below the title row, aligned with the locale selector button).
	const dropStart = 1
	dropW := 0
	if len(dropLines) > 0 {
		dropW = lipgloss.Width(dropLines[0])
	}

	var sb strings.Builder
	for i, line := range lines {
		di := i - dropStart
		if di >= 0 && di < len(dropLines) {
			// Paint dropdown on the right; show plain table text on the left.
			leftW := width - dropW
			if leftW < 0 {
				leftW = 0
			}
			plain := stripAnsi(line)
			left := truncateToWidth(plain, leftW)
			lw := uniseg.StringWidth(left)
			if lw < leftW {
				left += strings.Repeat(" ", leftW-lw)
			}
			sb.WriteString(left + dropLines[di])
		} else {
			sb.WriteString(line)
		}
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// buildLocaleDropdownLines returns each line of the locale dropdown box as a
// plain string (no trailing newline). The box is sized to fit the widest item.
func (m collectionModel) buildLocaleDropdownLines() []string {
	names := localeLanguageNames()

	// Determine inner width from the widest item.
	innerW := 0
	for _, loc := range m.locales {
		name, ok := names[loc]
		if !ok {
			name = loc
		}
		w := uniseg.StringWidth("► " + name + " (" + loc + ")")
		if w > innerW {
			innerW = w
		}
	}
	if innerW < 10 {
		innerW = 10
	}

	lines := make([]string, 0, len(m.locales)+2)
	lines = append(lines, "┌"+strings.Repeat("─", innerW)+"┐")
	for i, loc := range m.locales {
		name, ok := names[loc]
		if !ok {
			name = loc
		}
		prefix := "  "
		if i == m.localeDropdownCursor {
			prefix = "► "
		}
		content := prefix + name + " (" + loc + ")"
		cw := uniseg.StringWidth(content)
		if cw < innerW {
			content += strings.Repeat(" ", innerW-cw)
		}
		row := "│" + content + "│"
		if i == m.localeDropdownCursor {
			row = selectedItemStyle.Render(row)
		}
		lines = append(lines, row)
	}
	lines = append(lines, "└"+strings.Repeat("─", innerW)+"┘")
	return lines
}

// stripAnsi removes ANSI escape sequences from s so visual-width calculations
// and truncation work correctly on plain text.
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// buildSchemaLines pre-renders schema as a slice of styled strings.
func buildSchemaLines(colDef *ingitdb.CollectionDef) []string {
	var lines []string

	lines = append(lines, sectionTitleStyle.Render("Schema"))
	lines = append(lines, columnKeyStyle.Render("id: ")+colDef.ID)

	if rf := colDef.RecordFile; rf != nil {
		lines = append(lines, columnKeyStyle.Render("format: ")+string(rf.Format))
		lines = append(lines, columnKeyStyle.Render("file: ")+rf.Name)
	}

	lines = append(lines, "")
	lines = append(lines, sectionTitleStyle.Render("Columns"))

	for _, name := range orderedColumns(colDef) {
		col := colDef.Columns[name]
		lines = append(lines, columnKeyStyle.Render(name))
		lines = append(lines, columnTypeStyle.Render(fmt.Sprintf("  type: %s", col.Type)))
		if col.Required {
			lines = append(lines, columnTypeStyle.Render("  required"))
		}
	}

	if len(colDef.SubCollections) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionTitleStyle.Render("Sub-collections"))
		subs := make([]string, 0, len(colDef.SubCollections))
		for id := range colDef.SubCollections {
			subs = append(subs, id)
		}
		sort.Strings(subs)
		for _, id := range subs {
			lines = append(lines, itemStyle.Render("  "+id))
		}
	}

	return lines
}

// loadRecordsCmd returns a Tea command that reads all records for the collection.
func loadRecordsCmd(db dal.DB, colDef *ingitdb.CollectionDef) tea.Cmd {
	return func() tea.Msg {
		colID := colDef.ID
		qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(colID, "")))
		q := qb.SelectIntoRecord(func() dal.Record {
			key := dal.NewKeyWithID(colID, "")
			return dal.NewRecordWithData(key, map[string]any{})
		})

		var (
			records []map[string]any
			keys    []string
		)
		err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
			reader, txErr := tx.ExecuteQueryToRecordsReader(ctx, q)
			if txErr != nil {
				return txErr
			}
			defer func() { _ = reader.Close() }()
			for {
				rec, nextErr := reader.Next()
				if nextErr != nil {
					break
				}
				data := rec.Data().(map[string]any)
				keys = append(keys, fmt.Sprintf("%v", rec.Key().ID))
				records = append(records, data)
			}
			return nil
		})
		return recordsLoadedMsg{records: records, keys: keys, err: err}
	}
}

// orderedColumns returns columns in ColumnsOrder, then any remaining sorted.
func orderedColumns(colDef *ingitdb.CollectionDef) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(colDef.Columns))
	for _, c := range colDef.ColumnsOrder {
		if _, ok := colDef.Columns[c]; ok {
			result = append(result, c)
			seen[c] = true
		}
	}
	remaining := make([]string, 0)
	for c := range colDef.Columns {
		if !seen[c] {
			remaining = append(remaining, c)
		}
	}
	sort.Strings(remaining)
	return append(result, remaining...)
}

func collectionPanelWidths(totalWidth int) (left, right int) {
	left = totalWidth / 4
	if left < 24 {
		left = 24
	}
	right = totalWidth - left
	return
}

// buildDisplayColumns returns display column names, expanding L10N columns
// to "field.locale" format for the selected locale.
func buildDisplayColumns(colDef *ingitdb.CollectionDef, locale string) []string {
	base := orderedColumns(colDef)
	result := make([]string, 0, len(base))
	for _, name := range base {
		col := colDef.Columns[name]
		if col.Type == ingitdb.ColumnTypeL10N {
			result = append(result, name+"."+locale)
		} else {
			result = append(result, name)
		}
	}
	return result
}

// discoverLocales scans all records for map[locale]string columns and returns
// the set of available locale codes sorted by full language name.
func discoverLocales(records []map[string]any, colDef *ingitdb.CollectionDef) []string {
	l10nFields := make([]string, 0)
	for name, col := range colDef.Columns {
		if col.Type == ingitdb.ColumnTypeL10N {
			l10nFields = append(l10nFields, name)
		}
	}
	if len(l10nFields) == 0 {
		return nil
	}
	localeSet := make(map[string]bool)
	for _, rec := range records {
		for _, field := range l10nFields {
			val, ok := rec[field]
			if !ok {
				continue
			}
			localeMap, ok := val.(map[string]any)
			if !ok {
				continue
			}
			for k := range localeMap {
				localeSet[k] = true
			}
		}
	}
	locales := make([]string, 0, len(localeSet))
	for k := range localeSet {
		locales = append(locales, k)
	}
	names := localeLanguageNames()
	sort.Slice(locales, func(i, j int) bool {
		ni, oki := names[locales[i]]
		if !oki {
			ni = locales[i]
		}
		nj, okj := names[locales[j]]
		if !okj {
			nj = locales[j]
		}
		return ni < nj
	})
	return locales
}

// localeLanguageNames returns a map from locale code to full language name
// used for sorting locales alphabetically by language name.
func localeLanguageNames() map[string]string {
	return map[string]string{
		"ar": "Arabic",
		"cs": "Czech",
		"da": "Danish",
		"de": "German",
		"en": "English",
		"es": "Spanish",
		"fi": "Finnish",
		"fr": "French",
		"hi": "Hindi",
		"it": "Italian",
		"ja": "Japanese",
		"ko": "Korean",
		"nb": "Norwegian",
		"nl": "Dutch",
		"pl": "Polish",
		"pt": "Portuguese",
		"ru": "Russian",
		"sv": "Swedish",
		"tr": "Turkish",
		"uk": "Ukrainian",
		"zh": "Chinese",
	}
}

// localeIndex returns the index of locale in locales, or 0 if not found.
func localeIndex(locales []string, locale string) int {
	for i, l := range locales {
		if l == locale {
			return i
		}
	}
	return 0
}

// cellValue extracts the display value for a column from a record row.
// For L10N columns (display name "field.locale"), it extracts the nested locale value.
func (m collectionModel) cellValue(row map[string]any, displayCol string) string {
	dotIdx := strings.Index(displayCol, ".")
	if dotIdx > 0 {
		baseField := displayCol[:dotIdx]
		locale := displayCol[dotIdx+1:]
		col, ok := m.colDef.Columns[baseField]
		if ok && col.Type == ingitdb.ColumnTypeL10N {
			val, exists := row[baseField]
			if !exists {
				return ""
			}
			localeMap, ok := val.(map[string]any)
			if !ok {
				return ""
			}
			localeVal, exists := localeMap[locale]
			if !exists {
				return ""
			}
			return fmt.Sprintf("%v", localeVal)
		}
	}
	return fmt.Sprintf("%v", row[displayCol])
}

// isL10NDisplayCol returns true if the display column name corresponds to an
// expanded L10N column (i.e. "field.locale" where field has type map[locale]string).
func (m collectionModel) isL10NDisplayCol(displayCol string) bool {
	dotIdx := strings.Index(displayCol, ".")
	if dotIdx <= 0 {
		return false
	}
	baseField := displayCol[:dotIdx]
	col, ok := m.colDef.Columns[baseField]
	if !ok {
		return false
	}
	return col.Type == ingitdb.ColumnTypeL10N
}

func padRight(s string, width int) string {
	w := uniseg.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func padLeft(s string, width int) string {
	w := uniseg.StringWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

func isNumeric(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	}
	return false
}

// truncateToWidth truncates s to fit within maxWidth terminal columns.
func truncateToWidth(s string, maxWidth int) string {
	var (
		result  strings.Builder
		w       int
		cluster string
		rest    = s
		width   int
	)
	for rest != "" {
		cluster, rest, width, _ = uniseg.FirstGraphemeClusterInString(rest, -1)
		if w+width > maxWidth {
			break
		}
		result.WriteString(cluster)
		w += width
	}
	return result.String()
}

// replaceRegionalIndicators converts Regional Indicator Symbol pairs (flag emoji,
// e.g. 🇺🇸) into their two-letter ASCII country codes (e.g. "US") so they
// render predictably in terminals that lack emoji support.
func replaceRegionalIndicators(s string) string {
	const base = 0x1F1E6 // U+1F1E6 = 🇦  maps to 'A'
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			// First indicator of a pair.
			if i+1 < len(runes) && runes[i+1] >= 0x1F1E6 && runes[i+1] <= 0x1F1FF {
				b.WriteByte(byte('A' + (r - base)))
				b.WriteByte(byte('A' + (runes[i+1] - base)))
				i++ // consume second indicator
				continue
			}
			// Lone indicator — keep as letter.
			b.WriteByte(byte('A' + (r - base)))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
