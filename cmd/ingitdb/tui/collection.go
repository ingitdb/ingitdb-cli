package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// recordsLoadedMsg carries all records loaded from disk for a collection.
type recordsLoadedMsg struct {
	records []map[string]any
	keys    []string
	err     error
}

// collectionModel renders the collection detail screen:
// narrow left column (schema) + wide right panel (records).
type collectionModel struct {
	colDef  *ingitdb.CollectionDef
	db      dal.DB
	width   int
	height  int
	loading bool

	// schema viewport (left panel)
	schemaVP viewport.Model

	// records panel (right)
	records      []map[string]any
	recordKeys   []string
	recordCursor int
	columns      []string // ordered column names to display
	recordsVP    viewport.Model
}

func newCollectionModel(colDef *ingitdb.CollectionDef, db dal.DB, width, height int) collectionModel {
	leftW, rightW := collectionPanelWidths(width)
	leftInner := leftW - 4
	rightInner := rightW - 4
	innerH := height - 5

	schemaVP := viewport.New(leftInner, innerH)
	schemaVP.SetContent(renderSchemaContent(colDef, leftInner))

	recordsVP := viewport.New(rightInner, innerH)
	recordsVP.SetContent(mutedStyle.Render("Loading records…"))

	cols := orderedColumns(colDef)

	return collectionModel{
		colDef:    colDef,
		db:        db,
		width:     width,
		height:    height,
		loading:   true,
		schemaVP:  schemaVP,
		recordsVP: recordsVP,
		columns:   cols,
	}
}

func (m collectionModel) Init() tea.Cmd {
	return loadRecordsCmd(m.db, m.colDef)
}

