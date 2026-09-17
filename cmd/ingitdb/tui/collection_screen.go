package tui

import (
	"context"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/dal-go/record"

	"github.com/ingitdb/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// recordsLoadedMsg carries the records loaded from disk for a collection. It
// holds the query recordset and its retained per-record row handles (so computed
// columns resolve lazily, at paint time) plus each record's stored (non-computed)
// field values for layout and locale discovery — never pre-evaluated computed
// values.
type recordsLoadedMsg struct {
	// path identifies the collection screen the load is for (see
	// collectionModel.pathKey); a screen ignores loads for another path. Empty
	// means "any screen".
	path    string
	rs      recordset.Recordset
	rows    []recordset.Row
	records []map[string]any
	keys    []string
	err     error
}

// openSubcollectionMsg asks the root model to open a collection screen for a
// declared subcollection of the selected record.
//
// specscore: feature/subcollection-addressing
type openSubcollectionMsg struct {
	colDef *ingitdb.CollectionDef // the declared subcollection definition
	parent *record.Key            // the record the subcollection belongs to
	path   []string               // display path: root collection, record key, subcollection, ...
}

// collectionModel renders the collection detail screen:
// narrow left column (schema) + wide right panel (records).
type collectionModel struct {
	colDef  *ingitdb.CollectionDef
	db      dal.DB
	width   int
	height  int
	loading bool

	// parent is the record a subcollection screen belongs to (nil for a root
	// collection); path is the display path from the root collection
	// (["lists", "to-buy", "items"]).
	parent *record.Key
	path   []string

	// left panel: schema lines + scroll
	schemaLines  []string
	schemaOffset int

	// right panel: records + scroll
	rs           recordset.Recordset // query recordset; computed columns resolve lazily on access
	rows         []recordset.Row     // retained row handles so dalgo's per-row memoization survives re-renders
	records      []map[string]any    // each record's stored (non-computed) values; computed cells resolve via rs/rows
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

	// subcollection chooser state (shown when a collection declares several)
	subDropdownOpen   bool
	subDropdownCursor int

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
		path:        []string{colDef.ID},
		schemaLines: buildSchemaLines(colDef),
		columns:     orderedColumns(colDef),
		panels:      panelNav{count: 2, focus: 1}, // right (data) panel focused by default
	}
}

func (m collectionModel) Init() tea.Cmd {
	return loadRecordsCmd(m.db, m.colDef, m.parent, m.pathKey())
}

// pathKey returns the screen's path joined with "/" (for example
// "lists/to-buy/items"), identifying which screen a records load is for.
func (m collectionModel) pathKey() string {
	return strings.Join(m.path, "/")
}

// subcollectionIDs returns the IDs of the subcollections the collection
// declares, sorted.
func (m collectionModel) subcollectionIDs() []string {
	ids := make([]string, 0, len(m.colDef.SubCollections))
	for id := range m.colDef.SubCollections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// openSubcollectionCmd returns a command asking the root model to open the
// declared subcollection id of the selected record.
//
// specscore: feature/subcollection-addressing
func (m collectionModel) openSubcollectionCmd(id string) tea.Cmd {
	recordKey := m.recordKeys[m.recordCursor]
	path := make([]string, 0, len(m.path)+2)
	path = append(path, m.path...)
	path = append(path, recordKey, id)
	msg := openSubcollectionMsg{
		colDef: m.colDef.SubCollections[id],
		parent: record.NewKeyWithParentAndID(m.parent, m.colDef.ID, recordKey),
		path:   path,
	}
	return func() tea.Msg { return msg }
}

func (m collectionModel) Update(msg tea.Msg) (collectionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case recordsLoadedMsg:
		if msg.path != "" && msg.path != m.pathKey() {
			// A load for another screen (e.g. a subcollection left before its
			// records arrived) must not replace this screen's records.
			return m, nil
		}
		m.loading = false
		m.rs = msg.rs
		m.rows = msg.rows
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
			if m.subDropdownOpen {
				if m.subDropdownCursor > 0 {
					m.subDropdownCursor--
				}
			} else if m.localeDropdownOpen {
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
			if m.subDropdownOpen {
				if m.subDropdownCursor < len(m.colDef.SubCollections)-1 {
					m.subDropdownCursor++
				}
			} else if m.localeDropdownOpen {
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
			if len(m.locales) > 1 && !m.localeDropdownOpen && !m.subDropdownOpen {
				m.localeDropdownOpen = true
				m.localeDropdownCursor = localeIndex(m.locales, m.locale)
			}
		case "enter":
			switch {
			case m.localeDropdownOpen:
				m.locale = m.locales[m.localeDropdownCursor]
				m.columns = buildDisplayColumns(m.colDef, m.locale)
				m.localeDropdownOpen = false
				m.colWidths = m.computeColWidths()
				m.colCursor = 0
				m.colOffset = 0
			case m.subDropdownOpen:
				m.subDropdownOpen = false
				ids := m.subcollectionIDs()
				return m, m.openSubcollectionCmd(ids[m.subDropdownCursor])
			case len(m.colDef.SubCollections) > 0 && m.recordCursor < len(m.recordKeys):
				ids := m.subcollectionIDs()
				if len(ids) == 1 {
					return m, m.openSubcollectionCmd(ids[0])
				}
				m.subDropdownOpen = true
				m.subDropdownCursor = 0
			}
		case "esc":
			if m.localeDropdownOpen {
				m.localeDropdownOpen = false
			}
			if m.subDropdownOpen {
				m.subDropdownOpen = false
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
	help := helpStyle.Render(" ↑/↓ row  ←/→ column  alt+←/→ panels  l locale  enter open  esc back  q quit")
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

// loadRecordsCmd returns a Tea command that reads all records for the
// collection. For a subcollection, parent is the record it belongs to and the
// driver scopes the read to that record's subcollection records; path tags the
// resulting message with the screen it is for.
func loadRecordsCmd(db dal.DB, colDef *ingitdb.CollectionDef, parent *record.Key, path string) tea.Cmd {
	return func() tea.Msg {
		colID := colDef.ID
		qb := dal.NewQueryBuilder(dal.From(dal.NewCollectionRef(colID, "", parent)))
		q := qb.SelectIntoRecord(func() record.Record {
			key := record.NewKeyWithID(colID, "")
			return record.NewRecordWithData(key, map[string]any{})
		})

		var (
			rs      recordset.Recordset
			rows    []recordset.Row
			records []map[string]any
			keys    []string
		)
		err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
			reader, txErr := tx.ExecuteQueryToRecordsetReader(ctx, q)
			if txErr != nil {
				return txErr
			}
			defer func() { _ = reader.Close() }()
			var storedNames []string
			for {
				row, rsLocal, nextErr := reader.Next()
				if nextErr != nil {
					break
				}
				rs = rsLocal
				if storedNames == nil {
					// Read only stored (non-computed) columns: computed columns are
					// resolved lazily at paint time, never eagerly at load.
					storedNames = dalgo2ingitdb.StoredColumnNames(rs)
				}
				recKey := dalgo2ingitdb.RowKey(row, rs)
				stored, derr := dalgo2ingitdb.RowData(row, rs, colID, recKey, colDef, storedNames)
				if derr != nil {
					return derr
				}
				// Retain the row handle so dalgo's per-row memoization of computed
				// values survives re-renders and scroll-back.
				rows = append(rows, row)
				keys = append(keys, recKey)
				records = append(records, stored)
			}
			return nil
		})
		return recordsLoadedMsg{path: path, rs: rs, rows: rows, records: records, keys: keys, err: err}
	}
}
