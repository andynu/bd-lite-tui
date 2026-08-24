package main

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/gdamore/tcell/v2"
)

// updatedViewNow is the moment the Updated view tests bucket against. It is a
// Wednesday, so the week opened on Monday the 16th and "This week" is a span
// with room in it rather than a synonym for "Yesterday".
var updatedViewNow = time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)

// updatedIssues places one issue in every bucket timegroup knows about, plus a
// second in Today so the within-bucket ordering has something to order.
//
// Only UpdatedAt is set. CreatedAt stays at fixtureTime, which is January and
// so lands in Older: a renderer keyed on the wrong timestamp would pile the
// whole fixture into one bucket and fail these tests rather than pass them.
func updatedIssues() []*parser.Issue {
	at := func(y int, m time.Month, d, hour int) time.Time {
		return time.Date(y, m, d, hour, 0, 0, 0, time.UTC)
	}
	return []*parser.Issue{
		newIssue("tui-utd1", "Touched early today", withUpdatedAt(at(2026, 3, 18, 7))),
		newIssue("tui-utd2", "Touched late today", withUpdatedAt(at(2026, 3, 18, 9))),
		newIssue("tui-uysd", "Touched yesterday", withUpdatedAt(at(2026, 3, 17, 20))),
		newIssue("tui-uwek", "Touched this week", withUpdatedAt(at(2026, 3, 16, 9))),
		newIssue("tui-ulwk", "Touched last week", withUpdatedAt(at(2026, 3, 11, 9))),
		newIssue("tui-umon", "Touched this month", withUpdatedAt(at(2026, 3, 2, 9))),
		newIssue("tui-uold", "Touched long ago", withUpdatedAt(at(2026, 1, 5, 9))),
		newIssue("tui-uund", "Never stamped", withUpdatedAt(time.Time{})),
		newIssue("tui-ucls", "Closed today", withUpdatedAt(at(2026, 3, 18, 8)), withStatus(parser.StatusClosed)),
	}
}

func TestUpdatedViewGroupsByBucket(t *testing.T) {
	h := newHarness(t, updatedIssues(), withView(state.ViewUpdated), withNow(updatedViewNow))

	order, rows := sectionsOnScreen(t, h, updatedIssues())

	wantOrder := timeBucketHeaders
	if strings.Join(order, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("group headers = %v, want %v\n%s", order, wantOrder, h.screenText())
	}

	want := map[string][]string{
		"TODAY":      {"tui-utd2", "tui-utd1"}, // newest first within the bucket
		"YESTERDAY":  {"tui-uysd"},
		"THIS WEEK":  {"tui-uwek"},
		"LAST WEEK":  {"tui-ulwk"},
		"THIS MONTH": {"tui-umon"},
		"OLDER":      {"tui-uold"},
		"UNDATED":    {"tui-uund"},
	}
	for header, wantIDs := range want {
		got := strings.Join(rows[header], ",")
		if got != strings.Join(wantIDs, ",") {
			t.Errorf("rows under %s = %v, want %v\n%s", header, rows[header], wantIDs, h.screenText())
		}
	}

	// The closed issue was touched today but stays hidden until C.
	if strings.Contains(h.screenText(), "tui-ucls") {
		t.Errorf("closed issue rendered without the closed toggle:\n%s", h.screenText())
	}
}

func TestUpdatedViewSkipsEmptyBuckets(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-utd1", "Touched an hour ago", withUpdatedAt(updatedViewNow.Add(-time.Hour))),
	}
	h := newHarness(t, issues, withView(state.ViewUpdated), withNow(updatedViewNow))

	order, _ := sectionsOnScreen(t, h, issues)
	if len(order) != 1 || order[0] != "TODAY" {
		t.Fatalf("group headers = %v, want [TODAY] only\n%s", order, h.screenText())
	}
}

// Bucket headers are rows like the status headers are: the cursor can rest on
// one, but nothing acts on it, because it carries no issue.
func TestUpdatedViewHeadersCarryNoIssue(t *testing.T) {
	h := newHarness(t, updatedIssues(), withView(state.ViewUpdated), withNow(updatedViewNow))

	if id := h.currentIssueID(); id != "" {
		t.Fatalf("row 0 is issue %q, want the TODAY header", id)
	}

	// A priority digit on a header must not reach bd.
	h.typeRunes("0")
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("bd ran for a header row: %v", calls)
	}

	h.typeRunes("j")
	if id := h.currentIssueID(); id != "tui-utd2" {
		t.Fatalf("after j, selected %q, want tui-utd2 (newest in TODAY)", id)
	}

	// Every issue row, in display order, and no header among them.
	want := "tui-utd2,tui-utd1,tui-uysd,tui-uwek,tui-ulwk,tui-umon,tui-uold,tui-uund"
	if got := strings.Join(h.issueIDs(), ","); got != want {
		t.Errorf("issue rows are %s, want %s", got, want)
	}
}

