package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

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
