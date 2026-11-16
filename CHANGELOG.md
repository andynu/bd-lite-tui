# Changelog

All notable changes to bd-tui will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2026-11-16

Requires bd-lite v0.1.0, its first tagged release. bd-tui still starts against an older or unrecognised bd, but says so.

### Added

- **bd version check**: at startup bd-tui runs `bd --version` and compares it with the range it supports (v0.1.0 up to the v0.1 minor line). A missing bd, one too old to report a version, an older build, or a newer minor release bd-tui has not been tested with each produce a `[bd: ...]` field after the issue count in the status bar. A compatible bd shows nothing. A missing bd used to be reported only on stderr, just before the TUI took over the screen
- **bd update notice**: when bd reports that a newer release exists, the same field reads `[bd: vX available]`. It appears after the first bd command, since the startup check keeps bd's own update check off so launch never waits on GitHub

### Changed

- Issue groups and tree roots follow `bd ready` order (priority, then oldest first, then ID), instead of the order of `issues.jsonl`. bd-lite 0.1.0 writes that file by author and then ID, which put each group in author order and then in effectively random order. A bare-prefix search now jumps to the first match as the list draws it

### Fixed

- The dependency dialog (`D`) reported a failure after every successful add or remove, stayed open and skipped its refresh, because bd answers `dep add` and `dep remove` with a message instead of the changed issue
- A failed bd command showed bd's update notice in the status bar instead of the error, because the notice comes first on stderr and the status bar is one line
- The dependency dialog accepted an issue depending on itself, which bd-lite stores and which leaves the issue blocked for good
- In tree view an epic inside another epic became its own root, and dropped out of the outer epic, whenever it happened to be built first
- CI's lint job had failed since August on an unchecked error in a clipboard test

## [0.4.1] - 2026-09-10

### Fixed

- Action keys did nothing while the detail panel had focus: `c`, `e`, `x`, `A`, `D`, `L`, the `s` chord, `0`-`4`, `y`, `q` and the rest of the list bindings were passed to the panel, which ignored them. 0.4.0 made that a common state, since a click in the Paged layout focuses the detail and j/k browse issues without leaving it. These keys now act on the issue the panel shows. The panel keeps j/k, `g`/`G`, `h`/`l` and Space, and the second key of an `s` chord still goes to the chord

## [0.4.0] - 2026-09-10

### Added

#### Views
- **Created view** and **Updated view** — issues grouped by relative time buckets (Today, Yesterday, This week, Last week, This month, Older) keyed on creation or last-touch time. `t` now cycles list → tree → created → updated → list, and `--view` accepts all four
- **Paged layout** for narrow terminals (<90 columns) — list and detail each take the full screen, switched with Tab/Enter/ESC/Left; `v` cycles a layout pin (auto → Columns → Rows → Paged → auto)

#### Assignee & attribution
- **Assign dialog** (`A`) — set or clear an issue's assignee, with autocomplete from names already in use
- **Auto-claim on `si`** — moving an issue to in_progress claims it for `git user.name` when it has no assignee
- **Assignee filter** in the quick filter (`@name`, `mine`, `unassigned`)
- A dimmed `+name` on a row shows who filed it, when that isn't the current identity

#### Dependency navigation
- **Click a dependency line** in the detail panel to jump the list selection to that issue, revealing it first if it's closed, filtered out, or folded away
- Dependency lines in the detail panel now show the target's **status and title**, not just its bare ID
- Tree view nests an epic's outbound `blocks` targets under it as children, instead of losing them as stray roots
- **Row glyph `◌`** distinguishes an issue blocked by an open dependency from one explicitly set to `blocked` (`○`)
- The status bar names the remaining blocker (", still blocked by X") when a status change doesn't fully unblock an issue

#### Detail panel & navigation
- Left arrow leaves the detail panel back to the list
- Up/Down and j/k browse between issues while the detail panel is focused, skipping group headers
- Clicking a list row in Paged mode opens its detail (matching Enter)

#### Clipboard
- **OSC 52 clipboard route** for remote/SSH sessions, selectable via `BD_TUI_CLIPBOARD` (auto/helper/osc52)
- Clipboard failures now recover the display from tmux and report the underlying helper error instead of a bare exit code

