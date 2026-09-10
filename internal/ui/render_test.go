package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/rivo/tview"
)

// An issue parked by hand and an open issue waiting on a dependency are both
// blocked, but only the first changes when its status is cleared, so the row
// has to tell them apart.
func TestIssueStatusTellsExplicitFromDependencyBlocked(t *testing.T) {
	blocks := func(from, to string) []*parser.Dependency {
		return []*parser.Dependency{{IssueID: from, DependsOnID: to, Type: parser.DepBlocks}}
	}
	s := state.New()
	s.LoadIssues([]*parser.Issue{
		{ID: "blk", Status: parser.StatusOpen},
		{ID: "parked", Status: parser.StatusBlocked},
		{ID: "waiting", Status: parser.StatusOpen, Dependencies: blocks("waiting", "blk")},
		{ID: "both", Status: parser.StatusBlocked, Dependencies: blocks("both", "blk")},
		{ID: "working", Status: parser.StatusInProgress, Dependencies: blocks("working", "blk")},
	})

	for _, tc := range []struct{ id, want string }{
		{"blk", "●"},
		{"parked", "○"},
		{"waiting", "◌"},
		{"both", "○"},
		{"working", "◆"},
	} {
		if got, _ := issueStatus(s, s.GetIssueByID(tc.id)); got != tc.want {
			t.Errorf("issueStatus(%s) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// A tview List draws each item on exactly one row, so a "\n" in an item's text
// reaches the screen as nothing at all: it neither wraps nor opens a blank
// separator line. Header rows used to carry one anyway (tui-h9l3). Any item
// text with a newline in it is therefore describing a layout the user will
// never see, which is what this guards against.
func TestListItemsAreSingleLine(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	issues := []*parser.Issue{
		{
			ID: "t-1", Title: "In progress one", Status: parser.StatusInProgress,
			Priority: 1, IssueType: parser.TypeFeature, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "t-2", Title: "Ready one", Status: parser.StatusOpen,
			Priority: 2, IssueType: parser.TypeTask, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "t-3", Title: "Blocked one", Status: parser.StatusOpen,
			Priority: 2, IssueType: parser.TypeBug, CreatedAt: now, UpdatedAt: now,
			Dependencies: []*parser.Dependency{
				{IssueID: "t-3", DependsOnID: "t-2", Type: parser.DepBlocks},
			},
		},
		{
			ID: "t-4", Title: "Closed one", Status: parser.StatusClosed,
			Priority: 3, IssueType: parser.TypeChore, CreatedAt: now, UpdatedAt: now,
		},
	}

	for _, tc := range []struct {
		name   string
		mode   state.ViewMode
		issues []*parser.Issue
	}{
		{"list", state.ViewList, issues},
		{"tree", state.ViewTree, issues},
		{"created", state.ViewCreated, issues},
		{"updated", state.ViewUpdated, issues},
		{"empty", state.ViewList, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			s.SetViewMode(tc.mode)
			s.LoadIssues(tc.issues)

			list := tview.NewList().ShowSecondaryText(false)
			PopulateIssueList(list, s, true, true, now, map[int]*parser.Issue{})

			if list.GetItemCount() == 0 {
				t.Fatal("nothing rendered")
			}
			for i := 0; i < list.GetItemCount(); i++ {
				main, secondary := list.GetItemText(i)
				if strings.ContainsAny(main, "\n\r") {
					t.Errorf("item %d main text contains a line break: %q", i, main)
				}
				if strings.ContainsAny(secondary, "\n\r") {
					t.Errorf("item %d secondary text contains a line break: %q", i, secondary)
				}
			}
		})
	}
}