func TestUpdatedViewClosedToggle(t *testing.T) {
	h := newHarness(t, updatedIssues(), withView(state.ViewUpdated), withNow(updatedViewNow))

	h.typeRunes("C")

	_, rows := sectionsOnScreen(t, h, updatedIssues())
	want := []string{"tui-utd2", "tui-ucls", "tui-utd1"} // 09:00, 08:00, 07:00
	if got := strings.Join(rows["TODAY"], ","); got != strings.Join(want, ",") {
		t.Errorf("rows under TODAY after C = %v, want %v\n%s", rows["TODAY"], want, h.screenText())
	}
}

func TestUpdatedViewRespectsQuickFilter(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-utd1", "Today P1", withPriority(1), withUpdatedAt(updatedViewNow.Add(-time.Hour))),
		newIssue("tui-uysd", "Yesterday P3", withPriority(3), withUpdatedAt(updatedViewNow.Add(-26*time.Hour))),
	}
	h := newHarness(t, issues, withView(state.ViewUpdated), withNow(updatedViewNow))

	h.typeRunes("f")
	h.typeRunes("p1")
	h.key(tcell.KeyEnter)

	order, rows := sectionsOnScreen(t, h, issues)
	if len(order) != 1 || order[0] != "TODAY" {
		t.Fatalf("group headers under a P1 filter = %v, want [TODAY]\n%s", order, h.screenText())
	}
	if got := strings.Join(rows["TODAY"], ","); got != "tui-utd1" {
		t.Errorf("rows under TODAY = %v, want [tui-utd1]\n%s", rows["TODAY"], h.screenText())
	}
}

// bd-lite stamps updated_at on every write, so an issue changes bucket while
// the TUI is open. The reload after a quick update has to rebuild the grouping,
// not repaint the row where it already sits.
func TestUpdatedViewRegroupsAfterABdUpdate(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-utd1", "Touched two hours ago", withUpdatedAt(updatedViewNow.Add(-2*time.Hour))),
		newIssue("tui-uold", "Untouched since January", withUpdatedAt(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))),
	}
	h := newHarness(t, issues, withView(state.ViewUpdated), withNow(updatedViewNow))

	order, rows := sectionsOnScreen(t, h, issues)
	if strings.Join(order, ",") != "TODAY,OLDER" {
		t.Fatalf("group headers = %v, want [TODAY OLDER]\n%s", order, h.screenText())
	}
	if got := strings.Join(rows["OLDER"], ","); got != "tui-uold" {
		t.Fatalf("rows under OLDER = %v, want [tui-uold]\n%s", rows["OLDER"], h.screenText())
	}

	h.selectIssue("tui-uold")
	h.typeRunes("1") // bd update tui-uold --priority 1, which restamps updated_at

	h.waitFor("the post-command reload to pick up the new updated_at", func() bool {
		issue := h.tui.state.GetIssueByID("tui-uold")
		return issue != nil && issue.UpdatedAt.Equal(updatedViewNow)
	})
	h.press() // let the redraw settle before reading the screen

	order, rows = sectionsOnScreen(t, h, issues)
	if strings.Join(order, ",") != "TODAY" {
		t.Fatalf("group headers after the update = %v, want [TODAY] with OLDER emptied out\n%s",
			order, h.screenText())
	}
	// Restamped to exactly now, so it sorts above the row from two hours ago.
	if got := strings.Join(rows["TODAY"], ","); got != "tui-uold,tui-utd1" {
		t.Errorf("rows under TODAY after the update = %v, want [tui-uold tui-utd1]\n%s",
			rows["TODAY"], h.screenText())
	}
}

// The same regrouping when the write comes from outside: bd run in another
// terminal rewrites the file and the watcher-driven refresh moves the row.
func TestUpdatedViewRegroupsOnAWatcherRefresh(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-utd1", "Touched two hours ago", withUpdatedAt(updatedViewNow.Add(-2*time.Hour))),
		newIssue("tui-uold", "Untouched since January", withUpdatedAt(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))),
	}
	h := newHarness(t, issues, withView(state.ViewUpdated), withNow(updatedViewNow), withWatcher())

	h.findIssue("tui-uold").UpdatedAt = updatedViewNow.Add(-time.Minute)
	h.writeJSONL()

	h.waitFor("the watcher to reload the external write", func() bool {
		issue := h.tui.state.GetIssueByID("tui-uold")
		return issue != nil && issue.UpdatedAt.After(updatedViewNow.Add(-2*time.Minute))
	})
	h.press()

	order, rows := sectionsOnScreen(t, h, issues)
	if strings.Join(order, ",") != "TODAY" {
		t.Fatalf("group headers after the external write = %v, want [TODAY]\n%s", order, h.screenText())
	}
	if got := strings.Join(rows["TODAY"], ","); got != "tui-uold,tui-utd1" {
		t.Errorf("rows under TODAY = %v, want [tui-uold tui-utd1]\n%s", rows["TODAY"], h.screenText())
	}
}
