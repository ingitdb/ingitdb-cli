# Collection Screen

## Overview

The collection detail screen displays a single collection with its schema definition and all records. It provides an in-depth view of the collection structure with support for locale-aware data and record navigation.

## Layout

The screen is divided into two side-by-side panels:

### Left Panel: Schema

**Purpose:** Display the collection definition and structure.

- **Collection Header:** Collection ID with a styled title
- **Metadata:**
  - Collection ID
  - Record file format (YAML or JSON)
  - Record file name

- **Columns Section:** Lists all columns in the collection
  - Column name (styled)
  - Data type (e.g., `string`, `int`, `l10n` for localized strings)
  - `required` flag (if applicable)

- **Sub-collections Section:** (if present) Lists all child collections
  - Sub-collection IDs in sorted order
  - Indicates hierarchical structure

The schema panel is scrollable when content exceeds available height.

### Right Panel: Records

**Purpose:** Display all records from the collection as a table.

- **Header Row:** Collection title (left) and locale dropdown (right, if applicable)
  - Locale selector is positioned in the **top-right corner** of the data panel header
  - Displays the current locale with a chevron indicator: `[ en ▼ ]` (closed) or `[ en ▲ ]` (open)
  - Only displayed when collection has localized (L10N) columns

- **Column Headers:** Styled header row with column names
  - Separated by vertical pipe `│`
  - Styled to distinguish from data

- **Separator Line:** Visual divider between headers and data
  - Uses box-drawing characters (─ and ┼)

- **Data Rows:** Records displayed in table format
  - Columns aligned (right-aligned for numeric, left-aligned for text)
  - Current record highlighted
  - Values truncated with ellipsis (…) if they exceed max width (30 chars per column)
  - Emoji flag indicators converted to country codes (e.g., 🇺🇸 → US) for terminal compatibility

- **Status Line:** Record count summary at the bottom
  - Shows total number of records

## Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Navigate to previous record (or navigate locale list when dropdown is open) |
| `↓` / `j` | Navigate to next record (or navigate locale list when dropdown is open) |
| `l` / `L` | Open locale selector dropdown (if L10N columns exist) |
| `enter` | Confirm locale selection (when dropdown is open) |
| `esc` | Close locale dropdown (when open) / return to home screen (when closed) |
| `q` / `ctrl+c` | Quit |

## Locale Support

Collections with **L10N (Localization)** columns display localized field values. The TUI:

1. **Auto-detects Locales:** Scans all records to find available locale codes (e.g., `en`, `de`, `fr`)
2. **Sorts by Language Name:** Locales are sorted alphabetically by their full language name (English, French, German, etc.)
3. **Defaults to English:** Selects `en` locale by default if available
4. **Column Expansion:** L10N columns are expanded from `field: {locale: value}` to a display format `field.locale`
5. **User Switching:** Press `l` or `L` to open a dropdown in the top-right corner of the data
   panel. Use `↑`/`↓` to navigate the list, `enter` to confirm the selection, `esc` to cancel.

Example: A `title` column with L10N type might show values:
- `title.en` → English title
- `title.de` → German title
- `title.fr` → French title

## Data Loading

The records for the collection are loaded asynchronously when the screen opens:

1. **Init Command:** Triggers a load command via DALgo database interface
2. **Records Query:** Executes a query to read all records from the collection
3. **Display Update:** Once loaded, the `recordsLoadedMsg` updates the model and displays results
4. **Error Handling:** Loading errors are silently logged (shown inline on home screen instead)

## Code Reference

- **Files:** `collection_screen.go` (screen logic), `collection_schema_panel.go` (schema panel), `collection_data_panel.go` (data/records panel)
- **Primary Type:** `collectionModel`
- **Key Methods:**
  - `newCollectionModel()` — Constructor
  - `Init()` — Initiates async record loading
  - `Update()` — Handles input and state changes
  - `View()` — Renders the screen
  - `renderSchema()` — Renders left panel
  - `renderRecords()` — Renders right panel
  - `buildSchemaLines()` — Pre-renders schema content
  - `loadRecordsCmd()` — Returns Tea command to load records from database
  - `cellValue()` — Extracts display value from record (handles L10N expansion)
  - `discoverLocales()` — Scans records to find available locales
  - `buildDisplayColumns()` — Expands L10N columns to display format

## Scrolling Behavior

- **Records:** Automatically scrolls to keep cursor visible when navigating up/down
- **Schema:** Supports scroll for content exceeding viewport height

## Column Width Calculation

- Minimum width per column: Auto-calculated based on header and content
- Maximum width per column: 30 characters (longer values truncated with ellipsis)
- Separator adjustments: Uses box-drawing characters (│ and ─) for clean separation
