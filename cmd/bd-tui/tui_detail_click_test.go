package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
)

// Clicking the ID line in the detail panel copies the ID. A title long enough
// to wrap pushes the ID line down a row, and the click has to follow it.
func TestDetailClick_IDLineCopiesID(t *testing.T) {
	for _, tc := range []struct{ name, title string }{
		{"short title", "Bravo issue"},
		{"wrapped title", strings.Repeat("a title long enough to wrap ", 6)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := basicIssues()
			issues[1].Title = tc.title
			h := newHarness(t, issues)
			h.selectIssue("tui-bbb")

			h.clickText("click to copy")

			if got := h.copied(); len(got) != 1 || got[0] != "tui-bbb" {
				t.Fatalf("clicking the ID line copied %q, want [tui-bbb]", got)
			}
			if status := h.statusText(); !strings.Contains(status, "Copied tui-bbb") {
				t.Errorf("status bar does not report the copy: %q", status)
			}
		})
	}
}

// detailShows reports whether the detail panel's ID line names id. The ID line
// is unique to the detail panel, so this checks what the panel rendered rather
// than what a list row happens to say.
func detailShows(t *testing.T, h *harness, id string) bool {
	t.Helper()
	return strings.Contains(rowContaining(t, h, "click to copy"), id)
}

// Clicking a dependency line moves the list selection to the issue it names,
// and the detail panel follows.
func TestDetailClick_DependencyLineSelectsTarget(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-aaa", "Alpha issue", withBlockedBy("tui-bbb")),
		newIssue("tui-bbb", "Bravo issue"),
	})
	h.selectIssue("tui-aaa")

	h.clickText("blocked by tui-bbb")

	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Fatalf("selected %q after the click, want tui-bbb", got)
	}
	if !detailShows(t, h, "tui-bbb") {
		t.Errorf("the detail panel did not follow to tui-bbb:\n%s", h.screenText())
	}
	if got := h.copied(); len(got) != 0 {
		t.Errorf("a dependency click copied %q; only the ID line copies", got)
	}
}

// The target of a dependency is often closed, and closed issues are hidden by
// default. The click reveals it rather than doing nothing.
func TestDetailClick_DependencyLineRevealsAClosedTarget(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-aaa", "Alpha issue", withBlockedBy("tui-bbb")),
		newIssue("tui-bbb", "Bravo issue", withStatus(parser.StatusClosed)),
	})
	if ids := h.issueIDs(); len(ids) != 1 || ids[0] != "tui-aaa" {
		t.Fatalf("fixture: expected only tui-aaa visible with closed hidden, got %v", ids)
	}
	h.selectIssue("tui-aaa")

	h.clickText("blocked by tui-bbb")

	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Fatalf("selected %q after the click, want the closed tui-bbb; rows are %v", got, h.issueIDs())
	}
	if !detailShows(t, h, "tui-bbb") {
		t.Errorf("the detail panel did not follow to tui-bbb:\n%s", h.screenText())
	}
	if status := h.statusText(); !strings.Contains(status, "[revealed]") {
		t.Errorf("status bar does not say the target was revealed: %q", status)
	}
}

// In the paged layout the detail page stays up and repaints for the target.
// The panel is narrow here, so the description and the first dependency line
// above the clicked one each wrap onto several rows; the click has to land on
// the second dependency anyway.
func TestDetailClick_DependencyLineInPagedLayoutBelowWrappedLines(t *testing.T) {
	alpha := newIssue("tui-aaa", "Alpha issue",
		withBlockedBy("tui-bbb"), withBlockedBy("tui-ccc"))
	alpha.Description = strings.Repeat("a description long enough to wrap ", 8)
	h := newHarness(t, []*parser.Issue{
		alpha,
		newIssue("tui-bbb", strings.Repeat("a blocker title that wraps ", 4)),
		newIssue("tui-ccc", "Charlie issue"),
	}, withScreenSize(60, 30))
	h.selectIssue("tui-aaa")
	h.key(tcell.KeyTab) // turn the page to the detail

	h.clickText("blocked by tui-ccc")

	if got := h.currentIssueID(); got != "tui-ccc" {
		t.Fatalf("selected %q after the click, want tui-ccc", got)
	}
	// The status bar is showing the jump message, so check what is mounted: a
	// group header on screen means the list page came back.
	if text := h.screenText(); strings.Contains(text, "READY (") {
		t.Fatalf("the click turned the page back to the list:\n%s", text)
	}
	var detailFocused bool
	h.read(func() { detailFocused = h.tui.app.GetFocus() == h.tui.detailPanel })
	if !detailFocused {
		t.Fatalf("the detail panel lost focus after the click")
	}
	if !detailShows(t, h, "tui-ccc") {
		t.Errorf("the detail page did not repaint for tui-ccc:\n%s", h.screenText())
	}
}

// A dependency on an issue this store does not have says so, and leaves the
// selection where it was.
func TestDetailClick_UnresolvedDependencyReportsAnError(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-aaa", "Alpha issue", withBlockedBy("tui-gone")),
		newIssue("tui-bbb", "Bravo issue"),
	})
	h.selectIssue("tui-aaa")

	h.clickText("blocked by tui-gone")

	if got := h.currentIssueID(); got != "tui-aaa" {
		t.Fatalf("selection moved to %q; an unresolved dependency should leave it on tui-aaa", got)
	}
	if status := h.statusText(); !strings.Contains(status, "tui-gone not found") {
		t.Errorf("status bar does not report the missing issue: %q", status)
	}
}
