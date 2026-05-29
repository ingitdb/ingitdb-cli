package tui

// specscore: feature/cli/resolve/manual-resolve

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ConflictsModel renders a placeholder screen for the not-yet-implemented
// interactive (manual) resolution of source-data conflicts. It describes what
// the future UI will do and exits on any of q / esc / enter / ctrl+c.
type ConflictsModel struct {
	files  []string
	width  int
	height int
}

// NewConflictsModel builds the placeholder model for the given conflicted files.
func NewConflictsModel(files []string, width, height int) ConflictsModel {
	return ConflictsModel{files: files, width: width, height: height}
}

func (m ConflictsModel) Init() tea.Cmd { return nil }

// Update quits on q/esc/enter/ctrl+c and tracks terminal resizes.
func (m ConflictsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "enter", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the placeholder screen: a title, a description of the envisioned
// per-field resolution UI, the conflicted files, and a not-implemented notice.
func (m ConflictsModel) View() tea.View {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Interactive conflict resolution"))
	b.WriteString("\n\n")
	b.WriteString(itemStyle.Render("These files have source-data conflicts that need a human decision:"))
	b.WriteString("\n")
	for _, f := range m.files {
		b.WriteString(columnKeyStyle.Render("  • " + f))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Planned: present each conflict field-by-field and let you pick a"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("winner per field, instead of editing raw conflict markers."))
	b.WriteString("\n\n")
	b.WriteString(addButtonStyle.Render("Sorry, not implemented yet."))

	content := panelStyle.Width(m.panelWidth()).Render(b.String())
	header := headerStyle.Render(" Resolve conflicts ")
	help := helpStyle.Render(" press q / esc / enter to quit ")
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, content, help))
}

// panelWidth clamps the panel width to a sensible range based on terminal size.
func (m ConflictsModel) panelWidth() int {
	w := m.width - 4
	w = max(w, 40)
	w = min(w, 100)
	return w
}

// RunConflicts launches the placeholder TUI program for the conflicted files.
func RunConflicts(ctx context.Context, files []string, width, height int) error {
	m := NewConflictsModel(files, width, height)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
