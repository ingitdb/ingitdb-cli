// Package tui provides the interactive terminal UI for the inGitDB CLI,
// launched when the tool is invoked without a subcommand inside an inGitDB
// repository.
package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dal-go/dalgo/dal"
	"github.com/rivo/uniseg"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

type screen int

const (
	screenHome screen = iota
	screenCollection
)

// Model is the root bubbletea model that owns the current screen and manages
// screen transitions.
type Model struct {
	dbPath string
	def    *ingitdb.Definition
	newDB  func(string, *ingitdb.Definition) (dal.DB, error)
	width  int
	height int

	currentScreen screen
	home          homeModel
	collection    *collectionModel
	// parents holds the collection screens a subcollection screen was opened
	// from, root first; esc returns to the last one.
	parents []collectionModel
}

// New creates the root model. width/height are the initial terminal dimensions.
func New(
	dbPath string,
	def *ingitdb.Definition,
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	width, height int,
) Model {
	home := newHomeModel(dbPath, def, newDB, width, height)
	return Model{
		dbPath:        dbPath,
		def:           def,
		newDB:         newDB,
		width:         width,
		height:        height,
		currentScreen: screenHome,
		home:          home,
	}
}

func (m Model) Init() tea.Cmd {
	return m.home.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.home, _ = m.home.Update(msg)
		if m.collection != nil {
			updated, _ := m.collection.Update(msg)
			m.collection = &updated
		}
		for i := range m.parents {
			m.parents[i], _ = m.parents[i].Update(msg)
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "backspace":
			if m.currentScreen == screenCollection {
				// Like esc: an open dropdown closes first.
				if m.collection != nil && (m.collection.localeDropdownOpen || m.collection.subDropdownOpen) {
					updated, cmd := m.collection.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
					m.collection = &updated
					return m, cmd
				}
				m = m.back()
			}
			return m, nil

		case "esc":
			if m.currentScreen == screenCollection {
				// If a dropdown is open, close it instead of navigating back.
				if m.collection != nil && (m.collection.localeDropdownOpen || m.collection.subDropdownOpen) {
					updated, cmd := m.collection.Update(msg)
					m.collection = &updated
					return m, cmd
				}
				return m.back(), nil
			}
			// On home screen, route esc to homeModel for dropdown handling.
			updated, cmd := m.home.Update(msg)
			m.home = updated
			return m, cmd

		case "enter":
			if m.currentScreen == screenHome {
				// If data panel dropdown is open, route enter to homeModel.
				if m.home.panels.IsFocused(panelData) && m.home.preview != nil && m.home.preview.localeDropdownOpen {
					updated, cmd := m.home.Update(msg)
					m.home = updated
					return m, cmd
				}
				colDef := m.home.SelectedCollection()
				if colDef != nil {
					return m.openCollection(colDef)
				}
			}
		}

	case openSubcollectionMsg:
		if m.currentScreen == screenCollection && m.collection != nil {
			return m.openSubcollection(msg)
		}
		return m, nil

	case recordsLoadedMsg:
		if m.currentScreen == screenHome {
			updated, cmd := m.home.Update(msg)
			m.home = updated
			return m, cmd
		}
		if m.collection != nil {
			updated, cmd := m.collection.Update(msg)
			m.collection = &updated
			return m, cmd
		}
	}

	// Delegate remaining key events to the active screen.
	return m.delegateUpdate(msg)
}

func (m Model) openCollection(colDef *ingitdb.CollectionDef) (Model, tea.Cmd) {
	db, err := m.newDB(m.dbPath, m.def)
	if err != nil {
		// Show error inline on welcome panel — don't crash.
		_ = err
		return m, nil
	}
	col := newCollectionModel(colDef, db, m.width, m.height)
	m.collection = &col
	m.currentScreen = screenCollection
	return m, m.collection.Init()
}

// openSubcollection opens a collection screen for a declared subcollection of
// the current screen's selected record, keeping the current screen to return to.
//
// specscore: feature/subcollection-addressing
func (m Model) openSubcollection(msg openSubcollectionMsg) (Model, tea.Cmd) {
	parents := make([]collectionModel, 0, len(m.parents)+1)
	parents = append(parents, m.parents...)
	parents = append(parents, *m.collection)
	col := newCollectionModel(msg.colDef, m.collection.db, m.width, m.height)
	col.parent = msg.parent
	col.path = msg.path
	m.parents = parents
	m.collection = &col
	return m, col.Init()
}

// back leaves the current collection screen: to the screen a subcollection was
// opened from, or to the home screen from a root collection.
func (m Model) back() Model {
	if n := len(m.parents); n > 0 {
		parent := m.parents[n-1]
		m.parents = m.parents[:n-1]
		m.collection = &parent
		return m
	}
	m.currentScreen = screenHome
	m.collection = nil
	return m
}

func (m Model) delegateUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.currentScreen {
	case screenHome:
		updated, cmd := m.home.Update(msg)
		m.home = updated
		return m, cmd
	case screenCollection:
		if m.collection != nil {
			updated, cmd := m.collection.Update(msg)
			m.collection = &updated
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	header := m.renderHeader()
	var body string
	switch m.currentScreen {
	case screenHome:
		body = m.home.View()
	case screenCollection:
		if m.collection != nil {
			body = m.collection.View()
		}
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderHeader() string {
	var title string
	switch m.currentScreen {
	case screenHome:
		title = "  inGitDB"
	case screenCollection:
		const prefix = "  inGitDB  ›  "
		colPath := ""
		if m.collection != nil {
			colPath = strings.Join(m.collection.path, " › ")
		}
		// headerStyle pads 2 columns on each side; the path gets what remains
		// after the prefix and is cut from the left so the header never wraps.
		avail := m.width - headerStyle.GetHorizontalFrameSize() - uniseg.StringWidth(prefix)
		title = fmt.Sprintf("%s%s", prefix, truncateLeft(colPath, avail))
	}
	return headerStyle.Width(m.width).Render(title)
}

// truncateLeft keeps the end of s within maxWidth display columns, replacing
// the dropped beginning with "…", so the deepest path segments stay visible.
//
// specscore: feature/subcollection-addressing
func truncateLeft(s string, maxWidth int) string {
	if uniseg.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth < 1 {
		return ""
	}
	g := uniseg.NewGraphemes(s)
	var clusters []string
	for g.Next() {
		clusters = append(clusters, g.Str())
	}
	width := 1 // the ellipsis
	start := len(clusters)
	for start > 0 {
		w := uniseg.StringWidth(clusters[start-1])
		if width+w > maxWidth {
			break
		}
		width += w
		start--
	}
	return "…" + strings.Join(clusters[start:], "")
}
