package main

import (
	"fmt"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowQuickFilter displays a dialog for quick filtering of issues
func (h *DialogHelpers) ShowQuickFilter() {
	form := tview.NewForm()
	var filterQuery string

	emphasisColor := formatting.GetEmphasisColor()
	accentColor := formatting.GetAccentColor()
	mutedColor := formatting.GetMutedColor()

	helpText := fmt.Sprintf(`[%s]Quick Filter Syntax:[-]
  p0-p4    Priority (e.g., 'p1' or 'p1,p2')
  bug, feature, task, epic, chore    Types
  open, in_progress, blocked, closed    Statuses
  #label   Label (e.g., '#ui' or '#bug,#urgent')
  @name    Assignee ('@andy' also matches 'andy/claude-xyz')
  mine     Assigned to you (git user.name)
  unassigned    Nobody is on it

[%s]Examples:[-]
  p1 bug          P1 bugs only
  feature,task    Features and tasks
  p0,p1 open      High priority open issues
  #ui #urgent     Issues with 'ui' or 'urgent' labels
  mine in_progress    What you are working on
  unassigned p0,p1    High priority nobody has claimed

[%s]Leave empty to clear all filters[-]`, emphasisColor, accentColor, mutedColor)

	form.AddTextView("", helpText, 0, 17, false, false)
	form.AddInputField("Filter", "", 50, nil, func(text string) {
		filterQuery = text
	})

	// Apply filter function
	applyQuickFilter := func() {
		// Clear existing filters
		h.AppState.ClearAllFilters()

		if filterQuery == "" {
			// Empty query = clear all filters
			h.Pages.RemovePage("quick_filter")
			h.App.SetFocus(h.IssueList)
			h.filtersChanged()
			return
		}

		// Parse filter query (space or comma separated)
		query := strings.ToLower(strings.TrimSpace(filterQuery))
		tokens := strings.FieldsFunc(query, func(r rune) bool {
			return r == ' ' || r == ','
		})

		// Process each token
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			// Check for label (starts with #)
			if strings.HasPrefix(token, "#") {
				label := strings.TrimPrefix(token, "#")
				if label != "" {
					h.AppState.ToggleLabelFilter(label)
				}
				continue
			}

			// Check for assignee (starts with @). "@me" and "@none" are aliases
			// for the bare 'mine' and 'unassigned' words below.
			if strings.HasPrefix(token, "@") {
				switch name := strings.TrimPrefix(token, "@"); name {
				case "":
					// bare "@" means nothing; ignore it
				case "me":
					h.AppState.ToggleAssigneeFilter(state.AssigneeMine)
				case "none":
					h.AppState.ToggleAssigneeFilter(state.AssigneeUnassigned)
				default:
					h.AppState.ToggleAssigneeFilter(name)
				}
				continue
			}

			// Check for the assignee keywords
			switch token {
			case "mine":
				h.AppState.ToggleAssigneeFilter(state.AssigneeMine)
				continue
			case "unassigned":
				h.AppState.ToggleAssigneeFilter(state.AssigneeUnassigned)
				continue
			}

			// Check for priority (p0-p4)
			if len(token) == 2 && token[0] == 'p' && token[1] >= '0' && token[1] <= '4' {
				priority := int(token[1] - '0')
				h.AppState.TogglePriorityFilter(priority)
				continue
			}

			// Check for type
			switch token {
			case "bug":
				h.AppState.ToggleTypeFilter(parser.TypeBug)
			case "feature":
				h.AppState.ToggleTypeFilter(parser.TypeFeature)
			case "task":
				h.AppState.ToggleTypeFilter(parser.TypeTask)
			case "epic":
				h.AppState.ToggleTypeFilter(parser.TypeEpic)
			case "chore":
				h.AppState.ToggleTypeFilter(parser.TypeChore)
			}

			// Check for status
			switch token {
			case "open":
				h.AppState.ToggleStatusFilter(parser.StatusOpen)
			case "in_progress", "inprogress":
				h.AppState.ToggleStatusFilter(parser.StatusInProgress)
			case "blocked":
				h.AppState.ToggleStatusFilter(parser.StatusBlocked)
			case "closed":
				h.AppState.ToggleStatusFilter(parser.StatusClosed)
			}
		}

		// Apply filters
		h.Pages.RemovePage("quick_filter")
		h.App.SetFocus(h.IssueList)
		h.filtersChanged()
	}

	form.AddButton("Apply (Enter)", applyQuickFilter)
	form.AddButton("Clear All", func() {
		h.AppState.ClearAllFilters()
		h.Pages.RemovePage("quick_filter")
		h.App.SetFocus(h.IssueList)
		h.filtersChanged()
	})
	form.AddButton("Cancel (ESC)", func() {
		h.Pages.RemovePage("quick_filter")
		h.App.SetFocus(h.IssueList)
	})

	form.SetBorder(true).SetTitle(" Quick Filter ").SetTitleAlign(tview.AlignCenter)
	form.SetCancelFunc(func() {
		h.Pages.RemovePage("quick_filter")
		h.App.SetFocus(h.IssueList)
	})

	// Enter in the filter field applies, so filtering is one keystroke rather
	// than a tab to the button. tview dispatches key events from the root down,
	// so this capture runs before the focused child's: a focused button would
	// otherwise never see its own Enter and "Clear All" would apply instead.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyEnter {
			return event
		}
		if _, button := form.GetFocusedItemIndex(); button >= 0 {
			return event
		}
		applyQuickFilter()
		return nil
	})

	// Create modal (centered)
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 0, 2, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	h.Pages.AddPage("quick_filter", modal, true, true)
	h.App.SetFocus(form)
}
