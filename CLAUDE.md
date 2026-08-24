# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**bd-tui** is a terminal user interface for the [bd-lite](https://github.com/steveyegge/bd-lite) issue tracker. It provides real-time monitoring of `.beads/issues.jsonl` with vim-style navigation and displays issues segregated by ready/blocked/in-progress status.

This project uses bd-lite for its own issue tracking (dogfooding).

## Development Commands

### Build and Run

```bash
# Build the TUI
# IMPORTANT: Always build to the project root, not cmd/bd-tui/bd-tui
go build -o bd-tui ./cmd/bd-tui

# Run in current directory (finds .beads automatically)
./bd-tui

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Formatting

CI fails the `lint` job when any Go file needs gofmt, so run this before
committing:

```bash
make fmt-check   # lists drifted files and exits non-zero
make fmt         # rewrites them in place
```

### Working with bd-lite Issues

```bash
# View ready issues
bd ready

# Show all issues
bd list

# View dependency tree for one issue (the id is required)
bd dep tree <issue-id>

# Show issue details (includes comments; there is no `bd comments`)
bd show <issue-id>

# Create new issue (`bd new` is an alias; there is no `bd add`)
bd create "Issue title" -p 1 -t feature

# Add a comment to an issue
bd comment <issue-id> "Your comment text"

# Update issue status
bd update <issue-id> --status in_progress

# Reopen a closed issue
bd update <issue-id> --status open

# Close an issue with reason
bd close <issue-id> --reason "Fixed in v1.0"

# Update labels
bd update <issue-id> --labels "bug,ui,urgent"
```

**Note:** bd-lite writes directly to `.beads/issues.jsonl`. Changes are picked up by the TUI's file watcher automatically.

**The whole bd-lite command set** is `cleanup`, `close`, `comment`, `create`
(alias `new`), `dep` (`add`/`remove`/`tree`), `init`, `list`, `ready`, `show`,
`update`. Anything else you remember from full beads — `add`, `comments`,
`label`, `export`, `sync`, `status`, `search` — does not exist here. Check with
`bd <cmd> --help` before putting a command in docs or in code.

### Debug Logging

The TUI includes comprehensive diagnostic logging to help diagnose hangs and performance issues:

```bash
# Run with debug logging enabled
./bd-tui --debug

