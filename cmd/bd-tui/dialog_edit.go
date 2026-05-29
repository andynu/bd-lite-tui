package main

import (
	"fmt"
	"log"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowEditForm displays a dialog for editing all issue fields
func (h *DialogHelpers) ShowEditForm() {
	// Get current issue
	currentIndex := h.IssueList.GetCurrentItem()
	issue, ok := (*h.IndexToIssue)[currentIndex]
	if !ok {
		h.StatusBar.SetText(fmt.Sprintf("[%s]No issue selected[-]", formatting.GetErrorColor()))
		return
	}

	form := tview.NewForm()
	var title, description string
	var priority int
	var issueType string

	// Initialize with current values
	title = issue.Title
	description = issue.Description
	priority = issue.Priority
	issueType = string(issue.IssueType)

	form.AddTextView("Editing", issue.ID, 0, 1, false, false)
	form.AddInputField("Title", title, 60, nil, func(text string) {
		title = text
	})
	form.AddTextArea("Description", description, 60, 5, 0, func(text string) {
		description = text
	})
	form.AddDropDown("Priority", []string{"P0 (Critical)", "P1 (High)", "P2 (Normal)", "P3 (Low)", "P4 (Lowest)"}, priority, func(option string, index int) {
		priority = index
	})

	// Find index of current type
	typeOptions := []string{"bug", "feature", "task", "epic", "chore"}
	typeIndex := 1 // default to feature
	for i, t := range typeOptions {
		if t == issueType {
			typeIndex = i
			break
		}
	}
	form.AddDropDown("Type", typeOptions, typeIndex, func(option string, index int) {
		issueType = option
	})

	// Save function
	saveChanges := func() {
		issueID := issue.ID // Capture before potential refresh

		// Use execBdJSON (exec without a shell) so title/description content is
		// passed as discrete argv entries — no shell quoting/injection concerns —
		// and stderr warnings are kept out of the JSON parsed from stdout.
		log.Printf("BD COMMAND: Updating issue: bd update %s ...", issueID)
		updatedIssue, err := execBdJSONIssue("update", issueID,
			"--title", title,
			"--description", description,
			"--priority", fmt.Sprintf("%d", priority),
			"--type", issueType)
		if err != nil {
			log.Printf("BD COMMAND ERROR: Update failed: %v", err)
			h.StatusBar.SetText(fmt.Sprintf("[%s]Error updating issue: %v[-]", formatting.GetErrorColor(), err))
			return
		}

		log.Printf("BD COMMAND: Issue updated successfully: %s", updatedIssue.Title)
		h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Updated [%s]%s[-][-]", formatting.GetSuccessColor(), formatting.GetAccentColor(), updatedIssue.ID))
		h.Pages.RemovePage("edit_form")
		h.App.SetFocus(h.IssueList)
		h.ScheduleRefresh(issueID)
	}

	form.AddButton("Save (Ctrl-S)", saveChanges)
	form.AddButton("Cancel", func() {
		h.Pages.RemovePage("edit_form")
		h.App.SetFocus(h.IssueList)
	})

	form.SetBorder(true).SetTitle(" Edit Issue ").SetTitleAlign(tview.AlignCenter)
	form.SetCancelFunc(func() {
		h.Pages.RemovePage("edit_form")
		h.App.SetFocus(h.IssueList)
	})

	// Add Ctrl-S handler for save
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			saveChanges()
			return nil
		}
		return event
	})

	// Create modal (centered, larger for editing)
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 0, 1, false)

	h.Pages.AddPage("edit_form", modal, true, true)
	h.App.SetFocus(form)
}
