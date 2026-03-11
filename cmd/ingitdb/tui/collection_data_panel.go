package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

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

	// Use cached widths when available; recompute otherwise.
	colWidths := m.colWidths
	if len(colWidths) != len(cols) {
		colWidths = m.computeColWidths()
	}

	// Determine which columns are visible given the horizontal scroll offset.
	colOffset := m.colOffset
	if colOffset >= len(cols) {
		colOffset = 0
	}
	visIdx := visibleColumns(colOffset, colWidths, width)

	// Build all content lines as a slice so the dropdown can be overlaid.
	var lines []string

	// Line 0: collection name (top-left) + locale selector (top-right).
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

	// Line 1: title underline.
	borderW := width
	if borderW < 1 {
		borderW = 1
	}
	lines = append(lines, mutedStyle.Render(strings.Repeat("─", borderW)))

	// Line 2: column headers (visible columns only, no wrapping).
	headerCells := make([]string, len(visIdx))
	for vi, i := range visIdx {
		headerCells[vi] = columnKeyStyle.Render(padRight(cols[i], colWidths[i]))
	}
	headerLine := strings.Join(headerCells, " │ ")
	if colOffset > 0 {
		headerLine = mutedStyle.Render("◀ ") + headerLine
	}
	if visIdx[len(visIdx)-1] < len(cols)-1 {
		headerLine = headerLine + mutedStyle.Render(" ▶")
	}
	lines = append(lines, headerLine)

	// Line 3: separator (visible columns only).
	seps := make([]string, len(visIdx))
	for vi, i := range visIdx {
		seps[vi] = strings.Repeat("─", colWidths[i])
	}
	sepLine := strings.Join(seps, "─┼─")
	if colOffset > 0 {
		sepLine = "──" + sepLine
	}
	if visIdx[len(visIdx)-1] < len(cols)-1 {
		sepLine = sepLine + "──"
	}
	lines = append(lines, mutedStyle.Render(sepLine))

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
		cells := make([]string, len(visIdx))
		for vi, i := range visIdx {
			c := cols[i]
			raw := m.cellValue(row, c)
			v := replaceRegionalIndicators(raw)
			if uniseg.StringWidth(v) > colWidths[i] {
				v = truncateToWidth(v, colWidths[i]-1) + "…"
			}
			var cell string
			if numericCol[i] {
				cell = padLeft(v, colWidths[i])
			} else {
				cell = padRight(v, colWidths[i])
			}
			if ri == m.recordCursor && i == m.colCursor {
				cell = selectedItemStyle.Render(cell)
			}
			cells[vi] = cell
		}
		line := strings.Join(cells, " │ ")
		if colOffset > 0 {
			line = "  " + line
		}
		if visIdx[len(visIdx)-1] < len(cols)-1 {
			line = line + "  "
		}
		lines = append(lines, line)
	}

	// Blank line + record count.
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

// computeColWidths calculates the display width for each column, capped at 30,
// considering both column names and all record values.
func (m collectionModel) computeColWidths() []int {
	cols := m.columns
	if len(cols) == 0 {
		return nil
	}
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = uniseg.StringWidth(c)
	}
	for _, row := range m.records {
		for i, c := range cols {
			raw := m.cellValue(row, c)
			v := replaceRegionalIndicators(raw)
			if w := uniseg.StringWidth(v); w > widths[i] {
				widths[i] = w
			}
		}
	}
	for i := range widths {
		if widths[i] > 30 {
			widths[i] = 30
		}
	}
	return widths
}

// visibleColumns returns the slice of column indices that fit within width,
// starting from colOffset. At least one column is always included.
func visibleColumns(colOffset int, colWidths []int, width int) []int {
	if len(colWidths) == 0 {
		return nil
	}
	result := []int{}
	used := 0
	for i := colOffset; i < len(colWidths); i++ {
		extra := 0
		if len(result) > 0 {
			extra = 3 // " │ "
		}
		// Always include the first column even if it exceeds width.
		if len(result) > 0 && used+extra+colWidths[i] > width {
			break
		}
		used += extra + colWidths[i]
		result = append(result, i)
	}
	return result
}

// computeColOffset adjusts the horizontal scroll offset so that colCursor
// remains visible within the given panel width.
func computeColOffset(colCursor, curOffset int, colWidths []int, width int) int {
	n := len(colWidths)
	if n == 0 {
		return 0
	}
	if colCursor < 0 {
		colCursor = 0
	}
	if colCursor >= n {
		colCursor = n - 1
	}

	// Snap left if cursor moved before current offset.
	offset := curOffset
	if colCursor < offset {
		return colCursor
	}

	// Advance offset until colCursor is within the visible range.
	for offset < colCursor {
		vis := visibleColumns(offset, colWidths, width)
		if len(vis) == 0 {
			break
		}
		if colCursor <= vis[len(vis)-1] {
			break
		}
		offset++
	}
	return offset
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
