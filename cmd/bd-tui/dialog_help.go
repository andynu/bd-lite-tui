package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// helpText is the body of the `?` screen. It is a package-level const so
// dialog_help_test.go can assert on it without standing up a dialog, which is
// what keeps the view-mode names here from drifting away from
// state.ViewModeNames().
//
// The colors are hardcoded rather than themed: this screen documents the
// palette, so it has to show those colors whatever the active theme is.
const helpText = `[yellow::b]bd-tui Keyboard Shortcuts[-::-]

[green::b]Quick Start[-::-]
  j / k         Navigate up/down (or use arrow keys)
  Enter         View issue details
  a             Create new issue
  /             Search issues
  ?             This help screen
  q             Quit

[cyan::b]Navigation[-::-]
  j / ↓       Move down
  k / ↑       Move up
  gg          Jump to top
  G           Jump to bottom
  Space       Page down (wraps to the top at the end)
  Ctrl-f      Page down
  Ctrl-b      Page up
  Tab         Focus detail panel for scrolling
  Enter       Focus detail panel (when on issue)
  ESC / ←     Return focus to issue list
  ESC ESC     Quit (twice within a second, from the list)

[cyan::b]Search[-::-]
  /           Start search mode (matches issue ID, title, labels)
  n           Next search result
  N           Previous search result
  ESC         Exit search mode

  Searching an ID/number jumps to that issue even when it is
  hidden (collapsed in tree view, filtered out, or closed),
  revealing it as needed.

[cyan::b]Quick Actions[-::-]
  0-4         Set priority
              P0 critical, P1 high, P2 normal, P3 low, P4 lowest
  s           Start a status change (see Two-Character Shortcuts)
  R           Rename issue (edit title)
  a           Create new issue (vim-style "add")
  c           Add comment to selected issue
  e           Edit issue (title, description, priority, type)
  x           Close issue with optional reason
  X           Reopen closed issue with optional reason
  D           Manage dependencies (add/remove blocks, parent-child, related)
  L           Manage labels (add/remove labels)
  A           Assign issue (set or clear assignee, autocompletes)
  y           Yank (copy) issue ID to clipboard
  Y           Yank (copy) issue ID with title to clipboard
  B           Copy git branch name to clipboard

[cyan::b]Two-Character Shortcuts[-::-]
  so          Set status to open
  si          Set status to in_progress
  sb          Set status to blocked
  sc          Set status to closed

  The s prefix is lowercase, and expires after 2 seconds.
  (Uppercase S opens the statistics dashboard.)

  si also claims the issue for you when it has no assignee. An
  issue someone else holds is never reassigned.

  A change that moves the row to another group leaves the
  cursor behind, on the next row of the group it left.

[cyan::b]Names on a row[-::-]
  A dimmed @name is who holds the issue. A dimmed +name is who
  filed it, and only appears when that was someone other than
  you. Identity is git user.name, or $USER when git has no name
  to give; an agent may tag itself after a slash
  ("andy/claude-xyz"), which still counts as andy's.

  A           Assign issue (set or clear, autocompletes known names)
  f  mine        Filter to issues assigned to you
  f  unassigned  Filter to issues nobody has claimed
  f  @andy       Filter to andy (matches "andy/claude-xyz" too)

[cyan::b]View Controls[-::-]
  t           Cycle view: list → tree → created → updated → list
  v           Cycle layout: auto → columns → rows → paged → auto
  o           Collapse/expand node in tree view (vim-style fold)
  O           Expand all nodes in tree view
  Z           Collapse all nodes in tree view
  C           Toggle showing closed issues
  p           Toggle issue ID prefix (tui-abc vs abc)
  f           Quick filter (type: p1 bug, feature, etc.)
  S           Show statistics dashboard
  m           Toggle mouse mode on/off
  Click       Select an issue row; in the paged layout this also
              opens its detail, the same as Enter
  Click       In the detail: the ID line copies the ID, a
              dependency line jumps to the issue it names
  r           Manual refresh

  The four views draw the same rows and differ only in grouping.
  The list border names the mode t goes to next.

    list      By status: in progress, then ready, then blocked.
    tree      By dependency, as a foldable hierarchy.
    created   By age, newest bucket first: Today, Yesterday,
              This week, Last week, This month, Older, Undated.
              Empty buckets are skipped.
    updated   The same buckets keyed on when the issue was last
              touched. bd stamps updated_at on every write, so
              whatever you just edited moves to the top of Today.

  C and f shape the three flat views. Tree view ignores both: it
  never shows closed issues and always draws the whole graph.

  The layout follows the terminal width. Under 90 columns it
  pages: the list and the detail each take the whole screen,
  and Tab/Enter/ESC switch between them. A pinned layout (v)
  is marked with * in the status bar and still gives way to
  paging when the window is too narrow to split.

[cyan::b]Detail Panel (when focused)[-::-]
  j / ↓       Show the next issue (headers skipped)
  k / ↑       Show the previous issue
  Ctrl-d      Scroll down half page
  Ctrl-u      Scroll up half page
  Ctrl-f      Scroll down full page (vim)
  Ctrl-b      Scroll up full page (vim)
  Ctrl-e      Scroll down one line
  Ctrl-y      Scroll up one line
  PageDown    Scroll down full page
  PageUp      Scroll up full page
  Home        Jump to top of details
  End         Jump to bottom of details

[cyan::b]General[-::-]
  ?           Show this help screen
  q           Quit

[cyan::b]Command Line Options[-::-]
  --theme <name>      Set color theme
    bd-tui --theme gruvbox-dark

  --view <mode>       Start in list, tree, created, or updated view
    bd-tui --view tree

  --issue <id>        Show only a specific issue
    bd-tui --issue tui-abc

  --debug             Enable debug logging

[cyan::b]Themes[-::-]
  Available themes: default, gruvbox-dark (the default), gruvbox-light,
  nord, solarized-dark, solarized-light, dracula, tokyo-night,
  catppuccin-mocha, catppuccin-latte, colorblind, high-contrast

  There is no theme-switching key. Pick one, highest priority first:
    bd-tui --theme gruvbox-dark
    export BEADS_THEME=gruvbox-dark
    "theme" in ~/.bd-tui/config.json

[cyan::b]Status Icons[-::-]
  ●           Open/Ready
  ○           Blocked (status)
  ◌           Blocked by a dependency
  ◆           In Progress
  ·           Other

[cyan::b]Priority Colors[-::-]
  [red]P0[-]          Critical
  [orangered]P1[-]          High
  [lightskyblue]P2[-]          Normal
  [gray]P3[-]          Low
  [gray]P4[-]          Lowest

[cyan::b]Status Colors[-::-]
  [limegreen]●[-]           Ready
  [gold]○[-]           Blocked (status)
  [gold]◌[-]           Blocked by a dependency
  [deepskyblue]◆[-]           In Progress
  [gray]·[-]           Closed`

