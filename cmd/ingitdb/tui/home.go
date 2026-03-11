package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

const (
	githubURL  = "https://github.com/ingitdb/ingitdb-cli"
	addBtnText = "+ Add collection"
)

// homeModel is the main screen showing collection list and welcome/preview panel.
type homeModel struct {
	dbPath              string
	def                 *ingitdb.Definition
	newDB               func(string, *ingitdb.Definition) (dal.DB, error)
	collections         []collectionEntry // sorted list of all collections
	filteredCollections []collectionEntry // subset of collections matching filterValue
	filterValue         string
	filterFocused       bool
	cursor              int // 0..len(filteredCollections) inclusive (+1 for add button)
	width               int
	height              int

	// right-panel preview for the currently selected collection
	preview   *collectionModel
	previewID string
}

type collectionEntry struct {
	id      string
	dirPath string
}

// newHomeModel constructs a homeModel and pre-creates a preview for cursor 0 (if any).
func newHomeModel(dbPath string, def *ingitdb.Definition, newDB func(string, *ingitdb.Definition) (dal.DB, error), width, height int) homeModel {
	entries := make([]collectionEntry, 0, len(def.Collections))
	for id, c := range def.Collections {
		entries = append(entries, collectionEntry{id: id, dirPath: c.DirPath})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	m := homeModel{
		dbPath:         dbPath,
		def:            def,
		newDB:          newDB,
		collections:    entries,
		filterFocused:  true,
		width:          width,
		height:         height,
	}
	m.filteredCollections = m.applyFilter()
	if len(m.filteredCollections) > 0 && newDB != nil {
		m.buildPreview(0)
	}
	return m
}

// applyFilter returns the entries from m.collections that match m.filterValue
// (case-insensitive substring on entry.id).
func (m homeModel) applyFilter() []collectionEntry {
	if m.filterValue == "" {
		result := make([]collectionEntry, len(m.collections))
		copy(result, m.collections)
		return result
	}
	lower := strings.ToLower(m.filterValue)
	filtered := make([]collectionEntry, 0)
	for _, e := range m.collections {
		if strings.Contains(strings.ToLower(e.id), lower) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// buildPreview constructs a collectionModel for filteredCollections[i] and
// stores it in m.preview / m.previewID. It does NOT fire the load command.
func (m *homeModel) buildPreview(i int) {
	if i >= len(m.filteredCollections) || m.newDB == nil {
		return
	}
	entry := m.filteredCollections[i]
	colDef := m.def.Collections[entry.id]
	if colDef == nil {
		return
	}
	db, err := m.newDB(m.dbPath, m.def)
	if err != nil {
		return
	}
	col := newCollectionModel(colDef, db, m.width, m.height)
	m.preview = &col
	m.previewID = entry.id
}

// Init returns the command that loads records for the initial preview (if any).
func (m homeModel) Init() tea.Cmd {
	if m.preview != nil {
		return m.preview.Init()
	}
	return nil
}

// refreshPreview ensures m.preview is up-to-date for the current cursor position
// and returns a Cmd to load records when the collection has changed.
func (m *homeModel) refreshPreview() tea.Cmd {
	if m.cursor >= len(m.filteredCollections) {
		m.preview = nil
		m.previewID = ""
		return nil
	}
	entry := m.filteredCollections[m.cursor]
	if entry.id == m.previewID && m.preview != nil {
		return nil // already loaded
	}
	m.buildPreview(m.cursor)
	if m.preview == nil {
		return nil
	}
	return m.preview.Init()
}

func (m homeModel) Update(msg tea.Msg) (homeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case recordsLoadedMsg:
		if m.preview != nil {
			updated, cmd := m.preview.Update(msg)
			m.preview = &updated
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.preview != nil {
			updated, _ := m.preview.Update(msg)
			m.preview = &updated
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			m.filterFocused = !m.filterFocused
			return m, nil

		case "backspace":
			if m.filterFocused && len(m.filterValue) > 0 {
				runes := []rune(m.filterValue)
				m.filterValue = string(runes[:len(runes)-1])
				m.filteredCollections = m.applyFilter()
				if m.cursor > len(m.filteredCollections) {
					m.cursor = len(m.filteredCollections)
				}
				cmd := m.refreshPreview()
				return m, cmd
			}

		case "up", "k":
			if m.filterFocused {
				// already at filter, nothing above
			} else if m.cursor == 0 {
				m.filterFocused = true
			} else {
				m.cursor--
			}
			cmd := m.refreshPreview()
			return m, cmd

		case "down", "j":
			if m.filterFocused {
				if len(m.filteredCollections) > 0 {
					m.filterFocused = false
					m.cursor = 0
				}
			} else if m.cursor < len(m.filteredCollections) {
				m.cursor++
			}
			cmd := m.refreshPreview()
			return m, cmd

		default:
			if m.filterFocused {
				key := msg.String()
				runes := []rune(key)
				if len(runes) == 1 && runes[0] >= 32 && runes[0] != 127 {
					m.filterValue += string(runes[0])
					m.filteredCollections = m.applyFilter()
					if m.cursor > len(m.filteredCollections) {
						m.cursor = len(m.filteredCollections)
					}
					cmd := m.refreshPreview()
					return m, cmd
				}
			}
		}
	}
	return m, nil
}

// SelectedCollection returns the CollectionDef at the cursor, or nil if the
// cursor is on the add-button row.
func (m homeModel) SelectedCollection() *ingitdb.CollectionDef {
	if m.cursor < len(m.filteredCollections) {
		entry := m.filteredCollections[m.cursor]
		return m.def.Collections[entry.id]
	}
	return nil
}

func (m homeModel) View() string {
	leftWidth, rightWidth := m.panelWidths()
	leftInner := leftWidth - 4
	rightInner := rightWidth - 4
	innerH := m.height - 2 // header + help bar
	contentH := innerH - 2 // panel border consumes 2 rows

	leftContent := m.renderCollectionList(leftInner, contentH)
	left := focusedPanelStyle.Width(leftInner).Height(innerH).Render(leftContent)

	var right string
	if !m.filterFocused && m.cursor < len(m.filteredCollections) && m.preview != nil {
		right = m.renderPreview(rightWidth, innerH)
	} else {
		rightContent := m.renderWelcome(rightInner, contentH)
		right = panelStyle.Width(rightInner).Height(innerH).Render(rightContent)
	}

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := helpStyle.Render(" ↑/↓ navigate  tab filter  enter open  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

// renderPreview renders the schema and records panels for the preview collection
// side by side within totalWidth.
func (m homeModel) renderPreview(totalWidth, height int) string {
	schemaOuter := totalWidth / 3
	if schemaOuter < 24 {
		schemaOuter = 24
	}
	recordsOuter := totalWidth - schemaOuter
	schemaInner := schemaOuter - 4
	if schemaInner < 1 {
		schemaInner = 1
	}
	recordsInner := recordsOuter - 4
	if recordsInner < 1 {
		recordsInner = 1
	}

	contentH := height - 2 // panel border consumes 2 rows
	schemaContent := m.preview.renderSchema(schemaInner, contentH)
	recordsContent := m.preview.renderRecords(recordsInner, contentH)

	schemaPanel := focusedPanelStyle.Width(schemaInner).Height(height).Render(schemaContent)
	recordsPanel := panelStyle.Width(recordsInner).Height(height).Render(recordsContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, schemaPanel, recordsPanel)
}

func (m homeModel) panelWidths() (left, right int) {
	total := m.width
	left = total / 3
	if left < 30 {
		left = 30
	}
	right = total - left
	return
}

func (m homeModel) renderCollectionList(width, height int) string {
	var sb strings.Builder

	title := sectionTitleStyle.Width(width - 4).Render("Collections")
	sb.WriteString(title)
	sb.WriteByte('\n')

	// Filter input line.
	filterCursor := ""
	if m.filterFocused {
		filterCursor = "█"
	}
	filterLine := "  Filter: " + m.filterValue + filterCursor
	if m.filterFocused {
		filterLine = selectedItemStyle.Render(filterLine)
	} else {
		filterLine = mutedStyle.Render(filterLine)
	}
	sb.WriteString(filterLine)
	sb.WriteByte('\n')

	maxVisible := height - 4 // title + filter + add button + 1 margin
	start := 0
	if m.cursor >= maxVisible && m.cursor < len(m.filteredCollections) {
		start = m.cursor - maxVisible + 1
	}

	for i := start; i < len(m.filteredCollections) && i < start+maxVisible; i++ {
		entry := m.filteredCollections[i]
		rel, err := filepath.Rel(m.dbPath, entry.dirPath)
		if err != nil {
			rel = entry.dirPath
		}
		relPath := shortenPath(rel, width-4)
		label := fmt.Sprintf(" %-*s", width-2, entry.id)
		pathLabel := mutedStyle.Render(fmt.Sprintf("  %s", relPath))
		var row string
		if i == m.cursor && !m.filterFocused {
			row = selectedItemStyle.Width(width).Render(label) + "\n" + pathLabel
		} else {
			row = itemStyle.Render(label) + "\n" + pathLabel
		}
		sb.WriteString(row)
		sb.WriteByte('\n')
	}

	// Add button at the bottom.
	addIdx := len(m.filteredCollections)
	var btnLabel string
	if addIdx == m.cursor && !m.filterFocused {
		btnLabel = selectedItemStyle.Render(" " + addBtnText)
	} else {
		btnLabel = addButtonStyle.Render(" " + addBtnText)
	}
	sb.WriteString(btnLabel)

	return sb.String()
}

func (m homeModel) renderWelcome(width, _ int) string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Welcome to inGitDB CLI"))
	sb.WriteString("\n\n")

	desc := wordWrap(`inGitDB is a Git-backed database that stores records as YAML or JSON files `+
		`directly in your Git repository. Every change is version-controlled, diffable, and `+
		`auditable — no separate database server required.`, width)
	sb.WriteString(desc)
	sb.WriteString("\n\n")

	features := []string{
		"● Schema-first: define collections in YAML",
		"● Records stored as plain YAML/JSON files",
		"● Full Git history for every record change",
		"● Built-in validation and materialized views",
		"● GitHub-backed remote databases",
	}
	for _, f := range features {
		sb.WriteString(itemStyle.Render(f))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(mutedStyle.Render("GitHub: "))
	sb.WriteString(linkStyle.Render(githubURL))

	dbLine := fmt.Sprintf("\n\nDB path: %s", shortenPath(m.dbPath, width-10))
	sb.WriteString(mutedStyle.Render(dbLine))

	return sb.String()
}

func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	var sb strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := len(w)
		if lineLen > 0 && lineLen+1+wl > width {
			sb.WriteByte('\n')
			lineLen = 0
		} else if i > 0 {
			sb.WriteByte(' ')
			lineLen++
		}
		sb.WriteString(w)
		lineLen += wl
	}
	return sb.String()
}

// collectionRelPath returns dirPath relative to dbPath.
// If filepath.Rel returns an error the absolute dirPath is returned unchanged.
func collectionRelPath(dbPath, dirPath string) string {
	rel, err := filepath.Rel(dbPath, dirPath)
	if err != nil {
		return dirPath
	}
	return rel
}

func shortenPath(p string, maxLen int) string {
	if len(p) <= maxLen || maxLen <= 0 {
		return p
	}
	half := (maxLen - 1) / 2
	return p[:half] + "…" + p[len(p)-half:]
}
