package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowCloseIssueDialog displays a dialog for closing an issue
func (h *DialogHelpers) ShowCloseIssueDialog() {
	// Get current issue
	currentIndex := h.IssueList.GetCurrentItem()
	issue, ok := (*h.IndexToIssue)[currentIndex]
	if !ok {
		h.StatusBar.SetText(fmt.Sprintf("[%s]No issue selected[-]", formatting.GetErrorColor()))
		return
	}

	// Don't allow closing already closed issues
	if issue.Status == parser.StatusClosed {
		h.StatusBar.SetText(fmt.Sprintf("[%s]Issue is already closed[-]", formatting.GetWarningColor()))
		return
	}

	form := tview.NewForm()
	var reason string

	form.AddTextView("Closing", issue.ID+" - "+issue.Title, 0, 2, false, false)
	form.AddInputField("Reason (optional)", "", 60, nil, func(text string) {
		reason = text
	})

	form.AddButton("Close Issue", func() {
		issueID := issue.ID // Capture before potential refresh
		args := []string{"close", issueID}
		if reason != "" {
			args = append(args, "--reason", reason)
		}
		log.Printf("BD COMMAND: Closing issue: bd %s", strings.Join(args, " "))
		closedIssue, err := execBdJSONIssue(args...)
		if err != nil {
			log.Printf("BD COMMAND ERROR: Close failed: %v", err)
			h.StatusBar.SetText(fmt.Sprintf("[%s]Error closing issue: %v[-]", formatting.GetErrorColor(), err))
		} else {
			log.Printf("BD COMMAND: Issue closed successfully: %s", closedIssue.ID)
			h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Closed [%s]%s[-][-]", formatting.GetSuccessColor(), formatting.GetAccentColor(), closedIssue.ID))
			h.Pages.RemovePage("close_issue_dialog")
			h.App.SetFocus(h.IssueList)
			h.ScheduleRefresh(issueID)
		}
	})
	form.AddButton("Cancel", func() {
		h.Pages.RemovePage("close_issue_dialog")
		h.App.SetFocus(h.IssueList)
	})

	form.SetBorder(true).SetTitle(" Close Issue (Enter to submit) ").SetTitleAlign(tview.AlignCenter)
	form.SetCancelFunc(func() {
		h.Pages.RemovePage("close_issue_dialog")
		h.App.SetFocus(h.IssueList)
	})

	// Add Enter key handler to close
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			issueID := issue.ID
			args := []string{"close", issueID}
			if reason != "" {
				args = append(args, "--reason", reason)
			}
			log.Printf("BD COMMAND: Closing issue (Enter): bd %s", strings.Join(args, " "))
			closedIssue, err := execBdJSONIssue(args...)
			if err != nil {
				log.Printf("BD COMMAND ERROR: Close failed: %v", err)
				h.StatusBar.SetText(fmt.Sprintf("[%s]Error closing issue: %v[-]", formatting.GetErrorColor(), err))
			} else {
				log.Printf("BD COMMAND: Issue closed successfully: %s", closedIssue.ID)
				h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Closed [%s]%s[-][-]", formatting.GetSuccessColor(), formatting.GetAccentColor(), closedIssue.ID))
				h.Pages.RemovePage("close_issue_dialog")
				h.App.SetFocus(h.IssueList)
				h.ScheduleRefresh(issueID)
			}
			return nil
		}
		return event
	})

	// Create modal (centered)
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 0, 2, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	h.Pages.AddPage("close_issue_dialog", modal, true, true)
	h.App.SetFocus(form)
}

// ShowReopenIssueDialog displays a dialog for reopening a closed issue
func (h *DialogHelpers) ShowReopenIssueDialog() {
	// Get current issue
	currentIndex := h.IssueList.GetCurrentItem()
	issue, ok := (*h.IndexToIssue)[currentIndex]
	if !ok {
		h.StatusBar.SetText(fmt.Sprintf("[%s]No issue selected[-]", formatting.GetErrorColor()))
		return
	}

	// Only allow reopening closed issues
	if issue.Status != parser.StatusClosed {
		h.StatusBar.SetText(fmt.Sprintf("[%s]Issue is not closed[-]", formatting.GetWarningColor()))
		return
	}

	issueID := issue.ID
	log.Printf("BD COMMAND: Reopening issue: bd update %s --status open", issueID)
	updatedIssue, err := execBdJSONIssue("update", issueID, "--status", "open")
	if err != nil {
		log.Printf("BD COMMAND ERROR: Reopen failed: %v", err)
		h.StatusBar.SetText(fmt.Sprintf("[%s]Error reopening issue: %v[-]", formatting.GetErrorColor(), err))
	} else {
		log.Printf("BD COMMAND: Issue reopened successfully: %s", updatedIssue.ID)
		h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Reopened [%s]%s[-][-]", formatting.GetSuccessColor(), formatting.GetAccentColor(), updatedIssue.ID))
		h.ScheduleRefresh(issueID)
	}
}
