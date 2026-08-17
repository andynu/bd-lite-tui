package state

import (
	"reflect"
	"sort"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// assigneeFixture is one open issue per assignee shape the filter has to
// distinguish: unassigned, the human, that human's agent, and someone else.
func assigneeFixture() []*parser.Issue {
	return []*parser.Issue{
		{ID: "a-none", Title: "Unclaimed", Status: parser.StatusOpen, IssueType: parser.TypeTask},
		{ID: "a-me", Title: "Mine", Status: parser.StatusOpen, IssueType: parser.TypeTask, Assignee: "andy"},
		{ID: "a-agent", Title: "My agent's", Status: parser.StatusOpen, IssueType: parser.TypeTask, Assignee: "andy/claude-xyz"},
		{ID: "a-other", Title: "Scott's", Status: parser.StatusOpen, IssueType: parser.TypeTask, Assignee: "scott"},
		{ID: "a-other-agent", Title: "Scott's agent's", Status: parser.StatusOpen, IssueType: parser.TypeTask, Assignee: "scott/claude-abc"},
	}
}

// readyIDs is the ids of the ready issues after filtering, sorted so the
// assertion does not depend on input order.
func readyIDs(s *State) []string {
	var ids []string
	for _, issue := range s.GetReadyIssues() {
		ids = append(ids, issue.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestAssigneeFilter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		me     string
		values []string
		want   []string
	}{
		{
			name:   "mine covers the human and their agent",
			me:     "andy",
			values: []string{AssigneeMine},
			want:   []string{"a-agent", "a-me"},
		},
		{
			name:   "mine ignores case in the identity",
			me:     "ANDY",
			values: []string{AssigneeMine},
			want:   []string{"a-agent", "a-me"},
		},
		{
			name:   "unassigned finds only the unclaimed issue",
			me:     "andy",
			values: []string{AssigneeUnassigned},
			want:   []string{"a-none"},
		},
		{
			name:   "a named assignee also matches their agent",
			me:     "andy",
			values: []string{"scott"},
			want:   []string{"a-other", "a-other-agent"},
		},
		{
			name:   "a named assignee is matched case-insensitively",
			me:     "andy",
			values: []string{"SCOTT"},
			want:   []string{"a-other", "a-other-agent"},
		},
		{
			name:   "a fully qualified agent value matches only itself",
			me:     "andy",
			values: []string{"scott/claude-abc"},
			want:   []string{"a-other-agent"},
		},
		{
			name:   "several values are OR-ed",
			me:     "andy",
			values: []string{AssigneeUnassigned, "scott"},
			want:   []string{"a-none", "a-other", "a-other-agent"},
		},
		{
			name:   "an unknown assignee matches nothing",
			me:     "andy",
			values: []string{"nobody"},
			want:   nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.SetIdentity(tc.me)
			s.LoadIssues(assigneeFixture())
			for _, value := range tc.values {
				s.ToggleAssigneeFilter(value)
			}

			if got := readyIDs(s); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filtered ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// Toggling the same value twice removes the filter entirely rather than leaving
// an empty one that matches nothing.
func TestAssigneeFilterToggleOff(t *testing.T) {
	s := New()
	s.SetIdentity("andy")
	s.LoadIssues(assigneeFixture())

	s.ToggleAssigneeFilter(AssigneeMine)
	if !s.HasActiveFilters() {
		t.Fatal("filter did not become active")
	}
	if !s.IsAssigneeFiltered(AssigneeMine) {
		t.Error("IsAssigneeFiltered did not report the active sentinel")
	}

	s.ToggleAssigneeFilter(AssigneeMine)
	if s.HasActiveFilters() {
		t.Error("toggling the same value off left a filter active")
	}
	if got := len(readyIDs(s)); got != 5 {
		t.Errorf("after clearing, %d issues are visible, want 5", got)
	}
}

// Without a git user.name there is nothing to compare against, so "mine" is
// refused rather than silently matching everything or nothing.
func TestAssigneeFilterMineNeedsAnIdentity(t *testing.T) {
	s := New()
	s.LoadIssues(assigneeFixture())

	s.ToggleAssigneeFilter(AssigneeMine)

	if s.HasActiveFilters() {
		t.Error("'mine' was accepted with no identity resolved")
	}
	if got := len(readyIDs(s)); got != 5 {
		t.Errorf("%d issues are visible, want all 5", got)
	}
}

// A named assignee filter still works with no identity; only "mine" needs one.
func TestAssigneeFilterNamedWorksWithoutAnIdentity(t *testing.T) {
	s := New()
	s.LoadIssues(assigneeFixture())

	s.ToggleAssigneeFilter("scott")

	if got, want := readyIDs(s), []string{"a-other", "a-other-agent"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filtered ids = %v, want %v", got, want)
	}
}

// The assignee filter narrows alongside the others rather than replacing them.
func TestAssigneeFilterCombinesWithOtherFilters(t *testing.T) {
	issues := assigneeFixture()
	issues[1].Priority = 0 // a-me
	issues[2].Priority = 3 // a-agent

	s := New()
	s.SetIdentity("andy")
	s.LoadIssues(issues)

	s.ToggleAssigneeFilter(AssigneeMine)
	s.TogglePriorityFilter(0)

	if got, want := readyIDs(s), []string{"a-me"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filtered ids = %v, want %v", got, want)
	}
}

func TestClearAllFiltersDropsTheAssigneeFilter(t *testing.T) {
	s := New()
	s.SetIdentity("andy")
	s.LoadIssues(assigneeFixture())

	s.ToggleAssigneeFilter(AssigneeMine)
	s.ClearAllFilters()

	if s.HasActiveFilters() {
		t.Error("ClearAllFilters left the assignee filter active")
	}
	if got := len(readyIDs(s)); got != 5 {
		t.Errorf("%d issues are visible after clearing, want 5", got)
	}
}

// The status bar description renders the sentinels as words, and sorts so the
// text does not churn between repaints.
func TestGetActiveFiltersDescribesAssignees(t *testing.T) {
	s := New()
	s.SetIdentity("andy")
	s.LoadIssues(assigneeFixture())

	s.ToggleAssigneeFilter(AssigneeMine)
	s.ToggleAssigneeFilter(AssigneeUnassigned)
	s.ToggleAssigneeFilter("scott")

	if got, want := s.GetActiveFilters(), "Assignee: mine,scott,unassigned"; got != want {
		t.Errorf("GetActiveFilters() = %q, want %q", got, want)
	}
}

func TestGetAllAssignees(t *testing.T) {
	s := New()
	s.LoadIssues(assigneeFixture())

	want := []string{"andy", "andy/claude-xyz", "scott", "scott/claude-abc"}
	if got := s.GetAllAssignees(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetAllAssignees() = %v, want %v", got, want)
	}
}

func TestGetAllAssigneesWithNoneAssigned(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		{ID: "a-none", Title: "Unclaimed", Status: parser.StatusOpen, IssueType: parser.TypeTask},
	})

	if got := s.GetAllAssignees(); len(got) != 0 {
		t.Errorf("GetAllAssignees() = %v, want empty", got)
	}
}

func TestIsMine(t *testing.T) {
	s := New()
	s.SetIdentity("andy")

	for _, tc := range []struct {
		assignee string
		want     bool
	}{
		{"andy", true},
		{"andy/claude-xyz", true},
		{"scott", false},
		{"", false},
	} {
		issue := &parser.Issue{ID: "x", Assignee: tc.assignee}
		if got := s.IsMine(issue); got != tc.want {
			t.Errorf("IsMine(assignee=%q) = %v, want %v", tc.assignee, got, tc.want)
		}
	}

	if s.IsMine(nil) {
		t.Error("IsMine(nil) = true, want false")
	}
}
