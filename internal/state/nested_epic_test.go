package state

import (
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

func epic(id string, priority int, deps ...*parser.Dependency) *parser.Issue {
	return &parser.Issue{ID: id, Title: id, Status: parser.StatusOpen, Priority: priority, IssueType: parser.TypeEpic, Dependencies: deps}
}

func dep(from, to string, typ parser.DependencyType) *parser.Dependency {
	return &parser.Dependency{IssueID: from, DependsOnID: to, Type: typ}
}

func rootIDs(s *State) []string {
	var out []string
	for _, n := range s.GetTreeNodes() {
		out = append(out, n.Issue.ID)
	}
	return out
}

func childIDs(n *TreeNode) []string {
	var out []string
	for _, c := range n.Children {
		out = append(out, c.Issue.ID)
	}
	return out
}

// An epic inside another epic nests under it whichever of the two the tree
// meets first. The inner epic sorts first here (P0 before P2), which used to
// make it a root and drop it from the outer epic.
func TestNestedEpicNestsWhicheverComesFirst(t *testing.T) {
	for _, tc := range []struct {
		name string
		link func() (outer, inner *parser.Issue)
	}{
		{"parent-child", func() (*parser.Issue, *parser.Issue) {
			return epic("t-out", 2), epic("t-in", 0, dep("t-in", "t-out", parser.DepParentChild))
		}},
		{"outer epic blocks dep", func() (*parser.Issue, *parser.Issue) {
			return epic("t-out", 2, dep("t-out", "t-in", parser.DepBlocks)), epic("t-in", 0)
		}},
		{"dotted id", func() (*parser.Issue, *parser.Issue) {
			return epic("t-out", 2), epic("t-out.1", 0)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outer, inner := tc.link()
			s := New()
			s.LoadIssues([]*parser.Issue{inner, outer})
			s.SetViewMode(ViewTree)

			roots := s.GetTreeNodes()
			if len(roots) != 1 || roots[0].Issue.ID != outer.ID {
				t.Fatalf("roots = %v, want only %s", rootIDs(s), outer.ID)
			}
			if got := childIDs(roots[0]); len(got) != 1 || got[0] != inner.ID {
				t.Errorf("%s children = %v, want [%s]", outer.ID, got, inner.ID)
			}
		})
	}
}

// Two epics that list each other both still appear, once.
func TestMutuallyContainingEpicsBothAppear(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		epic("t-a", 1, dep("t-a", "t-b", parser.DepBlocks)),
		epic("t-b", 2, dep("t-b", "t-a", parser.DepBlocks)),
	})
	s.SetViewMode(ViewTree)

	roots := s.GetTreeNodes()
	if len(roots) != 1 {
		t.Fatalf("roots = %v, want one root holding the other epic", rootIDs(s))
	}
	if got := childIDs(roots[0]); len(got) != 1 {
		t.Errorf("%s children = %v, want the other epic", roots[0].Issue.ID, got)
	}
}