# Log file location
~/.bd-tui/debug-YYYY-MM-DD-HH-MM-SS.log
```

**What gets logged:**
- All keyboard input events (key, rune, modifiers, current mode)
- Issue refresh operations (start, JSONL parse, UI update, completion)
- bd command executions (priority/status changes)
- File watcher events (changes detected, refresh triggers)
- Application lifecycle (startup, shutdown, errors)
- Timestamps with microsecond precision
- Source file and line numbers

**Use cases:**
- **TUI hangs**: Examine log to see last keyboard event and any stuck operations
- **Performance issues**: Check refresh timing and JSONL parse duration
- **Command failures**: See exact bd commands executed and error messages
- **Watcher problems**: Verify file changes are detected

**Example log output:**
```
2025/11/14 09:45:23.123456 main.go:78: === bd-tui started in debug mode ===
2025/11/14 09:45:23.234567 main.go:89: Finding .beads directory
2025/11/14 09:45:23.345678 tui.go:538: Setting up file watcher on: /path/.beads/issues.jsonl
2025/11/14 09:45:23.456789 tui.go:552: WATCHER: File watcher started successfully
2025/11/14 09:45:30.123456 tui.go:917: KEY EVENT: key=Rune rune='j' mod=0 searchMode=false detailFocus=false
2025/11/14 09:45:31.234567 tui.go:917: KEY EVENT: key=Rune rune='1' mod=0 searchMode=false detailFocus=false
2025/11/14 09:45:31.345678 tui.go:1396: BD COMMAND: Executing priority update: bd update tui-123 --priority 1
2025/11/14 09:45:31.456789 tui.go:1402: BD COMMAND: Priority update successful for tui-123 -> P1
```

### Testing Considerations

**CRITICAL:** Never pollute the production `.beads/issues.jsonl` with test data. When manually testing, use a separate directory:

```bash
mkdir /tmp/test-beads && cd /tmp/test-beads
bd init --prefix test
bd create "Test issue" -p 1
```

#### TUI integration tests

`cmd/bd-tui/tui_harness_test.go` runs the real application against a tcell
`SimulationScreen`, so key bindings, refreshes, search and dialogs are covered
end to end. `newTUI` (cmd/bd-tui/tui.go) exists to make this possible: it builds
the whole app from a `tuiOptions` struct without starting the event loop.

```bash
go test ./cmd/bd-tui/ -run TestSearch -v
go test -race ./cmd/bd-tui/          # the event loop is concurrent; run this
```

Each test gets a temp `.beads` directory written from a fixture, and the package
var `execBdJSON` is swapped for a fake, so nothing touches the real issue store
or shells out to `bd`.

- `h.press(...)` / `h.typeRunes("gg")` queue keys into the same channel a
  terminal feeds and return once the event loop has handled them. The
  synchronization is an unbound key (`tcell.KeyF64`) queued behind the real
  ones; because tview drains one ordered event channel, its arrival proves the
  earlier keys are done. Never use `time.Sleep` for this.
- `h.read(fn)` runs `fn` on the event loop goroutine. All widget state must be
  read that way, or the race detector will (correctly) complain.
- `h.waitFor(desc, cond)` polls for things the app does on a timer: the 500ms
  post-command reload, the 200ms watcher debounce.
- `h.screenText()` is what actually reached the screen, which is not the same as
  what the code passed to `SetText` — tview parses `[Word]` as a style tag and
  swallows it. Assert on rendered output when the question is "does the user see
  this".
- `h.calls()` returns the recorded `bd` invocations; `h.setBdResponse` installs
  a failing one to cover error paths.

- `withNow(t)` pins the clock the time-keyed views bucket against. Any test
  asserting on a `TODAY`/`THIS WEEK` header needs it, or it passes today and
  fails next week.

Fixtures: `basicIssues()` (three ready, one closed), `treeIssues()` (a
parent-child edge), `createdIssues()` (one issue per time bucket), or build your
own with `newIssue(id, title, withStatus(...), withPriority(...),
withBlockedBy(...), withCreatedAt(...), ...)`.

## Architecture

### High-Level Structure

```
bd-lite-tui/
├── cmd/
│   ├── bd-tui/          # Main application
│   │   ├── main.go      # Flags, config, theme, logging, signals only
│   │   ├── tui.go       # newTUI: widgets, key bindings, refresh plumbing
│   │   ├── status_bar.go    # Status line assembly and the .beads path label
│   │   ├── bd_helpers.go    # execBd / execBdJSON shell-outs to bd
│   │   ├── dialogs.go       # DialogHelpers struct wiring the dialogs
│   │   └── dialog_*.go      # One file per modal (create, edit, close, ...)
│   └── list-themes/     # Prints the registered themes
├── internal/
│   ├── clipboard/       # xclip/xsel/wl-copy wrapper, plus an OSC 52 route
│   ├── config/          # ~/.bd-tui/config.json and per-repo collapse state
│   ├── formatting/      # Colors, status glyphs, detail-panel rendering
│   ├── identity/        # Who the TUI acts as (git user.name)
│   ├── layout/          # Columns/Rows/Paged decision from terminal width
│   ├── parser/          # JSONL parser for bd-lite issues
│   │   ├── types.go     # Issue, Dependency, Comment types
│   │   └── parser.go    # Line-by-line JSONL reader
│   ├── state/           # Issue categorization, filtering, tree building
│   ├── storage/         # .beads discovery and the JSONLReader for issues.jsonl
│   ├── theme/           # Built-in and TOML themes
│   ├── timegroup/       # Timestamp -> relative bucket (Today, This week, ...)
│   ├── ui/              # Issue list, tree, and time-grouped rendering
│   └── watcher/         # fsnotify wrapper with debouncing
└── go.mod
```

`main.go` is a thin entry point. Everything the TUI does lives in `tui.go` and
the `dialog_*.go` files, so `newTUI` can be built against a tcell
`SimulationScreen` from tests. Search `tui.go`, not `main.go`, when looking for
behavior.

### Key Concepts

**JSONL as data source:** The TUI reads directly from `.beads/issues.jsonl`. This file is the git-committed source of truth maintained by bd-lite.

**Issue categorization:** The `state` package implements the core logic for segregating issues:
- **Ready:** Open issues with no open blocking dependencies
- **Blocked:** Open issues with unresolved blocking dependencies OR status="blocked"
- **In Progress:** Issues with status="in_progress"
- **Closed:** Issues with status="closed" (hidden by default; `C` toggles them into list view)

**Filesystem watching:** Uses fsnotify with 200ms debouncing to watch `.beads/issues.jsonl` for changes and refresh the UI automatically.

**Vim-style navigation:** j/k for up/down, q to quit. Details auto-show on navigation (tui-p62).

**Mouse mode enabled by default:** `app.EnableMouse(true)` at startup; `m`
toggles it off when you want the terminal's own text selection back.

A left click on an issue row goes through `issueList.SetSelectedFunc` in
`cmd/bd-tui/tui.go`. In Paged mode it does what Enter does — select the row and
turn the page to that issue's detail — because the detail panel is not mounted
and a click that only moved the highlight showed the user nothing. In
Columns/Rows it just moves the selection. Header rows are ignored. tview calls
the handler before assigning its own `currentItem`, so the handler calls
`SetCurrentItem` itself to get the detail repainted in the right order.

**View modes:** `state.ViewMode` is an N-way cycle, not a toggle:
- **List View (default):** Issues grouped by status (ready/blocked/in-progress)
- **Tree View:** Issues displayed as dependency hierarchy with ASCII tree characters
- **Created View:** the same rows as list view, grouped by when each issue was
  created — `TODAY`, `YESTERDAY`, `THIS WEEK`, and so on, newest bucket first,
  newest issue first within a bucket. Empty buckets are omitted. `C` and `f`
  apply here exactly as in list view.
- **Updated View:** the same buckets keyed on `updated_at`. bd-lite restamps
  that field on every write, so a row changes bucket while the TUI is open;
  `populateIssueList` rebuilds the grouping from scratch on each refresh rather
  than repainting rows in place, which is what moves the row you just edited to
  the top of `TODAY`.
- `t` advances the cycle; `--view list|tree|created|updated` picks the
  starting mode and rejects anything else (`main.go` exits 2 rather than
  falling back to list). `state.NextViewMode` names the next mode without
  switching to it, which is what the list border shows.

`C` and `f` shape the three flat views only. Tree view reads neither: it walks
`s.issues` in `buildDependencyTree` and skips closed issues unconditionally, so
the closed toggle and the quick filter are inert there. Say "the flat views",
not "list view", when documenting either key.

`viewModes` in `internal/state/state.go` is the one list `String`,
`ParseViewMode` and `NextViewMode` all read, so a fifth mode is one entry there,
one case in `PopulateIssueList`, and the three doc places under "Changing
Keybindings" below.

**Layout modes:** `internal/layout` picks Columns, Rows, or Paged from the
terminal width. Below `layout.NarrowThreshold` (90 columns) the result is always
Paged — the list and the detail each take the whole screen and Tab/Enter/ESC
switch between them — even against an explicit pin, because neither split leaves
a readable column. `v` cycles the pin auto → Columns → Rows → Paged → auto; a
pinned mode is marked with `*` in the status bar.

**Panel focus system:** The TUI has two focusable panels (issue list and detail panel):
- **Default:** Issue list is focused (for navigation)
- **Tab or Enter:** Focus detail panel to enable keyboard scrolling
- **ESC or Left arrow:** Return focus to issue list
- **Visual indicators:** Focused panel has yellow border and updated title text
- **Status bar:** Shows current focus (List or Details)
- In Paged mode focus also decides which panel is mounted, so Tab/Enter is how
  you turn the page

**Quick issue updates:** Issues can be updated directly from the TUI without leaving the interface:
- **Priority changes:** Press `0`-`4` to instantly set priority (P0=critical, P1=high, P2=medium, P3=low, P4=backlog)
- **Status chord:** Press `s`, then one of `o`/`i`/`b`/`c` for open, in_progress,
  blocked, closed. The chord resets itself after 2 seconds. (`s` alone no longer
  cycles status, and the prefix is lowercase — uppercase `S` opens the stats
  dashboard.)
- **Auto-claim on `si`:** moving an issue to in_progress also passes
  `--assignee <git user.name>` in the same `bd update` call, but only when the
  issue has no assignee. An issue someone else holds is never reassigned.
- **Immediate feedback:** Confirmation message shown in status bar with green checkmark
- **Auto-refresh:** Issue list automatically refreshes 500ms after update to show changes
- **No dialogs:** Updates are instant with single keypress, no confirmation needed

**Issue creation dialog:** Create new issues directly from the TUI with the `a` key (tui-ywv.6):
- **Modal form:** Centered dialog with title, description, priority, type fields
- **Smart defaults:** Pre-selects P2 priority and feature type
- **Parent relationship:** Optional checkbox to add as child of currently selected issue
- **Form navigation:** Tab between fields, Enter/Ctrl-S to submit, ESC to cancel
- **Validation:** Requires title before allowing creation
- **Command execution:** Executes `bd create` (see `cmd/bd-tui/dialog_create.go`) with form data
- **Natural language detection:** Priority and type keywords in the title
  pre-select the matching form fields (`nldetect_test.go` covers this)
- **Auto-refresh:** Issue list refreshes 500ms after creation to show new issue

### Editing Dialogs

Every write goes through the `bd` CLI rather than a direct JSONL write, so
bd-lite stays the single writer of `.beads/issues.jsonl`. Each dialog lives in
its own `cmd/bd-tui/dialog_*.go` and is reached through the `DialogHelpers`
struct in `dialogs.go`.

| Key | Dialog | File | bd call |
|-----|--------|------|---------|
| `a` | Create issue | `dialog_create.go` | `bd create <title> -p <n> -t <type>` |
| `e` | Edit fields (title, description, design, acceptance, notes, priority, type) | `dialog_edit.go` | `bd update` |
| `R` | Rename (title only) | `dialog_rename.go` | `bd update --title` |
| `x` | Close with optional reason | `dialog_close.go` | `bd close --reason` |
| `X` | Reopen with optional reason | `dialog_close.go` | `bd update --status open` |
| `c` | Add comment | `dialog_comment.go` | `bd comment` |
| `D` | Dependencies (blocks, parent-child, related) | `dialog_dependencies.go` | `bd dep add` / `bd dep remove` |
| `L` | Labels | `dialog_labels.go` | `bd update --labels` (whole set, not a delta) |
| `A` | Assignee (autocompletes names already in use) | `dialog_assign.go` | `bd update --assignee` |
| `f` | Quick filter | `dialog_filter.go` | none, client-side |
| `S` | Stats dashboard | `dialog_stats.go` | none, client-side |
| `?` | Help screen | `dialog_help.go` | none |

Editing is in-TUI only: the fields are tview text areas. There is no `$EDITOR`
suspend-and-spawn path, and the modal-dialog approach replaced the original
plan for one. If you add that, it is new work, not a resumption.

While a dialog is up, `app.SetInputCapture` returns the event untouched
(`pages.GetFrontPage() != "main"`), so the dialog owns all input and the normal
keybindings are inert.

Clipboard writes (`y`, `Y`, `B`, and clicking the issue ID in the detail panel)
go through `internal/clipboard`, which shells out to xclip/xsel/wl-copy, or
writes an OSC 52 escape sequence when there is no local display to reach.

Two routes, two clipboards. A display helper reaches the clipboard of the host
running bd-tui; OSC 52 reaches the clipboard of the machine the user is sitting
at. `BD_TUI_CLIPBOARD` (`auto`/`helper`/`osc52`) picks between them, because
over SSH to a host with its own display the helper succeeds while copying
somewhere useless.

`Result.Confirmed` is the field that matters when touching this code. A helper
exiting cleanly is an acknowledgement, so the status bar earns its checkmark.
OSC 52 is acknowledged by nothing — the terminal never replies — so it reports
`Confirmed: false` and the status bar says "unverified" instead. Do not add a
code path that sets `Confirmed` without something that actually observed the
write. `osc52.go` checks preconditions instead (a terminal exists, a tmux client
has a real `client_tty`, tmux is configured to relay or pass through) and
refuses rather than sending bytes that provably go nowhere.

Do not route this through tcell's `Screen.SetClipboard`. It emits nothing unless
the terminfo entry sets the XT flag, which `tmux-256color` does not, and it
returns no error when it does nothing.

### Package Responsibilities

**`cmd/bd-tui/`** - TUI application
- `main.go`: flags (`--debug`, `--theme`, `--view`, `--issue`), config and theme
  resolution, log file setup, signal handling. No UI code. `--view` takes
  `list`, `tree`, `created`, or `updated`; both the flag's usage string and the
  parse go through `state.ViewModeNames`/`state.ParseViewMode`, so the accepted
  spellings are never typed out here.
- `tui.go`: `newTUI` builds the whole app from a `tuiOptions` struct — status
  bar, issue list, detail panel, `app.SetInputCapture` key bindings, the file
  watcher, and the refresh plumbing — without starting the event loop.
- `status_bar.go`: `formatStatusBar` assembles the status line and
  `statusLocation` shortens the `.beads` path to a label. Both are pure, so the
  field order is a unit test rather than a screenshot. Fields render most useful
  first (focus, layout, filters) because tview clips the tail of the line on a
  narrow terminal.
- `dialogs.go` + `dialog_*.go`: the modal dialogs, one file each.
- `bd_helpers.go`: `execBd`/`execBdJSON`, the only place that shells out to `bd`.
  Tests swap the `execBdJSON` package var.

**`internal/parser/`** - JSONL parsing
- `types.go`: Domain types matching bd-lite JSONL schema (Issue, Status, IssueType, Dependency, Comment)
- `parser.go`: Line-by-line JSONL reader with error handling
- No business logic - pure parsing only

**`internal/state/`** - State management
- Categorizes issues into ready/blocked/in-progress/closed
- Builds dependency graph to detect blocking relationships
- Provides getters for filtered issue lists
- Manages view mode state (list, tree, created, updated)
- Builds tree structure from parent-child and blocks dependencies
- `GroupByTime` (time_groups.go) buckets the visible issues for the time-keyed
  views: filters and the closed toggle applied, `timegroup.All` order, newest
  first inside a bucket, ties broken by ID so a rebuild does not shuffle rows
  under the cursor. `now` is a parameter, never a clock read.
- No UI concerns - pure business logic

**`internal/identity/`** - Who the TUI acts as
- `Resolve(dir)` reads `git user.name`; "" means unknown, which disables
  auto-claim and the "mine" filter rather than guessing
- `Human("andy/claude-xyz")` is "andy": an agent may tag itself after a slash,
  but the value always carries its human
- `IsMine` answers what `bd list --assignee` cannot, since bd matches the whole
  string exactly and would miss the tagged form
- `cmd/bd-tui/tui.go` calls it through the `resolveIdentity` package var, which
  tests swap so no assertion depends on the developer's git config

**`internal/layout/`** - Screen arrangement
- `Resolve(width, pin, pinned)` is a pure function, so the split decision is
  testable without standing up a tview app
- Below `NarrowThreshold` (90) the answer is always `Paged`, pin or no pin

**`internal/timegroup/`** - Relative time buckets
- `Of(t, now)` maps a timestamp to a `Bucket` (Today, Yesterday, This week,
  Last week, This month, Older, Undated). `now` is a parameter, not a clock
  read, so boundary tests are deterministic
- Bucket ordinals ascend newest to oldest; `All()` walks them in header order
- Day and week boundaries are drawn in `now`'s location, and weeks open on
  Monday (ISO 8601)
- A zero timestamp is `Undated` — bd-lite JSONL can omit a time field, which
  parses as the zero time rather than an error. A timestamp ahead of `now` is
  `Today`, since clock skew between machines sharing the JSONL is likelier than
  a real future date

**`internal/formatting/`** - Presentation
- Priority/status/type colors, status glyphs, `FormatIssueDetails` for the
  detail panel

**`internal/config/`** - Persistence outside the repo
- `~/.bd-tui/config.json` holds the theme
- `~/.bd-tui/collapse-<hash>.json` holds tree collapse state, keyed by a hash of
  the `.beads` path so each repo keeps its own

**`internal/watcher/`** - File monitoring
- fsnotify wrapper with debouncing (default 200ms)
- Triggers callback on file write/create events
- Handles cleanup and shutdown

### Data Flow

**Startup:**
1. Find `.beads` directory (current dir or walk up parent dirs)
2. Parse `.beads/issues.jsonl` via `parser.ParseFile()`
3. Load into `state.State` and categorize issues
4. Build tview UI with populated lists
5. Start fsnotify watcher on JSONL file
6. Display TUI

**Live updates:**
1. User runs `bd` command (e.g., `bd create`, `bd update`), or the TUI runs one
   on their behalf
2. bd-lite writes directly to `.beads/issues.jsonl`
3. fsnotify detects JSONL write
4. Watcher debounces (200ms) and triggers `refreshIssues()`
5. Re-parse JSONL, update state, queue UI redraw on main thread
6. TUI updates automatically

**Issue details:**
1. User navigates with j/k
2. `SetChangedFunc` handler fires on list selection change
3. Check if selected item is issue (not header)
4. Format full details (description, dependencies, comments, labels, close_reason, metadata)
5. Update detail panel with scrollable text

### Dependency Detection Algorithm

`State.categorizeIssues` in `internal/state/state.go`, which matches `bd ready`:

1. Build a child→parent map from explicit parent-child dependencies, then infer
   the rest from dotted IDs (`tui-y4h.1` is a child of `tui-y4h`) so list view
   and tree view agree. Explicit deps win over inferred ones.
2. First pass: mark an issue blocked when it has a `blocks` dependency on an
   issue that is not closed.
3. Second pass: propagate blocking down parent-child edges until nothing
   changes, so a child of a blocked parent is blocked too.
4. `related` and `discovered-from` dependencies never block. An explicit
   `status: blocked` does *not* propagate to children.

The result is stored in `s.effectivelyBlocked` and read back through
`IsEffectivelyBlocked()`.

An issue is "blocked" if:
- It has status="blocked", OR
- It has a "blocks" dependency where the blocking issue is not closed, OR
- Its parent is blocked by one of the above

### Tree View Algorithm

The tree view visualizes issue dependencies as a hierarchical structure:

**Building the tree** (`State.buildDependencyTree` in `internal/state/state.go`):
1. Build maps of parent→children (explicit parent-child deps plus dotted-ID
   children) and blocker→blocked (from blocks dependencies)
2. Find root nodes: epics are always roots; everything else is a root only when
   it has no incoming parent-child or blocks dependency
3. Recursively build tree from roots (epics first), adding both children and blocked issues
4. Skip closed issues in tree view
5. Track visited IDs for cycle detection, capped at `maxTreeDepth` (50)

**Rendering the tree** (`renderTreeNode` in `internal/ui/render.go`):
- Uses ASCII tree characters: `├──`, `└──`, `│`
- Status indicators: `●` (open), `○` (blocked), `◆` (in-progress)
- Color-coded by priority and status
- Hierarchical indentation based on depth
- `o`/`O`/`Z` fold and unfold nodes; the collapsed set persists per repo in
  `~/.bd-tui/collapse-<hash>.json`

**Example tree output:**
```
DEPENDENCY TREE
[●] tui-ywv [P1] Build bd-tui: Terminal UI for bd-lite issue tracker
├── [◆] tui-bne [P2] Add tree view mode for issue dependency visualization
├── [●] tui-hxu [P2] Add filtering capabilities (priority, type, status)
└── [●] tui-79b [P2] Write tests for core TUI components
```

## Common Development Tasks

### Adding a New Filter or View

1. Update `state.FilterMode` enum in `internal/state/state.go`
2. Add filter logic to `state.categorizeIssues()` or new getter method
3. Update `cmd/bd-tui/tui.go` to add keybinding and UI handler
4. Update status bar text to show active filter

### Adding Issue Details

1. Update `parser.Issue` struct in `internal/parser/types.go` if parsing new fields
2. Update `formatting.FormatIssueDetails()` in `internal/formatting/details.go`
3. Use tview color markup: `[color]text[-]` for foreground, `[::b]text[-::-]` for bold

### Changing Keybindings

Update `app.SetInputCapture()` in **`cmd/bd-tui/tui.go`** (not `main.go` — the
TUI moved out of it). Three other places describe the same bindings and drift
apart easily; change all of them together:

- `cmd/bd-tui/dialog_help.go` — the `?` help screen
- `README.md` — the "Keyboard Shortcuts" section
- this file

`cmd/bd-tui/tui_keys_test.go` is where a new binding gets its test.

The capture is layered: modal dialogs get every key untouched, then search mode,
then detail-panel-focused mode, then normal mode.

**Issue List Mode:**
- `q`: Quit (persists collapse state first)
- `ESC`: Clear search matches; with none, a second ESC within 1s quits
- `r`: Manual refresh
- `j` / `k`: Down / up (re-emitted as arrow keys)
- `Space`: Page down, wrapping to the top at the end
- `Ctrl-b` / `Ctrl-f`: Page up / down (vim style)
- `g` + `g`: Jump to top
- `G`: Jump to bottom
- `/`: Start search mode (ID, title, labels; jumps to hidden matches too)
- `n` / `N`: Next / previous search result
- `t`: Cycle view mode (list → tree → created → updated → list)
- `o`: Fold/unfold the selected node (tree view only, needs children)
- `O`: Expand all nodes (tree view only)
- `Z`: Collapse all nodes (tree view only)
- `v`: Cycle layout pin (auto → Columns → Rows → Paged → auto)
- `C`: Toggle showing closed issues (flat views only; tree view always hides
  them)
- `m`: Toggle mouse mode on/off
- `p`: Toggle issue ID prefix display (tui-abc vs abc)
- `f`: Quick filter dialog
- `S`: Stats dashboard
- `?`: Help screen
- `0`-`4`: Quick priority change (set current issue to P0-P4)
- `s` then `o`/`i`/`b`/`c`: Set status open / in_progress / blocked / closed.
  The prefix is lowercase and expires after 2s. `si` also claims an unassigned
  issue for `git user.name`.
- `a`: Create issue dialog
- `A`: Assign dialog (set or clear the assignee)
- `e`: Edit dialog (title, description, design, acceptance, notes, priority, type)
- `R`: Rename dialog (title only)
- `x` / `X`: Close / reopen dialog, with optional reason
- `c`: Comment dialog
- `D`: Dependency dialog
- `L`: Label dialog
- `y`: Yank issue ID to clipboard
- `Y`: Yank "id - title" to clipboard
- `B`: Yank git branch name to clipboard
- `Tab`: Focus detail panel for scrolling
- `Enter`: Focus detail panel (when on an issue)

**Detail Panel Mode (when focused):**
- `Tab`, `ESC`, or `Left`: Return focus to issue list (in Paged mode this turns
  the page back to the list)
- `j` / `Down`, `k` / `Up`: Move the list selection to the next/previous issue
  and repaint the detail for it, keeping focus here. Group headers are skipped
  and the ends stop rather than wrap, which makes the detail a pager over the
  issue set in Paged mode. Line scrolling is on `Ctrl-e`/`Ctrl-y`.
- `Ctrl-d`: Scroll down half page (vim style)
- `Ctrl-u`: Scroll up half page (vim style)
- `Ctrl-f`: Scroll down full page (vim style)
- `Ctrl-b`: Scroll up full page (vim style)
- `Ctrl-e`: Scroll down one line (vim style)
- `Ctrl-y`: Scroll up one line (vim style)
- `PageDown`: Scroll down full page
- `PageUp`: Scroll up full page
- `Home`: Jump to top of details
- `End`: Jump to bottom of details

**Search Mode (after `/`):** printable runes extend the query, Backspace trims a
full rune, Enter runs the search, ESC abandons it.

**Unbound on purpose:** there is no theme-cycling key. Themes come from
`--theme`, `$BEADS_THEME`, or `~/.bd-tui/config.json`.

### Debugging Watcher Issues

If live updates aren't working:
1. Verify `.beads/issues.jsonl` exists and is not empty
2. Check fsnotify errors in TUI startup output (stderr)
3. Use manual refresh with `r` key
4. Run with `--debug` flag and check the log for watcher events

## Key Files to Know

- **cmd/bd-tui/tui.go** - The TUI: widgets, key bindings, refresh plumbing
- **cmd/bd-tui/main.go** - Entry point only: flags, config, theme, logging, signals
- **cmd/bd-tui/bd_helpers.go** - The only place that shells out to `bd`
- **cmd/bd-tui/tui_harness_test.go** - SimulationScreen harness the tests drive
- **internal/parser/types.go** - Issue domain types (matches JSONL schema)
- **internal/state/state.go** - Issue categorization, filtering, tree building
- **internal/storage/jsonl.go** - JSONLReader for reading issues.jsonl
- **internal/watcher/watcher.go** - Debounced filesystem watching

## Integration with bd-lite

The TUI reads the JSONL directly but writes only through the `bd` CLI, from its
dialogs and quick-update keys. The same commands work from a shell:

```bash
bd create "New issue" -p 1
bd update bd-a1b2 --status in_progress
bd close bd-a1b2 --reason "Fixed in v1.0"

