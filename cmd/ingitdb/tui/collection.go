package tui

import (
"context"
"fmt"
"sort"
"strings"

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

case tea.KeyPressMsg:
_, innerH := m.panelInnerDims()
switch msg.String() {
case "up", "k":
if m.recordCursor > 0 {
m.recordCursor--
if m.recordCursor < m.recordOffset {
m.recordOffset = m.recordCursor
}
}
case "down", "j":
if m.recordCursor < len(m.records)-1 {
m.recordCursor++
visibleRows := innerH - 3 // header + separator + total line
if m.recordCursor >= m.recordOffset+visibleRows {
m.recordOffset = m.recordCursor - visibleRows + 1
}
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

leftContent := m.renderSchema(leftInner, innerH)
rightContent := m.renderRecords(rightInner, innerH)

left := focusedPanelStyle.Width(leftInner).Height(innerH).Render(leftContent)
right := panelStyle.Width(rightInner).Height(innerH).Render(rightContent)

panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
help := helpStyle.Render(" ↑/↓ navigate records  esc back  q quit")
return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

func (m collectionModel) panelInnerDims() (width, height int) {
leftW, _ := collectionPanelWidths(m.width)
return leftW - 4, m.height - 5
}

func (m collectionModel) renderSchema(width, height int) string {
lines := m.schemaLines
end := m.schemaOffset + height
if end > len(lines) {
end = len(lines)
}
visible := lines[m.schemaOffset:end]
// Pad to fill height.
for len(visible) < height {
visible = append(visible, "")
}
return strings.Join(visible[:height], "\n")
}

func (m collectionModel) renderRecords(width, height int) string {
if m.loading {
return mutedStyle.Render("Loading records…")
}
if len(m.records) == 0 {
return mutedStyle.Render("(no records)")
}
cols := m.columns
if len(cols) == 0 {
return mutedStyle.Render("(no columns defined)")
}

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

// Visible data rows.
visibleRows := height - 3
if visibleRows < 1 {
visibleRows = 1
}
end := m.recordOffset + visibleRows
if end > len(m.records) {
end = len(m.records)
}
for ri := m.recordOffset; ri < end; ri++ {
row := m.records[ri]
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
