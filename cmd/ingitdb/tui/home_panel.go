package tui

import (
	"fmt"
	"path/filepath"
	"strings"
)

// renderSchema renders the schema panel content.
func (m homeModel) renderSchema(width, height int) string {
	if m.preview == nil {
		return ""
	}
	return m.preview.renderSchema(width, height)
}

// renderRecords renders the records panel content, using the home screen's record cursor.
func (m homeModel) renderRecords(width, height int) string {
	if m.preview == nil {
		return ""
	}
	// Temporarily override the preview's record cursor with home screen's cursor
	originalCursor := m.preview.recordCursor
	originalOffset := m.preview.recordOffset
	m.preview.recordCursor = m.recordCursor
	m.preview.recordOffset = m.recordOffset
	content := m.preview.renderRecords(width, height)
	m.preview.recordCursor = originalCursor
	m.preview.recordOffset = originalOffset
	return content
}

func (m homeModel) renderCollectionList(width, height int) string {
	var sb strings.Builder

	title := sectionTitleStyle.Width(width).Render("Collections")
	sb.WriteString(title)
	sb.WriteByte('\n')

	// Filter input line.
	filterCursor := ""
	if m.focus == panelCollections {
		filterCursor = "█"
	}
	filterLine := "  Filter: " + m.filterValue + filterCursor
	if m.focus == panelCollections {
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
		if i == m.cursor && m.focus == panelCollections {
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
	if addIdx == m.cursor && m.focus == panelCollections {
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
