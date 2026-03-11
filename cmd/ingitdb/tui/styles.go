package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorMuted     = lipgloss.Color("#6B7280")
	colorSelected  = lipgloss.Color("#7C3AED")
	colorBorder    = lipgloss.Color("#374151")
	colorHighlight = lipgloss.Color("#EDE9FE")
	colorText      = lipgloss.Color("#F9FAFB")
	colorAccent    = lipgloss.Color("#A78BFA")
	colorGreen     = lipgloss.Color("#34D399")
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Padding(0, 2).
			Width(0) // set dynamically

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSelected).
				Background(colorHighlight)

	itemStyle = lipgloss.NewStyle().
			Foreground(colorText)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	addButtonStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	linkStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Underline(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorBorder)

	columnKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	columnTypeStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)
