# bd-tui

A powerful terminal user interface for the [bd-lite](https://github.com/andynu/bd-lite) issue tracker.

> **Note:** This project showcases two things:
> 1. **[bd-lite](https://github.com/andynu/bd-lite)** - A fork and minimization of Steve Yegge's [beads](https://github.com/steveyegge/beads) issue tracker. Beads is an ambitious distributed issue tracking system with SQLite, Dolt, a daemon, RPC, compaction, and multi-repo sync. bd-lite strips it down to basics: same data format, no database, no daemon, no sync — just issues in a JSONL file. If you want the full power, use [beads](https://github.com/steveyegge/beads); if you want something minimal and file-based, use [bd-lite](https://github.com/andynu/bd-lite).
> 2. **AI-Assisted Development** - This TUI is developed primarily by guiding [Claude Code](https://code.claude.com), demonstrating how AI pair programming can build complex, maintainable software.

## Features

### Core Functionality
- **Live monitoring** of `.beads/issues.jsonl` with automatic refresh
- **Dual view modes** - List view (grouped by status) and Tree view (dependency hierarchy)
- **Issue segregation** - Separate views for ready, blocked, and in-progress issues
- **Vim-style navigation** - j/k for movement, gg/G for jumps, familiar keybindings
- **Rich detail panel** - Full issue metadata, dependencies, comments, and acceptance criteria
- **Real-time updates** - Automatically refreshes when JSONL file changes

### Editing & Management
- **Full CRUD operations** - Create, edit, close, and reopen issues without leaving the TUI
- **Full-field editing** - Edit title, description, design, acceptance criteria, and notes via modal dialog (`e` key)
- **Quick updates** - Instant priority with `0`-`4`, status with the `s` chord (`so`/`si`/`sb`/`sc`)
- **Comment system** - Add comments to issues directly from the TUI
- **Dependency management** - Add/remove blocks, parent-child, and related dependencies via dialog
- **Label management** - Add/remove labels through dedicated dialog interface
- **Clipboard integration** - Yank issue IDs (y) or IDs with titles (Y) to clipboard

### Advanced Features
- **Statistics dashboard** - Press S to view issue distribution, priority breakdown, and completion metrics
- **Advanced filtering** - Filter by priority (p0-p4), type (bug, feature, task, epic, chore), status, or labels
- **Search functionality** - Full-text search with n/N navigation through results
- **Panel focus system** - Tab between issue list and detail panel with keyboard scrolling support
- **Mouse mode toggle** - Enable/disable mouse interaction (m key) for terminal text selection
- **Natural language detection** - Automatically detects priority and type keywords when creating issues

### Visual Design
- **Color-coded priorities** - Visual indicators for P0 (critical) through P4 (lowest)
- **Status icons** - ● (ready), ○ (blocked), ◆ (in-progress), · (closed)
- **Type emoji** - 🐛 (bug), ✨ (feature), 📋 (task), 🎯 (epic), 🔧 (chore)
- **Syntax highlighting** - Color-coded dependencies, labels, and metadata
- **Responsive layout** - Side-by-side above 90 columns; below that the list and detail each take the full screen and Tab/Enter/ESC page between them

## Installation

Build from source:

```bash
git clone https://github.com/andynu/bd-lite-tui
cd bd-lite-tui
go build -o bd-tui ./cmd/bd-tui
```

Or run directly:

```bash
go run ./cmd/bd-tui
```

**Note:** This project is not yet published to a package registry. Install from source as shown above.

## Usage

Navigate to a directory containing a `.beads` folder and run:

```bash
./bd-tui
```

The TUI will automatically find the `.beads/issues.jsonl` file in the current or parent directories.

### Debug Mode

Run with comprehensive diagnostic logging:

```bash
./bd-tui --debug
# Logs saved to: ~/.bd-tui/debug-YYYY-MM-DD-HH-MM-SS.log
```

Debug logs include keyboard events, refresh operations, bd command executions, and timing information - useful for diagnosing hangs or performance issues.

## Keyboard Shortcuts

### Navigation
- `j` / `↓` - Move down
- `k` / `↑` - Move up
- `gg` - Jump to top
- `G` - Jump to bottom
- `Space` - Page down, wrapping to the top at the end
- `Ctrl-f` / `Ctrl-b` - Page down / up in the list (vim)
- `Tab` - Focus detail panel for scrolling
- `Enter` - Focus detail panel (when on issue)
- `ESC` / `←` - Return focus to issue list

### Search
- `/` - Start search mode
- `n` - Next search result
- `N` - Previous search result
- `ESC` - Exit search mode

### Quick Actions
- `0-4` - Set priority (P0=critical, P1=high, P2=normal, P3=low, P4=lowest)
- `s` - Status chord prefix; see Two-Character Shortcuts below
- `R` - Rename issue (edit title)
- `a` - Create new issue (vim-style "add")
- `c` - Add comment to selected issue
- `e` - Edit issue (title, description, design, acceptance, notes, priority, type)
- `x` - Close issue with optional reason
- `X` - Reopen closed issue with optional reason
- `D` - Manage dependencies (add/remove blocks, parent-child, related)
- `L` - Manage labels (add/remove labels)
- `A` - Assign issue (set or clear the assignee, autocompletes known names)
- `y` - Yank (copy) issue ID to clipboard
- `Y` - Yank (copy) issue ID with title to clipboard
- `B` - Copy git branch name to clipboard

### Two-Character Shortcuts
- `so` - Set status to open
- `si` - Set status to in_progress
- `sb` - Set status to blocked
- `sc` - Set status to closed

The prefix is lowercase `s`; uppercase `S` opens the stats dashboard. The chord
expires 2 seconds after `s`, and any second key other than o/i/b/c cancels it.

`si` also claims the issue for you when it has no assignee. An issue someone
else holds keeps its assignee.

### Assignees

Issue rows carry a dimmed `@name` when an issue is assigned. Identity is always
`git user.name`; an agent may tag itself after a slash (`andy/claude-xyz`), and
that still counts as andy's. With `user.name` unset, auto-claim and the `mine`
filter are disabled rather than guessing at a name.

- `A` - Assign issue (set or clear, autocompletes from names already in use)
- `f` then `mine` - Only issues assigned to you
- `f` then `unassigned` - Only issues nobody has claimed
- `f` then `@andy` - Only andy's, including `andy/claude-xyz`

### View Controls
- `t` - Toggle between list and tree view
- `o` - Fold/unfold the selected node (tree view, node must have children)
- `O` - Expand all nodes (tree view)
- `Z` - Collapse all nodes (tree view)
- `v` - Cycle layout: auto → columns → rows → paged → auto
- `C` - Toggle showing closed issues in list view
- `p` - Toggle the issue ID prefix (`tui-abc` vs `abc`)
- `f` - Quick filter (type: `p1 bug`, `feature`, etc.)
- `S` - Show statistics dashboard
- `m` - Toggle mouse mode on/off
- `r` - Manual refresh

Fold state persists per repository in `~/.bd-tui/collapse-<hash>.json`.

### Detail Panel (when focused)
- `Tab` / `ESC` / `←` - Return focus to issue list
- `j` / `↓` - Show the next issue, skipping group headers (stops at the end)
- `k` / `↑` - Show the previous issue
- `Ctrl-d` - Scroll down half page
- `Ctrl-u` - Scroll up half page
- `Ctrl-f` - Scroll down full page (vim)
- `Ctrl-b` - Scroll up full page (vim)
- `Ctrl-e` - Scroll down one line
- `Ctrl-y` - Scroll up one line
- `PageDown` - Scroll down full page
- `PageUp` - Scroll up full page
- `Home` - Jump to top of details
- `End` - Jump to bottom of details

### General
- `?` - Show help screen
- `q` - Quit

## Quick Filter Syntax

The filter dialog (`f` key) supports natural language filtering:

```
p0-p4          Priority (e.g., 'p1' or 'p1,p2')
bug, feature, task, epic, chore    Types
open, in_progress, blocked, closed    Statuses
#label         Label (e.g., '#ui' or '#bug,#urgent')
```

**Examples:**
- `p1 bug` - P1 bugs only
- `feature,task` - Features and tasks
- `p0,p1 open` - High priority open issues
- `#ui #urgent` - Issues with 'ui' or 'urgent' labels

Leave empty to clear all filters.

## Status Indicators

- ● (green) - Ready to work on
- ○ (yellow) - Blocked by dependencies
- ◆ (blue) - In progress
- · (gray) - Closed

## Priority Colors

- `[P0]` - Critical (red)
- `[P1]` - High (orange)
- `[P2]` - Normal (light blue)
- `[P3]` - Low (gray)
- `[P4]` - Lowest (gray)

## Type Emoji

- 🐛 - Bug
- ✨ - Feature
- 📋 - Task
- 🎯 - Epic
- 🔧 - Chore

## Project Structure

```
bd-lite-tui/
├── cmd/
│   ├── bd-tui/          # Main application
│   │   ├── main.go      # Flags, config, theme, logging, signals only
│   │   ├── tui.go       # newTUI: widgets, keybindings, refresh plumbing
│   │   ├── status_bar.go    # Status line assembly and the .beads path label
│   │   ├── bd_helpers.go    # execBd / execBdJSON shell-outs to bd
│   │   ├── dialogs.go       # DialogHelpers struct wiring the dialogs
│   │   └── dialog_*.go      # One file per modal (create, edit, close, ...)
│   └── list-themes/     # Prints the registered themes
├── internal/
│   ├── clipboard/       # xclip/pbcopy/clip wrapper
│   ├── config/          # ~/.bd-tui/config.json and per-repo collapse state
│   ├── formatting/      # Color schemes, status formatting, detail rendering
│   ├── identity/        # Who the TUI acts as (git user.name)
│   ├── layout/          # Columns/Rows/Paged decision from terminal width
│   ├── parser/          # JSONL parser for bd-lite issues
│   ├── state/           # Issue categorization, filtering, tree building
│   ├── storage/         # .beads discovery and the JSONL file reader
│   ├── theme/           # Built-in and TOML themes
│   ├── ui/              # Issue list and tree rendering
│   └── watcher/         # Filesystem monitoring with debouncing
└── go.mod
```

`main.go` is a thin entry point. The TUI itself lives in `tui.go` and the
`dialog_*.go` files, so `newTUI` can be built against a tcell `SimulationScreen`
in tests. Search `tui.go`, not `main.go`, for behavior.

## Dependencies

- [tview](https://github.com/rivo/tview) - Terminal UI framework
- [tcell](https://github.com/gdamore/tcell) - Low-level terminal control
- [fsnotify](https://github.com/fsnotify/fsnotify) - Filesystem monitoring
- [toml](https://github.com/BurntSushi/toml) - Parsing TOML theme files
- [term](https://pkg.go.dev/golang.org/x/term) - Terminal size and capability queries

Clipboard access is `internal/clipboard`, which shells out to xclip/xsel/wl-copy
rather than taking a dependency, and can write an OSC 52 escape sequence instead
when there is no local display to reach. See
[Clipboard over SSH](#clipboard-over-ssh).

## Troubleshooting

### No issues displayed

The TUI reads directly from `.beads/issues.jsonl`. If no issues appear:

1. **Check JSONL file exists:**
   ```bash
   ls -la .beads/issues.jsonl
   ```

2. **Verify issues exist:**
   ```bash
   bd list
   ```

3. **Force a refresh:**
   - Press `r` in the TUI for manual refresh
   - Check debug logs with `--debug` flag

### TUI not updating after bd commands

The TUI watches the JSONL file for changes. Updates should appear within ~200ms.

**If updates don't appear:**
1. Check file watcher is running (no errors on startup)
2. Force manual refresh with `r` key
3. Run with `--debug` to check watcher events

### File not found error

Two different startup failures name a path, and they want different fixes.

`.beads directory not found` means there is no store in this directory or any
parent:

```bash
bd init                      # Initialize bd-lite if needed
bd init --prefix myproj      # ...with an explicit issue ID prefix
```

`.beads/issues.jsonl not found` means the directory is there but the store file
is not. `bd init` cannot fix that one; it refuses to run once `.beads` exists.

```bash
bd create "First issue"      # writes the store
touch .beads/issues.jsonl    # ...or open an empty board
```

### Clipboard over SSH

`y`, `Y` and `B` write to the clipboard of the machine running bd-tui, using
xclip, xsel or wl-copy. Over SSH that is usually the wrong machine, and on a
remote host with a display of its own the copy will succeed while putting the
text on a clipboard nobody can see.

`BD_TUI_CLIPBOARD` picks the route:

| Value | Behavior |
|-------|----------|
| `auto` (default) | Local display helper, falling back to OSC 52 when the host has no display |
| `helper` | Local display helper only |
| `osc52` | The terminal's own clipboard sequence only, even when a local display exists |

OSC 52 asks the terminal emulator to set the clipboard of the machine you are
sitting at. Set `BD_TUI_CLIPBOARD=osc52` when you SSH into a host that has its
own display, since auto mode would otherwise never reach for it.

Nothing acknowledges an OSC 52 write. The terminal sends no reply, so bd-tui
reports it as `→ Sent tui-abc to clipboard via OSC 52 (unverified)` rather than
with a checkmark. The text has very likely arrived; nothing in the process saw
it arrive. Paste to find out.

What bd-tui does check first, because these are the ways the sequence provably
goes nowhere:

- there is a terminal at all (`/dev/tty` opens)
- inside tmux, an attached client has a non-empty `client_tty`
- inside tmux, either `allow-passthrough` is on (the sequence is forwarded
  verbatim) or `set-clipboard` is `on`/`external` and the client's terminal
  advertises the `clipboard` feature

If your terminal wants BEL rather than `ESC \` to end an escape sequence, set
`BD_TUI_OSC52_TERMINATOR=bel`; `st8` sends the single-byte 8-bit terminator.
Both settings reject an unrecognised value rather than quietly using the
default.

### Editor integration

The TUI uses built-in text areas for editing. Press `e` to open the edit dialog with fields for title, description, design, acceptance criteria, and notes.

There is no `$EDITOR` suspend-and-spawn path. The modal-dialog approach replaced
the original plan for one, so adding it would be new work rather than finishing
something started.

## Development

This project uses [bd-lite](https://github.com/andynu/bd-lite) for its own issue tracking (dogfooding). View current work:

```bash
bd ready             # Show ready issues
bd list              # Show all issues
bd dep tree tui-xyz  # Show the dependency tree for one issue (the id is required)
bd show tui-xyz      # Show issue details, comments included
```

**The whole bd-lite command set** is `cleanup`, `close`, `comment`, `create`
(alias `new`), `dep` (`add`/`remove`/`tree`), `init`, `list`, `ready`, `show`,
`update`. Anything else you remember from full beads — `add`, `comments`,
`label`, `export`, `sync`, `status`, `search` — does not exist here. Check with
`bd <cmd> --help` before putting a command in docs or in code.

### Running Tests

```bash
go test ./...                          # All tests
go test -coverprofile=coverage.out ./... # With coverage
go tool cover -html=coverage.out       # View coverage report
```

### Development Commands

```bash
# Build
go build -o bd-tui ./cmd/bd-tui

# Run with debug logging
./bd-tui --debug

# Create new issue
bd create "Issue title" -p 1 -t feature

# Update issue
bd update tui-xyz --status in_progress

# Add comment
bd comment tui-xyz "Your comment text"
```

## Architecture

### Data Flow

**Startup:**
1. Find `.beads` directory (current dir or walk up parent dirs)
2. Parse `.beads/issues.jsonl` via JSONL parser
3. Load issues and categorize (ready/blocked/in-progress/closed)
4. Build tview UI with populated lists
5. Start fsnotify watcher on JSONL file
6. Display TUI

**Live updates:**
1. User runs `bd` command (e.g., `bd create`, `bd update`)
2. bd writes to `.beads/issues.jsonl`
3. fsnotify detects file write
4. Watcher debounces (200ms) and triggers refresh
5. Re-parse JSONL, update state, redraw UI
6. TUI updates automatically

**Issue categorization:**
- **Ready:** Open issues with no open blocking dependencies
- **Blocked:** Open issues with unresolved blocking dependencies OR status="blocked"
- **In Progress:** Issues with status="in_progress"
- **Closed:** Issues with status="closed" (hidden by default, toggle with C)

### Package Responsibilities

**`cmd/bd-tui/`** - Main application
- `main.go`: flags (`--debug`, `--theme`, `--view`, `--issue`), config and theme
  resolution, log setup, signal handling. No UI code.
- `tui.go`: `newTUI` builds the whole app from a `tuiOptions` struct — status
  bar, issue list, detail panel, keybindings, watcher, refresh plumbing
- `status_bar.go`: status line assembly and the shortened `.beads` path label
- `bd_helpers.go`: `execBd`/`execBdJSON`, the only place that shells out to `bd`
- `dialogs.go` + `dialog_*.go`: the modal dialogs, one file each

**`internal/clipboard/`** - Clipboard access
- Shells out to xclip/xsel/wl-copy, keeping the helper's stderr
- `osc52.go` writes the terminal's own clipboard escape sequence for sessions
  with no local display, and reports the result as unconfirmed because the
  terminal acknowledges nothing

**`internal/config/`** - Persistence outside the repo
- `~/.bd-tui/config.json` holds the theme
- `~/.bd-tui/collapse-<hash>.json` holds tree collapse state, keyed by a hash of
  the `.beads` path so each repo keeps its own

**`internal/formatting/`** - Presentation logic
- Color schemes for priority/status/type
- Detail panel formatting
- Status icon and emoji rendering

**`internal/identity/`** - Who the TUI acts as
- Reads `git user.name`; unset disables auto-claim and the `mine` filter rather
  than guessing at a name
- Treats `andy/claude-xyz` as andy, which `bd list --assignee` cannot do

**`internal/layout/`** - Screen arrangement
- Pure `Resolve(width, pin, pinned)` returning Columns, Rows, or Paged
- Below 90 columns the answer is always Paged, pin or no pin

**`internal/parser/`** - JSONL parsing
- Domain types matching bd-lite schema
- Line-by-line JSONL reader

**`internal/state/`** - Business logic
- Issue categorization (ready/blocked/in-progress/closed)
- Dependency graph building
- Filter and search logic
- Tree view structure building

**`internal/storage/`** - Data access
- `FindBeadsDir` walks up from the working directory to locate `.beads`
- `JSONLReader` reads and parses `.beads/issues.jsonl`

**`internal/theme/`** - Colors
- Built-in themes and TOML theme files, selected by `--theme`, `$BEADS_THEME`,
  or `~/.bd-tui/config.json`. There is no theme-cycling key.

**`internal/ui/`** - UI helpers
- `PopulateIssueList` rebuilds the issue list; `renderTreeNode` draws tree view

**`internal/watcher/`** - File monitoring
- fsnotify wrapper with 200ms debouncing
- Triggers refresh callback on file writes

## Integration with bd-lite

This TUI provides full CRUD operations for bd-lite issues. All modifications use the `bd` command under the hood:

```bash
# All these work from within the TUI via dialogs/keybindings
bd create "New issue" -p 1
bd update bd-a1b2 --status in_progress
bd comment bd-a1b2 "Added comment"
bd dep add bd-a1b2 bd-xyz3          # bd-a1b2 depends on bd-xyz3
bd update bd-a1b2 --labels "urgent" # the whole label set, not a delta
bd close bd-a1b2 --reason "Fixed in v1.0"

# Changes appear in TUI within ~200ms
```

The TUI is both a viewer and a full-featured editor for bd-lite issues.

## License

MIT
