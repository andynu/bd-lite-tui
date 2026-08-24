package ui

import (
	"fmt"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/rivo/tview"
)

// renderTimeGroups renders the time-keyed views: the same rows the
// status-grouped list view draws, under relative time headers instead of
// status headers.
//
// The caller has already bucketed and sorted; this only turns groups into
// rows. Headers stay unselectable the same way the status headers do, by
// being added to the list without an entry in indexToIssue.
func renderTimeGroups(
	issueList *tview.List,
	appState *state.State,
	groups []state.TimeGroup,
	showPrefix bool,
	currentIndex *int,
	indexToIssue map[int]*parser.Issue,
) {
	accentColor := formatting.GetAccentColor()

	for _, group := range groups {
		// Headers are uppercased to read as the same kind of row the status
		// groups draw, and carry no blank spacer row above them for the same
		// reason those don't.
		issueList.AddItem(fmt.Sprintf("[%s::b]⬤ %s (%d)[-::-]",
			accentColor, strings.ToUpper(group.Bucket.Label()), len(group.Issues)), "", 0, nil)
		*currentIndex++

		for _, issue := range group.Issues {
			icon, _ := issueStatus(appState, issue)
			issueList.AddItem(formatIssueListItem(issue, icon, showPrefix, appState.Identity()), "", 0, nil)
			indexToIssue[*currentIndex] = issue
			*currentIndex++
		}
	}
}
