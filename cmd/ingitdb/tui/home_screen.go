package tui

import (
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

type panelFocus int

const (
	panelCollections panelFocus = iota
	panelData
	panelSchema
)

// homeModel is the main screen showing collection list and preview panels.
type homeModel struct {
	dbPath              string
	def                 *ingitdb.Definition
	newDB               func(string, *ingitdb.Definition) (dal.DB, error)
	collections         []collectionEntry // sorted list of all collections
	filteredCollections []collectionEntry // subset of collections matching filterValue
	filterValue         string
	cursor              int // 0..len(filteredCollections) inclusive (+1 for add button)
	width               int
	height              int

	// panel focus management
	focus        panelFocus // which panel is active
	recordCursor int        // selected record in data panel
	recordOffset int        // scroll offset in data panel

	// preview for the currently selected collection
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
		dbPath:      dbPath,
		def:         def,
		newDB:       newDB,
		collections: entries,
		focus:       panelCollections,
		width:       width,
		height:      height,
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
		// When the data panel's locale dropdown is open, route all keys to the preview.
		if m.focus == panelData && m.preview != nil && m.preview.localeDropdownOpen {
			updated, cmd := m.preview.Update(msg)
			m.preview = &updated
			return m, cmd
		}

		switch msg.String() {
		case "backspace":
			if m.focus == panelCollections && len(m.filterValue) > 0 {
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
			if m.focus == panelCollections {
				if m.cursor > 0 {
					m.cursor--
					cmd := m.refreshPreview()
					return m, cmd
				}
			} else if m.focus == panelData && m.preview != nil && len(m.preview.records) > 0 {
				if m.recordCursor > 0 {
					m.recordCursor--
					if m.recordCursor < m.recordOffset {
						m.recordOffset = m.recordCursor
					}
				} else if len(m.preview.locales) > 1 {
					// At top row: give focus to locale selector dropdown.
					updated, cmd := m.preview.Update(msg)
					m.preview = &updated
					return m, cmd
				}
			}

		case "down", "j":
			if m.focus == panelCollections {
				if m.cursor < len(m.filteredCollections) {
					m.cursor++
					cmd := m.refreshPreview()
					return m, cmd
				}
			} else if m.focus == panelData && m.preview != nil && len(m.preview.records) > 0 {
				if m.recordCursor < len(m.preview.records)-1 {
					m.recordCursor++
					_, innerH := m.panelInnerDims()
					visibleRows := innerH - 4 // title + header + separator + total line
					if visibleRows < 1 {
						visibleRows = 1
					}
					if m.recordCursor >= m.recordOffset+visibleRows {
						m.recordOffset = m.recordCursor - visibleRows + 1
					}
				}
			}

		case "home":
			if m.focus == panelData && m.preview != nil && len(m.preview.records) > 0 {
				m.recordCursor = 0
				m.recordOffset = 0
			}

		case "end":
			if m.focus == panelData && m.preview != nil && len(m.preview.records) > 0 {
				m.recordCursor = len(m.preview.records) - 1
				_, innerH := m.panelInnerDims()
				visibleRows := innerH - 4 // title + header + separator + total line
				if visibleRows < 1 {
					visibleRows = 1
				}
				if m.recordCursor >= visibleRows {
					m.recordOffset = m.recordCursor - visibleRows + 1
				}
			}

		case "left":
			switch m.focus {
			case panelData:
				m.focus = panelCollections
			case panelSchema:
				m.focus = panelData
			case panelCollections:
				// already at leftmost panel
			}

		case "right":
			switch m.focus {
			case panelCollections:
				m.focus = panelData
			case panelData:
				m.focus = panelSchema
			case panelSchema:
				// already at rightmost panel
			}

		case "l", "L":
			if m.focus == panelData && m.preview != nil && len(m.preview.locales) > 1 {
				updated, cmd := m.preview.Update(msg)
				m.preview = &updated
				return m, cmd
			}

		case "esc":
			if m.focus == panelData && m.preview != nil && m.preview.localeDropdownOpen {
				updated, _ := m.preview.Update(msg)
				m.preview = &updated
				return m, nil
			}

		case "enter":
			if m.focus == panelData && m.preview != nil && m.preview.localeDropdownOpen {
				updated, _ := m.preview.Update(msg)
				m.preview = &updated
				return m, nil
			}

		default:
			if m.focus == panelCollections {
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

// panelInnerDims returns the available width and height for panel content.
func (m homeModel) panelInnerDims() (width, height int) {
	return m.width, m.height - 2 // header + help bar
}

func (m homeModel) View() string {
	_, innerH := m.panelInnerDims()
	contentH := innerH - 2 // panel border consumes 2 rows

	// Layout: 1/4 left (collections) | 1/2 middle (data) | 1/4 right (schema)
	leftWidth := m.width / 4
	if leftWidth < 20 {
		leftWidth = 20
	}
	rightWidth := m.width / 4
	if rightWidth < 24 {
		rightWidth = 24
	}
	middleWidth := m.width - leftWidth - rightWidth

	leftInner := leftWidth - 4
	if leftInner < 1 {
		leftInner = 1
	}
	middleInner := middleWidth - 4
	if middleInner < 1 {
		middleInner = 1
	}
	rightInner := rightWidth - 4
	if rightInner < 1 {
		rightInner = 1
	}

	// leftPanelW/middlePanelW/rightPanelW are the Width() args for lipgloss.
	// In lipgloss v2, Width() is the total outer (border-box) width.
	// Content area = Width - border(2) - padding(2) = allocatedWidth - 4 = innerWidth.
	leftPanelW := leftWidth
	middlePanelW := middleWidth
	rightPanelW := rightWidth

	// Left panel: collections list
	leftContent := m.renderCollectionList(leftInner, contentH)
	var leftPanel string
	if m.focus == panelCollections {
		leftPanel = focusedPanelStyle.Width(leftPanelW).Height(innerH).Render(leftContent)
	} else {
		leftPanel = panelStyle.Width(leftPanelW).Height(innerH).Render(leftContent)
	}

	// Middle panel: data table or welcome
	var middlePanel string
	if m.cursor < len(m.filteredCollections) && m.preview != nil {
		middleContent := m.renderRecords(middleInner, contentH)
		if m.focus == panelData {
			middlePanel = focusedPanelStyle.Width(middlePanelW).Height(innerH).Render(middleContent)
		} else {
			middlePanel = panelStyle.Width(middlePanelW).Height(innerH).Render(middleContent)
		}
	} else {
		welcomeContent := m.renderWelcome(middleInner, contentH)
		middlePanel = panelStyle.Width(middlePanelW).Height(innerH).Render(welcomeContent)
	}

	// Right panel: schema
	var rightPanel string
	if m.cursor < len(m.filteredCollections) && m.preview != nil {
		schemaContent := m.renderSchema(rightInner, contentH)
		if m.focus == panelSchema {
			rightPanel = focusedPanelStyle.Width(rightPanelW).Height(innerH).Render(schemaContent)
		} else {
			rightPanel = panelStyle.Width(rightPanelW).Height(innerH).Render(schemaContent)
		}
	} else {
		rightPanel = panelStyle.Width(rightPanelW).Height(innerH).Render("")
	}

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel)
	help := helpStyle.Render(" ↑/↓ navigate  ← → panels  l locale  home end  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
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
	return p[:half] + "\u2026" + p[len(p)-half:]
}
