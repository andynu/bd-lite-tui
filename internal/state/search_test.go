package state

import (
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

func searchTestIssues() []*parser.Issue {
	return []*parser.Issue{
		{ID: "wiser-15q", Title: "ExpiringPlansQueryTest needs rewrite", Status: parser.StatusOpen, Priority: 2, IssueType: parser.TypeBug},
		{ID: "wiser-pk2", Title: "Fix execute method", Status: parser.StatusOpen, Priority: 1, IssueType: parser.TypeTask},
		{ID: "wiser-abc", Title: "Closed work", Status: parser.StatusClosed, Priority: 3, IssueType: parser.TypeFeature},
	}
}

func TestFindIssueByQuery(t *testing.T) {
	s := New()
	s.LoadIssues(searchTestIssues())

	cases := []struct {
		name  string
		query string
		want  string // expected issue ID, "" means nil
	}{
		{"bare suffix", "15q", "wiser-15q"},
		{"full id", "wiser-15q", "wiser-15q"},
		{"uppercase suffix", "15Q", "wiser-15q"},
		{"prefix only matches first", "wiser", "wiser-15q"},
		{"title substring", "rewrite", "wiser-15q"},
		{"finds closed issue", "abc", "wiser-abc"},
		{"no match", "zzz999", ""},
		{"empty", "", ""},
		{"whitespace", "  15q  ", "wiser-15q"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.FindIssueByQuery(tc.query)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("expected nil, got %s", got.ID)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %s, got nil", tc.want)
			}
			if got.ID != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got.ID)
			}
		})
	}
}

func TestFindIssueByQueryPrefersIDOverTitle(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		{ID: "proj-title", Title: "unrelated", Status: parser.StatusOpen},
		{ID: "proj-xyz", Title: "this mentions title in the body", Status: parser.StatusOpen},
	})

	// "title" appears in proj-title's ID and in proj-xyz's title.
	// ID matches must win.
	got := s.FindIssueByQuery("title")
	if got == nil || got.ID != "proj-title" {
		t.Fatalf("expected proj-title (ID match wins), got %v", got)
	}
}

func TestIssuePassesFilters(t *testing.T) {
	s := New()
	s.LoadIssues(searchTestIssues())

	p1 := s.GetIssueByID("wiser-pk2") // priority 1
	p2 := s.GetIssueByID("wiser-15q") // priority 2

	if !s.IssuePassesFilters(p2) {
		t.Fatal("with no filters, every issue should pass")
	}

	s.TogglePriorityFilter(1) // only show P1
	if s.IssuePassesFilters(p2) {
		t.Fatal("P2 issue should not pass a P1-only filter")
	}
	if !s.IssuePassesFilters(p1) {
		t.Fatal("P1 issue should pass a P1-only filter")
	}
}

func TestExpandAncestors(t *testing.T) {
	// Parent with a child via ID-prefix relationship; collapse the parent, then
	// ensure ExpandAncestors reveals the child.
	s := New()
	s.LoadIssues([]*parser.Issue{
		{ID: "proj-1", Title: "parent", Status: parser.StatusOpen, IssueType: parser.TypeEpic},
		{ID: "proj-1.1", Title: "child", Status: parser.StatusOpen, IssueType: parser.TypeTask},
	})
	s.SetViewMode(ViewTree)

	if !s.HasChildren("proj-1") {
		t.Fatal("expected proj-1 to have a child in the tree")
	}

	s.SetCollapsed("proj-1", true)
	if !s.IsCollapsed("proj-1") {
		t.Fatal("proj-1 should be collapsed")
	}

	changed := s.ExpandAncestors("proj-1.1")
	if !changed {
		t.Fatal("expected ExpandAncestors to expand the collapsed parent")
	}
	if s.IsCollapsed("proj-1") {
		t.Fatal("proj-1 should be expanded after ExpandAncestors")
	}

	// Calling again when already expanded reports no change.
	if s.ExpandAncestors("proj-1.1") {
		t.Fatal("expected no change when ancestors already expanded")
	}

	// Unknown issue returns no change.
	if s.ExpandAncestors("does-not-exist") {
		t.Fatal("expected no change for unknown issue")
	}
}
