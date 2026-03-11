// Package tui provides the interactive terminal UI for the inGitDB CLI,
// launched when the tool is invoked without a subcommand inside an inGitDB
// repository.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
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

	// generic full-screen viewport for error display
	errVP    *viewport.Model
	errTitle string
}

// New creates the root model. width/height are the initial terminal dimensions.
func New(
	dbPath string,
	def *ingitdb.Definition,
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	width, height int,
) Model {
	home := newHomeModel(dbPath, def, width, height)
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
	return nil
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
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "esc":
			if m.currentScreen == screenCollection {
				m.currentScreen = screenHome
				m.collection = nil
			}
			return m, nil

		case "enter":
			if m.currentScreen == screenHome {
				colDef := m.home.SelectedCollection()
				if colDef != nil {
					return m.openCollection(colDef)
				}
			}
		}

	case recordsLoadedMsg:
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

func (m Model) View() string {
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
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m Model) renderHeader() string {
	var title string
	switch m.currentScreen {
	case screenHome:
		title = "  inGitDB"
	case screenCollection:
		colID := ""
		if m.collection != nil {
			colID = m.collection.colDef.ID
		}
		title = fmt.Sprintf("  inGitDB  ›  %s", colID)
	}
	return headerStyle.Width(m.width).Render(title)
}
