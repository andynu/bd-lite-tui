package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/rivo/tview"
)

// ShowLabelDialog displays a dialog for managing labels
func (h *DialogHelpers) ShowLabelDialog() {
	// Get current issue
	currentIndex := h.IssueList.GetCurrentItem()
	issue, ok := (*h.IndexToIssue)[currentIndex]
	if !ok {
		h.StatusBar.SetText(fmt.Sprintf("[%s]No issue selected[-]", formatting.GetErrorColor()))
		return
	}

	form := tview.NewForm()
	form.AddTextView("Managing labels for", issue.ID+" - "+issue.Title, 0, 2, false, false)

	// Show current labels
	if len(issue.Labels) > 0 {
		labelText := "Current Labels:\n  "
		for i, label := range issue.Labels {
			if i > 0 {
				labelText += ", "
			}
			labelText += label
		}
		form.AddTextView("", labelText, 0, 2, false, false)
	} else {
		form.AddTextView("", "No labels", 0, 1, false, false)
	}

	// Add new label field
	var newLabel string
	form.AddInputField("Add Label", "", 30, nil, func(text string) {
		newLabel = text
	})

	// Add button
	form.AddButton("Add Label", func() {
		trimmedLabel := strings.TrimSpace(newLabel)
		if trimmedLabel == "" {
			h.StatusBar.SetText(fmt.Sprintf("[%s]Error: Label cannot be empty[-]", formatting.GetErrorColor()))
			return
		}

		// Check if label already exists
		for _, existing := range issue.Labels {
			if existing == trimmedLabel {
				h.StatusBar.SetText(fmt.Sprintf("[%s]Error: Label '%s' already exists[-]", formatting.GetErrorColor(), trimmedLabel))
				return
			}
		}

		issueID := issue.ID // Capture before potential refresh
		// bd-lite uses bd update --labels with the full label list
		allLabels := append(issue.Labels, trimmedLabel)
		labelArg := strings.Join(allLabels, ",")
		log.Printf("BD COMMAND: Setting labels: bd update %s --labels %q", issueID, labelArg)
		updatedIssue, err := execBdJSONIssue("update", issueID, "--labels", labelArg)
		if err != nil {
			log.Printf("BD COMMAND ERROR: Label add failed: %v", err)
			h.StatusBar.SetText(fmt.Sprintf("[%s]Error adding label: %v[-]", formatting.GetErrorColor(), err))
		} else {
			log.Printf("BD COMMAND: Label added successfully to %s", updatedIssue.ID)
			h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Added label [%s]'%s'[-][-]", formatting.GetSuccessColor(), formatting.GetEmphasisColor(), trimmedLabel))
			h.Pages.RemovePage("label_dialog")
			h.App.SetFocus(h.IssueList)
			h.ScheduleRefresh(issueID)
		}
	})

	// Remove label buttons
	if len(issue.Labels) > 0 {
		form.AddTextView("", "\nRemove Labels:", 0, 1, false, false)
		for _, label := range issue.Labels {
			// Capture label in closure
			labelToRemove := label
			buttonLabel := fmt.Sprintf("Remove '%s'", labelToRemove)
			form.AddButton(buttonLabel, func() {
				issueID := issue.ID
				// bd-lite uses bd update --labels with the full label list (minus removed)
				var remaining []string
				for _, l := range issue.Labels {
					if l != labelToRemove {
						remaining = append(remaining, l)
					}
				}
				labelArg := strings.Join(remaining, ",")
				log.Printf("BD COMMAND: Setting labels: bd update %s --labels %q", issueID, labelArg)
				updatedIssue, err := execBdJSONIssue("update", issueID, "--labels", labelArg)
				if err != nil {
					log.Printf("BD COMMAND ERROR: Label remove failed: %v", err)
					h.StatusBar.SetText(fmt.Sprintf("[%s]Error removing label: %v[-]", formatting.GetErrorColor(), err))
				} else {
					log.Printf("BD COMMAND: Label removed successfully from %s", updatedIssue.ID)
					h.StatusBar.SetText(fmt.Sprintf("[%s]✓ Removed label [%s]'%s'[-][-]", formatting.GetSuccessColor(), formatting.GetEmphasisColor(), labelToRemove))
					h.Pages.RemovePage("label_dialog")
					h.App.SetFocus(h.IssueList)
					h.ScheduleRefresh(issueID)
				}
			})
		}
	}

	// Close button
	form.AddButton("Close", func() {
		h.Pages.RemovePage("label_dialog")
		h.App.SetFocus(h.IssueList)
	})

	form.SetBorder(true).SetTitle(" Manage Labels ").SetTitleAlign(tview.AlignCenter)
	form.SetCancelFunc(func() {
		h.Pages.RemovePage("label_dialog")
		h.App.SetFocus(h.IssueList)
	})

	// Create modal (centered)
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 0, 3, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	h.Pages.AddPage("label_dialog", modal, true, true)
	h.App.SetFocus(form)
}
