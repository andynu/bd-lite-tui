package main

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/gdamore/tcell/v2"
)

// createdViewNow is the moment the Created view tests bucket against. It is a
// Wednesday, so the week opened on Monday the 16th and "This week" is a span
// with room in it rather than a synonym for "Yesterday".
var createdViewNow = time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC)

// createdIssues places one issue in every bucket timegroup knows about, plus a
// second in Today so the within-bucket ordering has something to order.
func createdIssues() []*parser.Issue {
	at := func(y int, m time.Month, d, hour int) time.Time {
		return time.Date(y, m, d, hour, 0, 0, 0, time.UTC)
	}
	return []*parser.Issue{
		newIssue("tui-td1", "Today early", withCreatedAt(at(2026, 3, 18, 7))),
		newIssue("tui-td2", "Today late", withCreatedAt(at(2026, 3, 18, 9))),
		newIssue("tui-ysd", "Yesterday issue", withCreatedAt(at(2026, 3, 17, 20))),
		newIssue("tui-wek", "This week issue", withCreatedAt(at(2026, 3, 16, 9))),
		newIssue("tui-lwk", "Last week issue", withCreatedAt(at(2026, 3, 11, 9))),
		newIssue("tui-mon", "This month issue", withCreatedAt(at(2026, 3, 2, 9))),
		newIssue("tui-old", "Older issue", withCreatedAt(at(2026, 1, 5, 9))),
		newIssue("tui-und", "Undated issue", withCreatedAt(time.Time{})),
		newIssue("tui-cls", "Closed today", withCreatedAt(at(2026, 3, 18, 8)), withStatus(parser.StatusClosed)),
	}
}

// createdHeaders are the group headers, in the order the view must draw them.
var createdHeaders = []string{"TODAY", "YESTERDAY", "THIS WEEK", "LAST WEEK", "THIS MONTH", "OLDER", "UNDATED"}

// sectionsOnScreen splits the rendered screen into header -> rows sections, in
// screen order. Reading it back off the screen rather than out of the widget is
// the point: it is what the user sees, tview's tag parsing included.
func sectionsOnScreen(t *testing.T, h *harness) ([]string, map[string][]string) {
	t.Helper()

	var order []string
	rows := map[string][]string{}
	current := ""
	for _, line := range h.screenLines() {
		if header := headerOnLine(line); header != "" {
			order = append(order, header)
			current = header
			continue
		}
		if current == "" {
			continue
		}
		if id := issueIDOnLine(line); id != "" {
			rows[current] = append(rows[current], id)
		}
	}
	return order, rows
}

// headerOnLine returns the bucket header a rendered line carries, or "".
func headerOnLine(line string) string {
	for _, header := range createdHeaders {
		if strings.Contains(line, "⬤ "+header+" (") {
			return header
		}
	}
	return ""
}

// issueIDOnLine returns the fixture issue id a rendered line carries, or "".
// Rows are truncated at the pane's width, but the id sits near the left edge
// and always survives.
func issueIDOnLine(line string) string {
	for _, issue := range createdIssues() {
		if strings.Contains(line, issue.ID) {
			return issue.ID
		}
	}
	return ""
}

func TestCreatedViewGroupsByBucket(t *testing.T) {
	h := newHarness(t, createdIssues(), withView(state.ViewCreated), withNow(createdViewNow))

	order, rows := sectionsOnScreen(t, h)

	wantOrder := createdHeaders
	if strings.Join(order, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("group headers = %v, want %v\n%s", order, wantOrder, h.screenText())
	}

	want := map[string][]string{
		"TODAY":      {"tui-td2", "tui-td1"}, // newest first within the bucket
		"YESTERDAY":  {"tui-ysd"},
		"THIS WEEK":  {"tui-wek"},
		"LAST WEEK":  {"tui-lwk"},
		"THIS MONTH": {"tui-mon"},
		"OLDER":      {"tui-old"},
		"UNDATED":    {"tui-und"},
	}
	for header, wantIDs := range want {
		got := strings.Join(rows[header], ",")
		if got != strings.Join(wantIDs, ",") {
			t.Errorf("rows under %s = %v, want %v\n%s", header, rows[header], wantIDs, h.screenText())
		}
	}

	// The closed issue was created today but stays hidden until C.
	if strings.Contains(h.screenText(), "tui-cls") {
		t.Errorf("closed issue rendered without the closed toggle:\n%s", h.screenText())
	}
}

func TestCreatedViewSkipsEmptyBuckets(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-td1", "Today only", withCreatedAt(createdViewNow.Add(-time.Hour))),
	}
	h := newHarness(t, issues, withView(state.ViewCreated), withNow(createdViewNow))

	order, _ := sectionsOnScreen(t, h)
	if len(order) != 1 || order[0] != "TODAY" {
		t.Fatalf("group headers = %v, want [TODAY] only\n%s", order, h.screenText())
	}
}

// Bucket headers are rows like the status headers are: the cursor can rest on
// one, but nothing acts on it, because it carries no issue.
func TestCreatedViewHeadersCarryNoIssue(t *testing.T) {
	h := newHarness(t, createdIssues(), withView(state.ViewCreated), withNow(createdViewNow))

	if id := h.currentIssueID(); id != "" {
		t.Fatalf("row 0 is issue %q, want the TODAY header", id)
	}

	// A priority digit on a header must not reach bd.
	h.typeRunes("0")
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("bd ran for a header row: %v", calls)
	}

	h.typeRunes("j")
	if id := h.currentIssueID(); id != "tui-td2" {
		t.Fatalf("after j, selected %q, want tui-td2 (newest in TODAY)", id)
	}

	// Every issue row, in display order, and no header among them.
	want := "tui-td2,tui-td1,tui-ysd,tui-wek,tui-lwk,tui-mon,tui-old,tui-und"
	if got := strings.Join(h.issueIDs(), ","); got != want {
		t.Errorf("issue rows are %s, want %s", got, want)
	}
}

func TestCreatedViewClosedToggle(t *testing.T) {
	h := newHarness(t, createdIssues(), withView(state.ViewCreated), withNow(createdViewNow))

	h.typeRunes("C")

	_, rows := sectionsOnScreen(t, h)
	want := []string{"tui-td2", "tui-cls", "tui-td1"} // 09:00, 08:00, 07:00
	if got := strings.Join(rows["TODAY"], ","); got != strings.Join(want, ",") {
		t.Errorf("rows under TODAY after C = %v, want %v\n%s", rows["TODAY"], want, h.screenText())
	}
}

func TestCreatedViewRespectsQuickFilter(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-td1", "Today P1", withPriority(1), withCreatedAt(createdViewNow.Add(-time.Hour))),
		newIssue("tui-ysd", "Yesterday P3", withPriority(3), withCreatedAt(createdViewNow.Add(-26*time.Hour))),
	}
	h := newHarness(t, issues, withView(state.ViewCreated), withNow(createdViewNow))

	h.typeRunes("f")
	h.typeRunes("p1")
	h.key(tcell.KeyEnter)

	order, rows := sectionsOnScreen(t, h)
	if len(order) != 1 || order[0] != "TODAY" {
		t.Fatalf("group headers under a P1 filter = %v, want [TODAY]\n%s", order, h.screenText())
	}
	if got := strings.Join(rows["TODAY"], ","); got != "tui-td1" {
		t.Errorf("rows under TODAY = %v, want [tui-td1]\n%s", rows["TODAY"], h.screenText())
	}
}
