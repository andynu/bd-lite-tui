package main

import (
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
)

func TestStillBlockedNote(t *testing.T) {
	s := state.New()
	s.LoadIssues([]*parser.Issue{
		newIssue("tui-b1", "Blocker one"),
		newIssue("tui-b2", "Blocker two", withStatus(parser.StatusInProgress)),
		newIssue("tui-done", "Finished blocker", withStatus(parser.StatusClosed)),
		newIssue("tui-two", "Two open blockers",
			withBlockedBy("tui-b1"), withBlockedBy("tui-b2"), withBlockedBy("tui-done")),
		newIssue("tui-epic", "Blocked epic", withBlockedBy("tui-b1")),
		newIssue("tui-epic.1", "Dotted child of the blocked epic"),
		newIssue("tui-free", "Only a closed blocker", withBlockedBy("tui-done")),
		newIssue("tui-parked", "Parked by hand", withStatus(parser.StatusBlocked)),
		newIssue("tui-parked.1", "Child of the parked issue"),
	})

	for _, tc := range []struct{ id, want string }{
		{"tui-two", ", still blocked by tui-b1, tui-b2"},
		{"tui-epic.1", ", still blocked through parent tui-epic"},
		{"tui-free", ""},
		{"tui-parked", ""},
		{"tui-parked.1", ""},
		{"tui-missing", ""},
	} {
		if got := stillBlockedNote(s, tc.id); got != tc.want {
			t.Errorf("stillBlockedNote(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}