#### Other
- Status-bar confirmations now stay visible for their full duration across the reload they provoke, instead of being overwritten within a fraction of a second
- Status legends (help screen and README) corrected to list `✓` for closed issues, with a test pinning the legends to the glyphs rows actually draw
- CI now fails on gofmt drift (`make fmt-check`)
- A SimulationScreen-based integration test harness drives the real app end to end (key chords, refresh, search, dialogs), taking `cmd/bd-tui` coverage from 3.3% to 55.3%

### Changed
- The status chord (`s` then `o`/`i`/`b`/`c`) replaces plain status cycling
- After a status chord moves a row to another group, the cursor steps to the neighboring row in the old group instead of following the changed issue
- Clipboard handling moved from `atotto/clipboard` to `internal/clipboard`
- Status bar path is shortened (home-relative, long paths elided) so the focus and layout fields stay visible on narrow terminals
- Removed dead/unreachable code: the unused `internal/ui/components.go` two-column layout and `internal/app.AppContext`
- README, CLAUDE.md and the help screen were brought back in line with the actual keybindings and architecture several times over this window

### Fixed
- File watcher stalled permanently after `.beads` directory recreation (checkout/stash/rebase, worktree swaps, `bd init`)
- Issue refresh serialized behind 10s timeouts and leaked a goroutine per tick under load; refresh is now single-flight and non-blocking
- Bracketed tags (`[P1]`, layout/view indicators) were parsed as tview color tags and swallowed instead of rendering
- The quick filter dialog didn't repaint the list after applying or clearing
- Enter on a dialog button (filter, close) fell through to the wrong handler because a form-wide capture swallowed it
- Search couldn't find issues hidden by a collapsed tree ancestor, an active filter, or the closed toggle; it now reveals them
- A rune-indexed `ToLower` corrupted any string with multi-byte characters, including the emoji status/type icons
- 10 bugs from a code-review sweep: unescaped titles/labels breaking display, a lost `--issue` filter on refresh, bd stderr corrupting JSON parsing, a stale `gg` prefix, blocked-state not propagating through dotted-ID parents, and others
- A `.beads` directory with no `issues.jsonl` yet was a hard startup error instead of an empty board
- The startup hint for a missing issue store pointed at `bd export`, which doesn't exist in bd-lite
- An open dialog could end up underneath the issue list after a resize-triggered layout rebuild, taking keyboard input nowhere visible

## [0.3.0] - 2026-02-10

### Fixed
- **ESC in detail panel** no longer hides the pane — it returns focus to the issue list, matching the symmetric Tab-in/ESC-out expectation
- **Tree view indentation** — collapse indicator `[-]` was being eaten by tview's color parser, causing misaligned tree branches. Replaced with unicode triangles (▶/▼)
- **O(n²) parent detection** in tree builder replaced with O(n) map-based lookup, preventing slowdowns with large issue counts

### Added
- **Position indicator** in issue list title (e.g., "Issues [List] 3/42") updates on navigation
- **Empty state guidance** — shows "Press 'a' to create an issue" when list is empty, or "Press 'f' to modify filters" when filters produce zero results
- **Quick Start section** at top of help screen with the 6 essential keys for new users
- **Filter active banner** — prominent "⊘ FILTERED" indicator at top of issue list when filters are active
- **Startup check for bd CLI** — warns if `bd` is not in PATH so users know issue updates won't work
- **Max recursion depth** (50) in tree builder to prevent stack overflow with pathological dependency chains

### Changed
- Extracted `showTemporaryStatus` helper, replacing 9 repeated timer patterns (-40 lines)
- Replaced magic duration values with named constants (`statusMessageDuration`, `refreshDelay`, `queueUpdateTimeout`, `dbLoadTimeout`, `watcherDebounce`)
- Tree view collapse indicators changed from `[+]`/`[-]` to ▶/▼

## [0.2.1] - 2026-01-10

### Fixed
- Fix comment dialog failing to parse bd JSON response when daemon warnings are present. The `bd comment` command writes deprecation and daemon warnings to stderr, which corrupted JSON parsing when using `CombinedOutput()`. Now captures stdout and stderr separately.

## [0.2.0] - 2025-12-12

### Added

#### Tree View Enhancements
- **Collapsible nodes** (`o`) - Collapse/expand parent nodes in tree view (vim-style fold)
  - `[+]` indicator for collapsed nodes with child count
  - `[-]` indicator for expanded nodes
  - Leaf nodes maintain visual alignment
