package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

	// left panel: schema lines + scroll
	schemaLines  []string
	schemaOffset int

	// right panel: records + scroll
	records      []map[string]any
	recordKeys   []string
	recordCursor int
	recordOffset int
	columns      []string

	// cell selection and horizontal scroll
	colCursor int   // selected column index
	colOffset int   // first visible column index (horizontal scroll)
	colWidths []int // cached column display widths (for navigation)

	// locale handling for map[locale]string columns
	locale  string   // currently selected locale (e.g. "en")
	locales []string // available locales sorted by full language name

	// locale dropdown state
	localeDropdownOpen   bool
	localeDropdownCursor int

	// panel focus management
	panels panelNav // tracks which panel has keyboard focus
}

func newCollectionModel(colDef *ingitdb.CollectionDef, db dal.DB, width, height int) collectionModel {
	return collectionModel{
		colDef:      colDef,
		db:          db,
		width:       width,
		height:      height,
		loading:     true,
		schemaLines: buildSchemaLines(colDef),
		columns:     orderedColumns(colDef),
		panels:      panelNav{count: 2, focus: 1}, // right (data) panel focused by default
	}
}

func (m collectionModel) Init() tea.Cmd {
	return loadRecordsCmd(m.db, m.colDef)
}

func (m collectionModel) Update(msg tea.Msg) (collectionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case recordsLoadedMsg:
		m.loading = false
		m.records = msg.records
		m.recordKeys = msg.keys
		m.locales = discoverLocales(m.records, m.colDef)
		if len(m.locales) > 0 {
			m.locale = m.locales[0]
			for _, loc := range m.locales {
				if loc == "en" {
					m.locale = "en"
					break
				}
			}
			m.columns = buildDisplayColumns(m.colDef, m.locale)
		}
		m.colWidths = m.computeColWidths()
		m.colCursor = 0
		m.colOffset = 0

	case tea.KeyPressMsg:
		_, innerH := m.panelInnerDims()
		_ = innerH
		// Panel navigation takes priority.
		if m.panels.HandleKey(msg.String()) {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.localeDropdownOpen {
				if m.localeDropdownCursor > 0 {
					m.localeDropdownCursor--
				}
			} else if m.recordCursor > 0 {
				m.recordCursor--
				if m.recordCursor < m.recordOffset {
					m.recordOffset = m.recordCursor
				}
			} else if len(m.locales) > 1 {
				// At top row: give focus to locale selector dropdown.
				m.localeDropdownOpen = true
				m.localeDropdownCursor = localeIndex(m.locales, m.locale)
			}
		case "down", "j":
			if m.localeDropdownOpen {
				if m.localeDropdownCursor < len(m.locales)-1 {
					m.localeDropdownCursor++
				}
			} else if m.recordCursor < len(m.records)-1 {
				m.recordCursor++
				visibleRows := innerH - 4 // title + header + separator + total line
				if m.recordCursor >= m.recordOffset+visibleRows {
					m.recordOffset = m.recordCursor - visibleRows + 1
				}
			}
		case "l", "L":
			if len(m.locales) > 1 && !m.localeDropdownOpen {
				m.localeDropdownOpen = true
				m.localeDropdownCursor = localeIndex(m.locales, m.locale)
			}
		case "enter":
			if m.localeDropdownOpen {
				m.locale = m.locales[m.localeDropdownCursor]
				m.columns = buildDisplayColumns(m.colDef, m.locale)
				m.localeDropdownOpen = false
				m.colWidths = m.computeColWidths()
				m.colCursor = 0
				m.colOffset = 0
			}
		case "esc":
			if m.localeDropdownOpen {
				m.localeDropdownOpen = false
			}
		case "right":
			if !m.localeDropdownOpen && m.panels.IsFocused(1) && len(m.columns) > 0 && m.colCursor < len(m.columns)-1 {
				m.colCursor++
				_, rightW := collectionPanelWidths(m.width)
				innerW := rightW - 4
				m.colOffset = computeColOffset(m.colCursor, m.colOffset, m.colWidths, innerW)
			}
		case "left":
			if !m.localeDropdownOpen && m.panels.IsFocused(1) && m.colCursor > 0 {
				m.colCursor--
				_, rightW := collectionPanelWidths(m.width)
				innerW := rightW - 4
				m.colOffset = computeColOffset(m.colCursor, m.colOffset, m.colWidths, innerW)
			}
		}
	}
	return m, nil
}

func (m collectionModel) View() string {
	leftW, rightW := collectionPanelWidths(m.width)
	leftInner := leftW - 4
	rightInner := rightW - 4
	_, innerH := m.panelInnerDims()

	contentH := innerH - 2 // panel border consumes 2 rows
	leftContent := m.renderSchema(leftInner, contentH)
	rightContent := m.renderRecords(rightInner, contentH)

	// In lipgloss v2, Width() is the total outer (border-box) width.
	// Content area = Width - border(2) - padding(2) = leftW/rightW - 4 = inner widths.
	left := m.panels.Style(0).Width(leftW).Height(innerH).Render(leftContent)
	right := m.panels.Style(1).Width(rightW).Height(innerH).Render(rightContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := helpStyle.Render(" ↑/↓ row  ←/→ column  ctrl+←/→ panels  l locale  enter select  esc back  q quit")
	return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

func (m collectionModel) panelInnerDims() (width, height int) {
	leftW, _ := collectionPanelWidths(m.width)
	return leftW - 4, m.height - 2 // header + help bar
}

func collectionPanelWidths(totalWidth int) (left, right int) {
	left = totalWidth / 4
	if left < 24 {
		left = 24
	}
	right = totalWidth - left
	return
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
			reader, txErr := tx.ExecuteQueryToRecordsReader(ctx, q)
			if txErr != nil {
				return txErr
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
