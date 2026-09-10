package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// assignDialogPage is the page name the assign dialog mounts under.
const assignDialogPage = "assign_dialog"

// ShowAssignDialog displays a dialog for setting or clearing an issue's
// assignee. Completions come from assignees already present in the issue set,
// so the same spelling gets reused rather than a near-miss variant.
func (h *DialogHelpers) ShowAssignDialog() {
	currentIndex := h.IssueList.GetCurrentItem()
	issue, ok := (*h.IndexToIssue)[currentIndex]
	if !ok {
		h.StatusBar.SetText(fmt.Sprintf("[%s]No issue selected[-]", formatting.GetErrorColor()))
		return
	}

	issueID := issue.ID
	known := h.AppState.GetAllAssignees()
	me := h.AppState.Identity()

	// assign runs the bd update and reports the outcome. An empty assignee
	// clears the field: bd drops the key entirely when handed "".
	assign := func(assignee string) {
		assignee = strings.TrimSpace(assignee)
		log.Printf("BD COMMAND: Setting assignee: bd update %s --assignee %q", issueID, assignee)
		updatedIssue, err := execBdJSONIssue("update", issueID, "--assignee", assignee)
		if err != nil {
			log.Printf("BD COMMAND ERROR: Assignee update failed: %v", err)
			h.StatusBar.SetText(fmt.Sprintf("[%s]Error setting assignee: %v[-]", formatting.GetErrorColor(), err))
			return
		}

		if assignee == "" {
			log.Printf("BD COMMAND: Cleared assignee on %s", updatedIssue.ID)
			h.Confirm(fmt.Sprintf("[%s]✓ Cleared assignee on %s[-]",
				formatting.GetSuccessColor(), updatedIssue.ID))
		} else {
			log.Printf("BD COMMAND: Assigned %s to %s", updatedIssue.ID, assignee)
			h.Confirm(fmt.Sprintf("[%s]✓ Assigned %s to %s[-]",
				formatting.GetSuccessColor(), updatedIssue.ID, assignee))
		}

		h.Pages.RemovePage(assignDialogPage)
		h.App.SetFocus(h.IssueList)
		h.ScheduleRefresh(issueID)
	}

	form := tview.NewForm()
	form.AddTextView("Assigning", issueID+" - "+issue.Title, 0, 2, false, false)

	currentText := "unassigned"
	if issue.Assignee != "" {
		currentText = issue.Assignee
	}
	form.AddTextView("Currently", currentText, 0, 1, false, false)

	assignee := issue.Assignee
	field := tview.NewInputField().
		SetLabel("Assignee").
		SetText(issue.Assignee).
		SetFieldWidth(40).
		SetChangedFunc(func(text string) { assignee = text })
	field.SetAutocompleteFunc(func(currentText string) []string {
		currentText = strings.ToLower(strings.TrimSpace(currentText))
		if currentText == "" {
			return nil
		}
		var matches []string
		for _, candidate := range known {
			if strings.Contains(strings.ToLower(candidate), currentText) {
				matches = append(matches, candidate)
			}
		}
		return matches
	})
	form.AddFormItem(field)

	form.AddButton("Assign (Enter)", func() { assign(assignee) })

	// "Assign to me" is only meaningful when git user.name resolved.
	if me != "" {
		form.AddButton("Assign to me ("+me+")", func() { assign(me) })
	}

	form.AddButton("Clear", func() { assign("") })
	form.AddButton("Cancel (ESC)", func() {
		h.Pages.RemovePage(assignDialogPage)
		h.App.SetFocus(h.IssueList)
	})

	form.SetBorder(true).SetTitle(" Assign Issue ").SetTitleAlign(tview.AlignCenter)
	form.SetCancelFunc(func() {
		h.Pages.RemovePage(assignDialogPage)
		h.App.SetFocus(h.IssueList)
	})

	// Enter in the input field submits, so assigning is one keystroke rather
	// than a tab to the button. tview dispatches key events from the root down,
	// so this capture runs before the focused child's: a focused button would
	// otherwise never see its own Enter and "Clear" would assign instead.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyEnter {
			return event
		}
		if _, button := form.GetFocusedItemIndex(); button >= 0 {
			return event
		}
		assign(assignee)
		return nil
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 0, 2, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	h.Pages.AddPage(assignDialogPage, modal, true, true)
	h.App.SetFocus(form)
}
