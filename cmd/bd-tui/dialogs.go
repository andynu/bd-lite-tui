package main

import (
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/rivo/tview"
)

// DialogHelpers holds references to UI components needed by dialog functions
//
// This struct is shared across all dialog implementations in separate files:
// - dialog_comment.go: ShowCommentDialog
// - dialog_rename.go: ShowRenameDialog
// - dialog_filter.go: ShowQuickFilter
// - dialog_stats.go: ShowStatsOverlay
// - dialog_help.go: ShowHelpScreen
// - dialog_dependencies.go: ShowDependencyDialog
// - dialog_labels.go: ShowLabelDialog
// - dialog_close.go: ShowCloseIssueDialog, ShowReopenIssueDialog
// - dialog_edit.go: ShowEditForm
// - dialog_create.go: ShowCreateIssueDialog
type DialogHelpers struct {
	App             *tview.Application
	Pages           *tview.Pages
	IssueList       *tview.List
	IndexToIssue    *map[int]*parser.Issue
	StatusBar       *tview.TextView
	AppState        *state.State
	RefreshIssues   func(...string)
	ScheduleRefresh func(string)
	// OnFiltersChanged is called after a dialog mutates the filter state, so
	// the caller can repaint the list and status bar. Dialogs mount and return
	// immediately, so the caller cannot do this at the call site.
	OnFiltersChanged func()
}

// filtersChanged notifies the caller that filter state was mutated.
func (h *DialogHelpers) filtersChanged() {
	if h.OnFiltersChanged != nil {
		h.OnFiltersChanged()
	}
}