func (m collectionModel) Update(msg tea.Msg) (collectionModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftW, rightW := collectionPanelWidths(m.width)
		leftInner := leftW - 4
		rightInner := rightW - 4
		innerH := m.height - 5
		m.schemaVP.Width = leftInner
		m.schemaVP.Height = innerH
		m.recordsVP.Width = rightInner
		m.recordsVP.Height = innerH

	case recordsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.recordsVP.SetContent(lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).
				Render(fmt.Sprintf("Error loading records: %v", msg.err)))
		} else {
			m.records = msg.records
			m.recordKeys = msg.keys
			m.recordsVP.SetContent(m.renderRecordsTable())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.recordCursor > 0 {
				m.recordCursor--
				m.recordsVP.SetContent(m.renderRecordsTable())
			}
		case "down", "j":
			if m.recordCursor < len(m.records)-1 {
				m.recordCursor++
				m.recordsVP.SetContent(m.renderRecordsTable())
			}
		default:
			var cmd tea.Cmd
			m.schemaVP, cmd = m.schemaVP.Update(msg)
			cmds = append(cmds, cmd)
			m.recordsVP, cmd = m.recordsVP.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m collectionModel) View() string {
	leftW, rightW := collectionPanelWidths(m.width)
	leftInner := leftW - 4
	rightInner := rightW - 4
	innerH := m.height - 5

	left := focusedPanelStyle.Width(leftInner).Height(innerH).Render(m.schemaVP.View())
	right := panelStyle.Width(rightInner).Height(innerH).Render(m.recordsVP.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := helpStyle.Render(" ↑/↓ navigate records  esc back  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

// renderRecordsTable renders the records as a simple ASCII table.
func (m collectionModel) renderRecordsTable() string {
	if len(m.records) == 0 {
		return mutedStyle.Render("(no records)")
	}

	cols := m.columns
	if len(cols) == 0 {
		return mutedStyle.Render("(no columns defined)")
	}

	// Calculate column widths.
	colWidths := make([]int, len(cols))
	for i, c := range cols {
		colWidths[i] = len(c)
	}
	for _, row := range m.records {
		for i, c := range cols {
			v := fmt.Sprintf("%v", row[c])
			if len(v) > colWidths[i] {
				colWidths[i] = len(v)
			}
		}
	}
	// Cap each column at 30 chars.
	for i := range colWidths {
		if colWidths[i] > 30 {
			colWidths[i] = 30
		}
	}

	var sb strings.Builder

	// Header row.
	headerCells := make([]string, len(cols))
	for i, c := range cols {
		headerCells[i] = columnKeyStyle.Render(padRight(c, colWidths[i]))
	}
	sb.WriteString(strings.Join(headerCells, " │ "))
	sb.WriteByte('\n')

	// Separator.
	seps := make([]string, len(cols))
	for i, w := range colWidths {
		seps[i] = strings.Repeat("─", w)
	}
	sb.WriteString(mutedStyle.Render(strings.Join(seps, "─┼─")))
	sb.WriteByte('\n')

	// Data rows.
	for ri, row := range m.records {
		cells := make([]string, len(cols))
		for i, c := range cols {
			v := fmt.Sprintf("%v", row[c])
			if len(v) > colWidths[i] {
				v = v[:colWidths[i]-1] + "…"
			}
			cells[i] = padRight(v, colWidths[i])
		}
		line := strings.Join(cells, " │ ")
		if ri == m.recordCursor {
			line = selectedItemStyle.Render(line)
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}

	total := fmt.Sprintf("\n%d record(s)", len(m.records))
	sb.WriteString(mutedStyle.Render(total))
	return sb.String()
}

// renderSchemaContent builds the left-panel schema text.
func renderSchemaContent(colDef *ingitdb.CollectionDef, _ int) string {
	var sb strings.Builder

	title := sectionTitleStyle.Render("Schema")
	sb.WriteString(title)
	sb.WriteByte('\n')

	// Collection ID.
	sb.WriteString(columnKeyStyle.Render("id: "))
	sb.WriteString(colDef.ID)
	sb.WriteByte('\n')

	// Record file.
	if rf := colDef.RecordFile; rf != nil {
		sb.WriteString(columnKeyStyle.Render("format: "))
		sb.WriteString(string(rf.Format))
		sb.WriteByte('\n')
		sb.WriteString(columnKeyStyle.Render("file: "))
		sb.WriteString(rf.Name)
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(sectionTitleStyle.Render("Columns"))
	sb.WriteByte('\n')

	cols := orderedColumns(colDef)
	for _, name := range cols {
		col := colDef.Columns[name]
		sb.WriteString(columnKeyStyle.Render(name))
		sb.WriteByte('\n')
		sb.WriteString(columnTypeStyle.Render(fmt.Sprintf("  type: %s", col.Type)))
		sb.WriteByte('\n')
		if col.Required {
			sb.WriteString(columnTypeStyle.Render("  required"))
			sb.WriteByte('\n')
		}
	}

	if len(colDef.SubCollections) > 0 {
		sb.WriteByte('\n')
		sb.WriteString(sectionTitleStyle.Render("Sub-collections"))
		sb.WriteByte('\n')
		subs := make([]string, 0, len(colDef.SubCollections))
		for id := range colDef.SubCollections {
			subs = append(subs, id)
		}
		sort.Strings(subs)
		for _, id := range subs {
			sb.WriteString(itemStyle.Render("  " + id))
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// loadRecordsCmd returns a Tea command that reads all records for the collection.
func loadRecordsCmd(db dal.DB, colDef *ingitdb.CollectionDef) tea.Cmd {
	return func() tea.Msg {
		colID := colDef.ID
		qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(colID, "")))
		q := qb.SelectIntoRecord(func() dal.Record {
			key := dal.NewKeyWithID(colID, "")
			return dal.NewRecordWithData(key, map[string]any{})
		})

		var (
			records []map[string]any
			keys    []string
		)
		err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
			reader, err := tx.ExecuteQueryToRecordsReader(ctx, q)
			if err != nil {
				return err
			}
			defer func() { _ = reader.Close() }()
			for {
				rec, nextErr := reader.Next()
				if nextErr != nil {
					break
				}
				data := rec.Data().(map[string]any)
				keys = append(keys, fmt.Sprintf("%v", rec.Key().ID))
				records = append(records, data)
			}
			return nil
		})
		return recordsLoadedMsg{records: records, keys: keys, err: err}
	}
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

func collectionPanelWidths(totalWidth int) (left, right int) {
	left = totalWidth / 4
	if left < 24 {
		left = 24
	}
	right = totalWidth - left
	return
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