// helpScrollPosition names where the reader is in the help body, in vim's
// ruler vocabulary: All, Top, a percentage of the scrollable distance, Bot.
//
// offset is the first visible wrapped line, visible the number of rows on
// screen, total the wrapped length of the whole document. Offsets outside the
// document are clamped here, because tview clamps its own scroll offset while
// drawing the body, which happens after this line has been composed.
func helpScrollPosition(offset, visible, total int) string {
	if visible <= 0 || total <= visible {
		return "All"
	}
	if offset <= 0 {
		return "Top"
	}
	last := total - visible
	if offset >= last {
		return "Bot"
	}
	return fmt.Sprintf("%d%%", offset*100/last)
}

// helpFooter is the help screen's bottom line. It is drawn outside the
// scrolling body so it is on screen wherever the reader is. The hint it
// replaces was the last line of helpText, roughly 190 lines below the fold on
// an 80x24 terminal, so nothing visible said the screen scrolled at all.
func helpFooter(offset, visible, total int) string {
	return fmt.Sprintf(" [gray]%-4s  ↑/↓ PgUp/PgDn scroll[-]  [yellow]ESC or ? closes[-]",
		helpScrollPosition(offset, visible, total))
}

// ShowHelpScreen displays the keyboard shortcuts help screen
func (h *DialogHelpers) ShowHelpScreen() {
	body := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText).
		SetTextAlign(tview.AlignLeft)

	// Wrapping decides how long the document is, so the count is kept against
	// the width it was measured at and reused until the terminal changes size.
	measuredWidth, measuredLines := -1, 0
	wrappedLines := func(width int) int {
		if width != measuredWidth {
			measuredWidth, measuredLines = width, len(tview.WordWrap(helpText, width))
		}
		return measuredLines
	}

	// The footer composes itself at draw time from the body's scroll offset.
	// tview's Flex draws the focused item last, so the body's rect is already
	// assigned by the time this runs, and its offset is the one the body is
	// about to render.
	footer := tview.NewTextView().SetDynamicColors(true)
	footer.SetDrawFunc(func(_ tcell.Screen, x, y, width, height int) (int, int, int, int) {
		_, _, bodyWidth, bodyHeight := body.GetInnerRect()
		offset, _ := body.GetScrollOffset()
		footer.SetText(helpFooter(offset, bodyHeight, wrappedLines(bodyWidth)))
		return x, y, width, height
	})

	// The frame takes the whole page below the status bar rather than the
	// centered half it used to. At half an 80-column terminal these 76-column
	// lines wrapped into two, roughly doubling the distance to scroll.
	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(body, 0, 1, true).
		AddItem(footer, 1, 0, false)
	frame.SetBorder(true).
		SetTitle(" Help - Keyboard Shortcuts ").
		SetTitleAlign(tview.AlignCenter)

	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		isRune := event.Key() == tcell.KeyRune
		switch {
		case event.Key() == tcell.KeyEscape, isRune && (event.Rune() == '?' || event.Rune() == 'q'):
			h.Pages.RemovePage("help")
			h.App.SetFocus(h.IssueList)
			return nil
		case event.Key() == tcell.KeyEnd, isRune && event.Rune() == 'G':
			// tview's own End scrolls by tracking the end of the text, which it
			// resolves while drawing the body — after the footer has composed
			// its position field. Scrolling to a computed offset instead keeps
			// the two in step.
			_, _, bodyWidth, bodyHeight := body.GetInnerRect()
			bottom := wrappedLines(bodyWidth) - bodyHeight
			if bottom < 0 {
				bottom = 0
			}
			body.ScrollTo(bottom, 0)
			return nil
		}
		return event
	})

	// The empty first row is what leaves the status bar, which the main page
	// draws underneath, showing while the help is up.
	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(frame, 0, 1, true)

	h.Pages.AddPage("help", modal, true, true)
	h.App.SetFocus(modal)
}
