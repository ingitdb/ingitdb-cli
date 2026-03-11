package tui

import "charm.land/lipgloss/v2"

// lipglossStyle is an alias for lipgloss.Style used by panelNav.Style.
type lipglossStyle = lipgloss.Style

// panelNav tracks which of N side-by-side panels has keyboard focus and
// handles alt+left / alt+right navigation between them.
type panelNav struct {
	count int // total number of panels
	focus int // index of the focused panel (0 = leftmost)
}

// HandleKey processes alt+left / alt+right. Returns true if the key was consumed.
func (p *panelNav) HandleKey(key string) bool {
	switch key {
	case "alt+left":
		if p.focus > 0 {
			p.focus--
		}
		return true
	case "alt+right":
		if p.focus < p.count-1 {
			p.focus++
		}
		return true
	}
	return false
}

// IsFocused returns true when panel at index i is the focused one.
func (p panelNav) IsFocused(i int) bool { return p.focus == i }

// Style returns focusedPanelStyle for the focused panel, panelStyle for all others.
func (p panelNav) Style(i int) lipglossStyle {
	if p.IsFocused(i) {
		return focusedPanelStyle
	}
	return panelStyle
}
