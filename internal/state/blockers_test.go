package state

import (
	"reflect"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

func blocksDep(from, to string) *parser.Dependency {
	return &parser.Dependency{IssueID: from, DependsOnID: to, Type: parser.DepBlocks}
}

func blockerIDs(issues []*parser.Issue) []string {
	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	return ids
}

// Only blocks dependencies on unclosed issues in the store hold an issue up,
// which is the same rule categorizeIssues files it under BLOCKED by.
func TestOpenBlockersListsUnclosedBlocksTargets(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		{ID: "a", Status: parser.StatusOpen},
		{ID: "b", Status: parser.StatusClosed},
		{ID: "c", Status: parser.StatusInProgress},
		{ID: "d", Status: parser.StatusOpen},
		{ID: "x", Status: parser.StatusBlocked, Dependencies: []*parser.Dependency{
			blocksDep("x", "a"),
			blocksDep("x", "b"),
			blocksDep("x", "c"),
			blocksDep("x", "gone"),
			{IssueID: "x", DependsOnID: "d", Type: parser.DepRelated},
		}},
	})

	if got, want := blockerIDs(s.OpenBlockers("x")), []string{"a", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("OpenBlockers(x) = %v, want %v", got, want)
	}
	if got := s.OpenBlockers("a"); len(got) != 0 {
		t.Errorf("an issue with no dependencies has blockers %v", blockerIDs(got))
	}
	if got := s.OpenBlockers("nope"); len(got) != 0 {
		t.Errorf("an unknown issue has blockers %v", blockerIDs(got))
	}
}

// BlockedParent names the parent a child inherits dependency blocking from,
// whether the edge is explicit or inferred from a dotted ID. A parent that is
// only parked by hand passes nothing down.
func TestBlockedParentNamesTheParentBlockingIsInheritedFrom(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		{ID: "blk", Status: parser.StatusOpen},
		{ID: "epic", Status: parser.StatusOpen, Dependencies: []*parser.Dependency{blocksDep("epic", "blk")}},
		{ID: "kid", Status: parser.StatusOpen, Dependencies: []*parser.Dependency{
			{IssueID: "kid", DependsOnID: "epic", Type: parser.DepParentChild},
		}},
		{ID: "epic.1", Status: parser.StatusOpen},
		{ID: "parked", Status: parser.StatusBlocked},
		{ID: "parked.1", Status: parser.StatusOpen},
	})

	for _, tc := range []struct{ id, want string }{
		{"kid", "epic"},
		{"epic.1", "epic"},
		{"epic", ""},
		{"parked.1", ""},
		{"blk", ""},
		{"nope", ""},
	} {
		if got := s.BlockedParent(tc.id); got != tc.want {
			t.Errorf("BlockedParent(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}