# Changes will auto-refresh in TUI (~200ms latency)
```

If `bd` is not on `$PATH` the TUI still starts and displays issues, printing a
warning that updates will not work.

The JSONL file format is append-only with "last write wins" semantics. The TUI always reads the latest state.

## Troubleshooting

### No issues displayed

**Cause:** JSONL file doesn't exist or is empty.

**Fix:**
```bash
bd create "Test issue" -p 2
```

### File not found error

**Cause:** Not in a directory with `.beads` folder.

**Fix:**
```bash
bd init            # or: bd init --prefix myproj
```

### .beads with no issues.jsonl

Not an error. bd-lite treats a store file that has not been written yet as an
empty store, and so does the TUI: it opens an empty board with "No issues found
/ Press 'a' to create an issue" in the list. `storage.NewJSONLReader` accepts a
missing path and `LoadIssues` re-checks it on every load, so the first `bd
create` fills the board in place — the watcher registers the `.beads` directory
rather than the file inode, so the file appearing is an ordinary change event.

`bd init` is not the fix if you want the file sooner: it errors with ".beads
directory already exists" whenever the directory is present, `--prefix` or not,
and never repairs a partial store. `bd create "First issue"` writes it.

### TUI not updating after bd commands

**Cause:** File watcher not detecting changes.

**Fix:**
```bash
# Use manual refresh with 'r' key in the TUI
# Or restart the TUI
```

### Incorrect issue categorization

**Cause:** Dependency logic bug or stale JSONL.

**Debug:**
```bash
# Check what bd thinks
bd ready
bd list --status blocked

# Compare with TUI display
```