- **Expand/collapse all** - Bulk operations for tree nodes
  - `O` (Shift-O) - Expand all nodes
  - `Z` (Shift-Z) - Collapse all nodes
- **Smart collapse defaults** - Nodes auto-collapse/expand based on activity:
  - Auto-expand nodes with active work (any child is `in_progress`)
  - Auto-collapse nodes without active work (all children closed or pending)
  - Manual toggles override smart defaults
- **Collapse state persistence** - Remember collapse preferences between sessions
  - Saved to `~/.bd-tui/collapse-<hash>.json` (per-project)
  - Loads on startup, saves on toggle and exit

#### UI Improvements
- **View mode hint in panel title** - Shows current view mode (e.g., "Issues [List] (t:Tree)")
- Cleaner status bar (removed redundant view mode indicator)

## [0.1.0] - 2025-11-25

Initial release of bd-tui, a terminal user interface for the [bd-lite](https://github.com/steveyegge/bd-lite) issue tracker.

### Features

#### Core Functionality
- Real-time monitoring of bd-lite database with automatic UI refresh
- Direct SQLite database reading for instant updates
- Filesystem watching with debounced refresh (200ms)
- Graceful shutdown with signal handling

#### Views
- **List View**: Issues grouped by status (ready/blocked/in-progress)
- **Tree View**: Dependency hierarchy with ASCII tree visualization
- Toggle between views with `t` key
- Stats dashboard overlay with `S` key

#### Navigation (Vim-style)
- `j`/`k` - Move up/down
- `g` `g` / `G` - Jump to top/bottom
- `Ctrl-d`/`Ctrl-u` - Half-page scroll in detail panel
- `Ctrl-b`/`Ctrl-f` - Full-page scroll
- `/` - Search with `n`/`N` for next/previous match
- `Tab`/`Enter` - Focus detail panel, `ESC` to return

#### Issue Management
- **Create issues** (`a`) - Form-based dialog with natural language detection for priority/type
- **Edit issues** (`e`) - Form-based editing of all fields (description, design, acceptance, notes)
- **Quick priority** (`0`-`4`) - Instantly set priority P0-P4
- **Status cycling** (`s`) - Cycle through open → in_progress → blocked → closed
- **Close/reopen** (`x`/`o`) - Quick issue status changes
- **Add comments** (`c`) - Comment dialog with keyboard shortcuts

#### Dependency Management
- **Dependency dialog** (`D`) - Add/remove blocking and parent-child relationships
- Human-readable dependency phrases ("blocked by", "child of" instead of raw types)
- Visual dependency indicators in list and tree views
- Automatic blocked status detection based on open dependencies
- Blocking propagates through parent-child relationships (matches `bd ready` behavior)

#### Labels
- **Label management** (`L`) - Add/remove labels from issues
- Label display with hashtag prefix in issue list
- Filter by label in quick filter

#### Filtering
- **Quick filter** (`f`) - Filter by status, priority, type, or label
- **Show closed** (`C`) - Toggle visibility of closed issues
- Persistent filter state during session

#### Themes
- 10+ built-in color themes (Gruvbox Dark default)
- High-contrast and colorblind accessibility themes
- TOML-based theme files with embed.FS
- Theme configuration via `~/.config/bd-tui/config.toml`

#### Clipboard
- `y` - Yank issue ID to clipboard
- `Y` - Yank issue ID with title
- Click issue ID in detail panel to copy

#### Other Features
- Help screen (`?`) with all keyboard shortcuts
- Mouse mode toggle (`m`) for terminal text selection
- Debug logging (`--debug`) for troubleshooting
- Type emoji icons in issue list
- Keyboard shortcut hints in dialog titles

### Technical

- Built with [tview](https://github.com/rivo/tview) terminal UI framework
- Uses [fsnotify](https://github.com/fsnotify/fsnotify) for file watching
- SQLite database access via [go-sqlite3](https://github.com/ncruces/go-sqlite3)
- Cross-platform: macOS (Intel/Apple Silicon) and Linux (amd64/arm64)

[0.4.1]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.4.1
[0.4.0]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.4.0
[0.3.0]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.3.0
[0.2.1]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.2.1
[0.2.0]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.2.0
[0.1.0]: https://github.com/andynu/bd-lite-tui/releases/tag/v0.1.0
