# TUI (Terminal User Interface)

The inGitDB TUI provides an interactive terminal interface for browsing, searching, and previewing collections and records. It's launched when running `ingitdb` with no subcommand inside an inGitDB repository.

## Screens

| Screen | File | Description |
|--------|------|-------------|
| [Home](home_screen.md) | `home_screen.go` | Main entry screen with collections list (left), records table (middle), and schema (right); only one panel focused at a time |
| [Collection](collection_screen.md) | `collection_screen.go` | Detailed collection view with schema definition (left) and records table (right); only one panel focused at a time |

## Panel Focus System

**All screens implement a single-focus model:** only one panel can be focused (active) at a time. The focused panel:

- Receives visual distinction (highlighted border in styles)
- Handles input from arrow keys and navigation commands
- Shows cursor/selection state
- Uses `←` / `→` keys to move focus between panels

When a panel loses focus, its cursor/selection state is preserved, allowing users to navigate between panels without losing their position in the data.

## Architecture

The TUI is built with:

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — Terminal UI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** — Styling and layout
- **[DALgo](https://github.com/dal-go/dalgo)** — Database abstraction layer for reading records
- **[Uniseg](https://github.com/rivo/uniseg)** — Unicode grapheme cluster segmentation

## Root Model

The root `Model` type in `model.go` manages screen transitions and coordinates input/output between screens:

- **Current Screen:** Tracks which screen is active (home or collection)
- **Screen Instances:** Owns `homeModel` and optional `collectionModel`
- **Input Delegation:** Routes keyboard and system events to the active screen
- **State Management:** Handles width/height window resize events

## Navigation Flow

```
Home Screen
  ↓ (press enter on collection)
  └→ Collection Screen
       ↓ (press esc)
       └→ Back to Home Screen
```

## Key Features

- **Filterable Collections:** Type to search by collection ID
- **Live Preview:** Schema and sample records appear instantly in a side panel
- **Locale-Aware Display:** Select language via a dropdown in the data panel top-right corner
  (press `l` to open)
- **Unicode Support:** Proper handling of multi-byte characters, emoji flags, and regional indicators
- **Responsive Layout:** Panels resize based on terminal width/height
- **Keyboard Navigation:** Vi-keys (`hjkl`) and arrow keys supported

## Entry Point

The TUI is launched from `cmd/ingitdb/main.go`:

```go
// When no subcommand is provided in an inGitDB repository
tui.New(dbPath, definition, newDB, width, height)
```

Pass the database path, schema definition, database constructor, and initial terminal dimensions. The TUI handles all subsequent rendering and input.

## Locale Selector Convention

Whenever a locale/language selector is needed in the TUI, it appears in the **top-right corner** of
the relevant panel's header. It displays the current locale code with a chevron indicator
(e.g., `[ en ▼ ]`). Pressing `l` opens the dropdown menu; use `↑`/`↓` to navigate the list,
`enter` to confirm the selection, `esc` to cancel and close the dropdown without changing the locale.

## Styling

All UI elements use a consistent style defined in `styles.go`:

- **Panels:** Bordered containers with styled titles
- **Selection:** Highlight current cursor position
- **Headers:** Bold section titles and column headers
- **Separators:** Box-drawing characters for clean divisions
- **Status:** Muted text for secondary information

## State Persistence

State is maintained only during the TUI session. On exit:

- All state is discarded
- No data is written unless a record CRUD operation is performed via the DALgo database interface

## Testing

Tests are located in `home_test.go` and focus on filter logic and state transitions. See test files for examples of constructing and updating models.
