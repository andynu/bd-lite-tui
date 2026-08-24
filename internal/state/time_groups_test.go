package state

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/timegroup"
)

// groupNow is a Wednesday, so the week opened on Monday the 16th and there is
// room between "Yesterday" and the start of the week.
var groupNow = time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)

func at(y int, m time.Month, d, hour int) time.Time {
	return time.Date(y, m, d, hour, 0, 0, 0, time.UTC)
}

// groupIssue is a minimal open issue created at created.
func groupIssue(id string, created time.Time, mods ...func(*parser.Issue)) *parser.Issue {
	issue := &parser.Issue{
		ID:        id,
		Title:     id,
		Status:    parser.StatusOpen,
		Priority:  2,
		IssueType: parser.TypeTask,
		CreatedAt: created,
		UpdatedAt: created,
	}
	for _, mod := range mods {
		mod(issue)
	}
	return issue
}

// summarize renders the groups as "BUCKET:id,id|BUCKET:id" so an expectation
// reads as one string instead of a nested literal.
func summarize(groups []TimeGroup) string {
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		ids := make([]string, 0, len(group.Issues))
		for _, issue := range group.Issues {
			ids = append(ids, issue.ID)
		}
		parts = append(parts, group.Bucket.Label()+":"+strings.Join(ids, ","))
	}
	return strings.Join(parts, "|")
}

func TestGroupByTimeOrdersBucketsNewestFirst(t *testing.T) {
	s := New()
	// Loaded oldest-first, so the output order can only come from the grouping.
	s.LoadIssues([]*parser.Issue{
		groupIssue("i-old", at(2026, 1, 5, 9)),
		groupIssue("i-mon", at(2026, 3, 2, 9)),
		groupIssue("i-lwk", at(2026, 3, 11, 9)),
		groupIssue("i-wek", at(2026, 3, 16, 9)),
		groupIssue("i-ysd", at(2026, 3, 17, 20)),
		groupIssue("i-td1", at(2026, 3, 18, 7)),
		groupIssue("i-td2", at(2026, 3, 18, 9)),
		groupIssue("i-und", time.Time{}),
	})

	got := summarize(s.GroupByTime(CreatedAt, groupNow, false))
	want := "Today:i-td2,i-td1|Yesterday:i-ysd|This week:i-wek|Last week:i-lwk|This month:i-mon|Older:i-old|Undated:i-und"
	if got != want {
		t.Errorf("GroupByTime =\n%s\nwant\n%s", got, want)
	}
}

func TestGroupByTimeOmitsEmptyBuckets(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{groupIssue("i-td1", at(2026, 3, 18, 7))})

	groups := s.GroupByTime(CreatedAt, groupNow, false)
	if len(groups) != 1 || groups[0].Bucket != timegroup.Today {
		t.Fatalf("GroupByTime = %s, want a lone Today group", summarize(groups))
	}
}

func TestGroupByTimeBreaksTiesByID(t *testing.T) {
	same := at(2026, 3, 18, 9)
	s := New()
	s.LoadIssues([]*parser.Issue{
		groupIssue("i-ccc", same),
		groupIssue("i-aaa", same),
		groupIssue("i-bbb", same),
	})

	if got := summarize(s.GroupByTime(CreatedAt, groupNow, false)); got != "Today:i-aaa,i-bbb,i-ccc" {
		t.Errorf("GroupByTime = %s, want ids ascending on a timestamp tie", got)
	}
}

func TestGroupByTimeIncludesClosedOnlyWhenAsked(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		groupIssue("i-open", at(2026, 3, 18, 9)),
		groupIssue("i-shut", at(2026, 3, 18, 8), func(i *parser.Issue) { i.Status = parser.StatusClosed }),
	})

	if got := summarize(s.GroupByTime(CreatedAt, groupNow, false)); got != "Today:i-open" {
		t.Errorf("GroupByTime without closed = %s, want just the open issue", got)
	}
	if got := summarize(s.GroupByTime(CreatedAt, groupNow, true)); got != "Today:i-open,i-shut" {
		t.Errorf("GroupByTime with closed = %s, want both", got)
	}
}

func TestGroupByTimeRespectsFilters(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		groupIssue("i-p1", at(2026, 3, 18, 9), func(i *parser.Issue) { i.Priority = 1 }),
		groupIssue("i-p3", at(2026, 3, 17, 9), func(i *parser.Issue) { i.Priority = 3 }),
	})
	s.TogglePriorityFilter(1)

	if got := summarize(s.GroupByTime(CreatedAt, groupNow, false)); got != "Today:i-p1" {
		t.Errorf("GroupByTime under a P1 filter = %s, want just i-p1", got)
	}
}

// Blocked and in-progress issues are grouped by time like any other, since the
// time views replace the status grouping rather than nesting inside it.
func TestGroupByTimeMixesStatusesInOneBucket(t *testing.T) {
	s := New()
	s.LoadIssues([]*parser.Issue{
		groupIssue("i-prog", at(2026, 3, 18, 9), func(i *parser.Issue) { i.Status = parser.StatusInProgress }),
		groupIssue("i-blok", at(2026, 3, 18, 8), func(i *parser.Issue) { i.Status = parser.StatusBlocked }),
		groupIssue("i-open", at(2026, 3, 18, 7)),
	})

	if got := summarize(s.GroupByTime(CreatedAt, groupNow, false)); got != "Today:i-prog,i-blok,i-open" {
		t.Errorf("GroupByTime = %s, want all three in Today, newest first", got)
	}
}
