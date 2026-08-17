package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowHelpScreen displays the keyboard shortcuts help screen
func (h *DialogHelpers) ShowHelpScreen() {
	// Note: This help screen uses hardcoded colors for documentation purposes
	// showing the current theme's colors as examples
	helpText := `[yellow::b]bd-tui Keyboard Shortcuts[-::-]

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
  ESC         Return focus to issue list
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
  0-4         Set priority (P0=critical, P1=high, P2=normal, P3=low, P4=lowest)
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

  si also claims the issue for you (git user.name) when it has
  no assignee. An issue someone else holds is never reassigned.

[cyan::b]Assignees[-::-]
  Rows show a dimmed @name when an issue is assigned. Identity is
  always git user.name; an agent may tag itself after a slash
  ("andy/claude-xyz"), which still counts as andy's.

  A           Assign issue (set or clear, autocompletes known names)
  f  mine        Filter to issues assigned to you
  f  unassigned  Filter to issues nobody has claimed
  f  @andy       Filter to andy (matches "andy/claude-xyz" too)

[cyan::b]View Controls[-::-]
  t           Toggle between list and tree view
  v           Cycle layout: auto → columns → rows → paged → auto

  The layout follows the terminal width. Under 90 columns it
  pages: the list and the detail each take the whole screen,
  and Tab/Enter/ESC switch between them. A pinned layout (v)
  is marked with * in the status bar and still gives way to
  paging when the window is too narrow to split.
  o           Collapse/expand node in tree view (vim-style fold)
  O           Expand all nodes in tree view
  Z           Collapse all nodes in tree view
  C           Toggle showing closed issues in list view
  p           Toggle issue ID prefix (tui-abc vs abc)
  f           Quick filter (type: p1 bug, feature, etc.)
  S           Show statistics dashboard
  m           Toggle mouse mode on/off
  r           Manual refresh

[cyan::b]Detail Panel Scrolling (when focused)[-::-]
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

  --view <mode>       Start in list or tree view
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
  ○           Blocked
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
  [gold]○[-]           Blocked
  [deepskyblue]◆[-]           In Progress
  [gray]·[-]           Closed

[gray]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]
[yellow]Press ESC or ? to close this help[-]`

	// Create help text view
	helpTextView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText).
		SetTextAlign(tview.AlignLeft)
	helpTextView.SetBorder(true).
		SetTitle(" Help - Keyboard Shortcuts ").
		SetTitleAlign(tview.AlignCenter)

	// Create modal (centered)
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(helpTextView, 0, 3, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	// Add input capture to close on ESC, q, or ?
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && (event.Rune() == '?' || event.Rune() == 'q')) {
			h.Pages.RemovePage("help")
			h.App.SetFocus(h.IssueList)
			return nil
		}
		return event
	})

	h.Pages.AddPage("help", modal, true, true)
	h.App.SetFocus(modal)
}
