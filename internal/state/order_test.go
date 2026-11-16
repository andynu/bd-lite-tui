package state

import (
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// bd-lite writes issues.jsonl by created_by, then id, which says nothing about
// importance or age. The groups follow bd ready instead: priority, then oldest
// first, with the id breaking ties so a reload never reshuffles rows.
func TestGroupsFollowBdReadyOrderNotFileOrder(t *testing.T) {
	base := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	open := func(id, by string, priority int, age time.Duration) *parser.Issue {
		return &parser.Issue{
			ID: id, Title: id, Status: parser.StatusOpen, Priority: priority,
			IssueType: parser.TypeTask, CreatedBy: by, CreatedAt: base.Add(-age),
		}
	}
	// File order, as bd-lite 0.1.0 writes it: by author, then id.
	issues := []*parser.Issue{
		open("t-ce0", "Andy", 2, 3*time.Hour),
		open("t-d0g", "amy", 0, 1*time.Hour),
		open("t-x7f", "amy", 3, 2*time.Hour),
		open("t-7um", "zed", 1, 4*time.Hour),
		open("t-aaa", "zed", 2, 3*time.Hour), // same priority and age as t-ce0
		open("t-old", "zed", 2, 5*time.Hour),
	}

	s := New()
	s.LoadIssues(issues)

	want := []string{"t-d0g", "t-7um", "t-old", "t-aaa", "t-ce0", "t-x7f"}
	got := ids(s.GetReadyIssues())
	if len(got) != len(want) {
		t.Fatalf("ready = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ready = %v, want %v", got, want)
		}
	}

	// The caller's slice keeps its order.
	if issues[0].ID != "t-ce0" {
		t.Errorf("LoadIssues reordered the caller's slice: first is %s", issues[0].ID)
	}
}

func ids(issues []*parser.Issue) []string {
	out := make([]string, len(issues))
	for i, issue := range issues {
		out[i] = issue.ID
	}
	return out
}
