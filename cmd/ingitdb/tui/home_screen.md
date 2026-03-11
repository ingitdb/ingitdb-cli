# Home Screen

## Overview

The home screen is the main entry point of the inGitDB TUI, launched when running `ingitdb` with no subcommand inside a repository. It displays all available collections in a filterable list with a three-panel layout for browsing collections, viewing records, and inspecting schemas.

## Layout

The screen is divided into three panels with only one focused (active) at a time:

### Left Panel: Collections List

**Purpose:** Browse and filter all collections in the database.

- **Filter Input:** Text field for searching collections by ID (case-insensitive substring match)
  - Focused when `panelCollections` is active (see panel focus below)
  - Input: Type characters to filter; use `backspace` to delete characters
  - Cursor: Visual block cursor (█) shows when focused

- **Collection Items:** Sorted list of collections matching the filter
  - Shows collection ID and relative directory path
  - Current selection highlighted only when this panel is focused
  - Supports scrolling when list exceeds viewport height

- **Add Button:** "+ Add collection" button at the bottom
  - Placeholder for future functionality
  - Can be navigated to with cursor (currently non-functional in this phase)

**Focus:** `panelCollections`

### Middle Panel: Records Table

**Purpose:** Show data records from the selected collection.

When a collection is selected:
- **Title Row:** Collection name (left) and locale dropdown (right, if applicable)
  - Locale selector is positioned in the **top-right corner** of the data panel header
  - Displays the current locale with a chevron indicator: `[ en ▼ ]` (closed) or `[ en ▲ ]` (open)
  - Only displayed when collection has localized (L10N) columns

- **Column Headers:** Styled header row with column names
  - Separated by vertical pipe `│`

- **Data Rows:** Records displayed in table format
  - Current record highlighted when this panel is focused
  - Columns auto-sized based on content
  - Values truncated with ellipsis (…) if they exceed 30 chars per column
  - Emoji flag indicators converted to country codes (e.g., 🇺🇸 → US)

- **Status Line:** Record count summary at the bottom
  - Shows total number of records

When no collection selected:
- Welcome screen appears showing inGitDB features and GitHub link
- Database path displayed at bottom

**Focus:** `panelData`

### Right Panel: Schema

**Purpose:** Display the collection's schema definition.

- **Collection Metadata:**
  - Collection ID
  - Record file format (YAML or JSON)
  - Record file name

- **Columns Section:** Lists all columns in the collection
  - Column name (styled)
  - Data type (e.g., `string`, `int`, `l10n`)
  - `required` flag (if applicable)

- **Sub-collections Section:** (if present) Lists all child collections
  - Sub-collection IDs in sorted order

The schema panel is scrollable when content exceeds available height.

**Focus:** `panelSchema`

## Panel Focus System

**Only one panel can be focused (active) at a time.** The focused panel:
- Receives visual distinction (border highlight)
- Handles input from arrow keys and other navigation commands
- Shows cursor/selection state

### Panel Navigation

| Key | Action |
|-----|--------|
| `←` | Move focus to previous panel (collections ← data ← schema) |
| `→` | Move focus to next panel (collections → data → schema) |

When switching panels, the previously focused content area remains visible and accessible.

## Navigation Within Panels

### Collections Panel (when focused)

| Key | Action |
|-----|--------|
| `↑` / `k` | Navigate up in collection list |
| `↓` / `j` | Navigate down in collection list |
| `a-z`, `0-9` | Type to filter collections |
| `backspace` | Delete last character in filter |
| `enter` | Open selected collection in collection detail screen |

### Records Panel (when focused)

| Key | Action |
|-----|--------|
| `↑` / `k` | Navigate to previous record (or navigate locale options when dropdown is open) |
| `↓` / `j` | Navigate to next record (or navigate locale options when dropdown is open) |
| `home` | Jump to first record |
| `end` | Jump to last record |
| `l` / `L` | Open locale selector dropdown (if L10N columns exist) |
| `enter` | Confirm locale selection (when dropdown is open) |
| `esc` | Close locale dropdown (when open) |

### Schema Panel (when focused)

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll schema content up |
| `↓` / `j` | Scroll schema content down |

## Global Navigation

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |

## Data Flow

1. **Initialization:** Collections loaded from `.ingitdb.yaml`, sorted alphabetically, and initial focus set to data panel
2. **Collection Selection:** When cursor moves in collections panel, a collection model is created and records are loaded asynchronously
3. **Panel Focus Change:** Left/right arrows switch focus without changing data; records table uses record cursor when data panel is focused
4. **Collection Detail:** Pressing `enter` in collections panel opens the collection detail screen
5. **Return Navigation:** Pressing `ESC` or `BACKSPACE` in collection detail screen returns to home screen

## Code Reference

- **Files:** `home_screen.go` (screen logic), `home_panel.go` (panel renderers)
- **Primary Type:** `homeModel`
- **Focus Enum:** `panelFocus` (values: `panelCollections`, `panelData`, `panelSchema`)
- **Key Fields:**
  - `focus` — Current focused panel
  - `recordCursor` — Selected record index in records table
  - `recordOffset` — Scroll offset in records table
  - `preview` — Collection model containing records and schema

- **Key Methods:**
  - `newHomeModel()` — Constructor
  - `Update()` — Handles input and state changes
  - `View()` — Renders all three panels
  - `renderCollectionList()` — Renders collections panel
  - `renderRecords()` — Renders records panel
  - `renderSchema()` — Renders schema panel
  - `applyFilter()` — Filters collections by name
  - `refreshPreview()` — Updates preview when cursor moves
  - `panelInnerDims()` — Calculates available panel dimensions
