package tui

import (
"fmt"
"sort"
"strings"

tea "charm.land/bubbletea/v2"
"charm.land/lipgloss/v2"

"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

const (
githubURL  = "https://github.com/ingitdb/ingitdb-cli"
addBtnText = "+ Add collection"
)

// homeModel is the main screen showing collection list and welcome panel.
type homeModel struct {
dbPath      string
def         *ingitdb.Definition
collections []collectionEntry // sorted list
cursor      int               // 0..len(collections) inclusive (+1 for add button)
width       int
height      int
}

type collectionEntry struct {
id      string
dirPath string
}

func newHomeModel(dbPath string, def *ingitdb.Definition, width, height int) homeModel {
entries := make([]collectionEntry, 0, len(def.Collections))
for id, c := range def.Collections {
entries = append(entries, collectionEntry{id: id, dirPath: c.DirPath})
}
sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
return homeModel{
dbPath:      dbPath,
def:         def,
collections: entries,
width:       width,
height:      height,
}
}

func (m homeModel) Update(msg tea.Msg) (homeModel, tea.Cmd) {
switch msg := msg.(type) {
case tea.KeyPressMsg:
switch msg.String() {
case "up", "k":
if m.cursor > 0 {
m.cursor--
}
case "down", "j":
if m.cursor < len(m.collections) {
m.cursor++
}
}
case tea.WindowSizeMsg:
m.width = msg.Width
m.height = msg.Height
}
return m, nil
}

// SelectedCollection returns the CollectionDef at the cursor, or nil if the
// cursor is on the add-button row.
func (m homeModel) SelectedCollection() *ingitdb.CollectionDef {
if m.cursor < len(m.collections) {
entry := m.collections[m.cursor]
return m.def.Collections[entry.id]
}
return nil
}

func (m homeModel) View() string {
leftWidth, rightWidth := m.panelWidths()
leftInner := leftWidth - 4
rightInner := rightWidth - 4
innerH := m.height - 5

leftContent := m.renderCollectionList(leftInner, innerH)
rightContent := m.renderWelcome(rightInner, innerH)

left := focusedPanelStyle.Width(leftInner).Height(innerH).Render(leftContent)
right := panelStyle.Width(rightInner).Height(innerH).Render(rightContent)

panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
help := helpStyle.Render(" ↑/↓ navigate  enter select  q quit")
return lipgloss.JoinVertical(lipgloss.Left, panels, help)
}

func (m homeModel) panelWidths() (left, right int) {
total := m.width
left = total / 3
if left < 30 {
left = 30
}
right = total - left
return
}

func (m homeModel) renderCollectionList(width, height int) string {
var sb strings.Builder

title := sectionTitleStyle.Width(width).Render("Collections")
sb.WriteString(title)
sb.WriteByte('\n')

maxVisible := height - 3
start := 0
if m.cursor >= maxVisible && m.cursor < len(m.collections) {
start = m.cursor - maxVisible + 1
}

for i := start; i < len(m.collections) && i < start+maxVisible; i++ {
entry := m.collections[i]
relPath := shortenPath(entry.dirPath, width-4)
label := fmt.Sprintf(" %-*s", width-2, entry.id)
pathLabel := mutedStyle.Render(fmt.Sprintf("  [%s]", relPath))
var row string
if i == m.cursor {
row = selectedItemStyle.Width(width).Render(label) + "\n" + pathLabel
} else {
row = itemStyle.Render(label) + "\n" + pathLabel
}
sb.WriteString(row)
sb.WriteByte('\n')
}

// Add button at the bottom.
addIdx := len(m.collections)
var btnLabel string
if addIdx == m.cursor {
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

func wordWrap(text string, width int) string {
if width <= 0 {
return text
}
words := strings.Fields(text)
var sb strings.Builder
lineLen := 0
for i, w := range words {
wl := len(w)
if lineLen > 0 && lineLen+1+wl > width {
sb.WriteByte('\n')
lineLen = 0
} else if i > 0 {
sb.WriteByte(' ')
lineLen++
}
sb.WriteString(w)
lineLen += wl
}
return sb.String()
}

func shortenPath(p string, maxLen int) string {
if len(p) <= maxLen || maxLen <= 0 {
return p
}
half := (maxLen - 1) / 2
return p[:half] + "…" + p[len(p)-half:]
}
