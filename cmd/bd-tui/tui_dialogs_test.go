package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
)

// Every dialog key opens its page and ESC puts the app back on "main", from
// either panel: the detail panel keeps only the keys it uses itself.
func TestDialogs_OpenAndCloseOnEscape(t *testing.T) {
	for _, tc := range []struct {
		key  string
		page string
	}{
		{"?", "help"},
		{"S", "stats"},
		{"f", "quick_filter"},
		{"a", "create_issue"},
		{"e", "edit_form"},
		{"c", "comment_dialog"},
		{"R", "rename_dialog"},
		{"x", "close_issue_dialog"},
		{"D", "dependency_dialog"},
		{"L", "label_dialog"},
		{"A", "assign_dialog"},
	} {
		for _, panel := range []string{"list", "detail"} {
			t.Run(tc.page+"/"+panel, func(t *testing.T) {
				h := newHarness(t, basicIssues())
				h.typeRunes("j") // the issue-scoped dialogs need a selected issue
				if panel == "detail" {
					h.key(tcell.KeyTab)
				}

				h.typeRunes(tc.key)
				if got := h.frontPage(); got != tc.page {
					t.Fatalf("%q from the %s opened page %q, want %q", tc.key, panel, got, tc.page)
				}

				h.key(tcell.KeyEscape)
				if got := h.frontPage(); got != "main" {
					t.Fatalf("ESC left page %q up, want main", got)
				}
				if calls := h.calls(); len(calls) != 0 {
					t.Errorf("dismissing the dialog ran bd: %v", calls)
				}

				if panel == "detail" {
					if status := h.statusText(); !strings.Contains(status, "Focus: Details") {
						t.Fatalf("closing the dialog moved focus out of the detail panel: %q", status)
					}
					// Dialogs hand tview focus to the list on the way out; the
					// detail bindings must not depend on where it went.
					h.typeRunes("j")
					if got := h.currentIssueID(); got != "tui-bbb" {
						t.Fatalf("j after the dialog closed: selected %q, want tui-bbb", got)
					}
				}
			})
		}
	}
}

// While a dialog is up the main key bindings must stay inert, so keys meant for
// the dialog do not also move the cursor or quit the app underneath it.
func TestDialogs_SwallowTheMainKeyBindings(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("jj") // tui-bbb
	before := h.currentIndex()

	h.typeRunes("?")
	if got := h.frontPage(); got != "help" {
		t.Fatalf("help did not open, front page is %q", got)
	}

	h.typeRunes("j") // would move the selection on the main page
	h.typeRunes("G") // would jump to the bottom
	if got := h.currentIndex(); got != before {
		t.Errorf("keys leaked through the dialog: selection moved from %d to %d", before, got)
	}

	h.key(tcell.KeyEscape)
	if got := h.frontPage(); got != "main" {
		t.Fatalf("ESC did not close the help page, front page is %q", got)
	}

	// Bindings work again once the dialog is gone.
	h.typeRunes("G")
	if got, want := h.currentIndex(), h.rowCount()-1; got != want {
		t.Errorf("after closing the dialog G moved to %d, want %d", got, want)
	}
}

// The help screen also closes on a second ?, and its content actually renders.
func TestDialogs_HelpTogglesAndRenders(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("?")
	text := h.screenText()
	// Only the first screenful is asserted; the modal is not scrolled.
	for _, want := range []string{"Help - Keyboard Shortcuts", "Quick Start", "gg          Jump to top"} {
		if !strings.Contains(text, want) {
			t.Errorf("the help screen is missing %q:\n%s", want, text)
		}
	}

	h.typeRunes("?")
	if got := h.frontPage(); got != "main" {
		t.Fatalf("a second ? did not close the help screen, front page is %q", got)
	}
}

// The help screen is ~200 lines and only ~20 fit on an 80x24 terminal, so the
// footer that says it scrolls has to be outside the scrolling body. tui-4gih:
// the old hint was the last line of helpText, which the reader never reached.
//
// Run at the size the bug is about. This briefly had to use the harness default
// instead: below layout.NarrowThreshold the first draw queues a layout rebuild
// that used to re-add the main page on top of whatever dialog was open, burying
// the help about a third of the time. That is tui-bl1e, now fixed.
func TestDialogs_HelpFooterIsPinnedAndTracksScrolling(t *testing.T) {
	h := newHarness(t, basicIssues(), withScreenSize(80, 24))

	h.typeRunes("?")
	first := h.screenText()
	for _, want := range []string{"↑/↓ PgUp/PgDn scroll", "ESC or ? closes", "Top"} {
		if !strings.Contains(first, want) {
			t.Errorf("the unscrolled help screen is missing %q:\n%s", want, first)
		}
	}

	h.key(tcell.KeyPgDn)
	paged := h.screenText()
	if !strings.Contains(paged, "↑/↓ PgUp/PgDn scroll") {
		t.Errorf("the footer scrolled away with the body:\n%s", paged)
	}
	if strings.Contains(paged, "Top") {
		t.Errorf("the position still reads Top after PgDn:\n%s", paged)
	}
	if paged == first {
		t.Error("PgDn did not scroll the help body")
	}

	h.typeRunes("G")
	if end := h.screenText(); !strings.Contains(end, "Bot") {
		t.Errorf("G did not put the position field at Bot:\n%s", end)
	}
}

// Resizing the terminal across layout.NarrowThreshold rebuilds the main page,
// and tview appends a re-added page to the end of its list. That drew "main"
// over any open dialog and made it GetFrontPage(), so the dialog vanished while
// it still held the keyboard and every key went somewhere invisible (tui-bl1e).
//
// Both directions are covered: a user who narrows their terminal with a dialog
// up, and one who widens it again.
func TestDialogs_SurviveALayoutRemount(t *testing.T) {
	for _, tc := range []struct {
		name          string
		start, resize [2]int
	}{
		{"narrowing", [2]int{120, 24}, [2]int{80, 24}},
		{"widening", [2]int{80, 24}, [2]int{120, 24}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, basicIssues(), withScreenSize(tc.start[0], tc.start[1]))

			h.typeRunes("S")
			if got := h.frontPage(); got != "stats" {
				t.Fatalf("S did not open the stats dashboard, front page is %q", got)
			}

			h.resizeAcrossLayoutThreshold(tc.resize[0], tc.resize[1])

			if got := h.frontPage(); got != "stats" {
				t.Fatalf("the layout rebuild buried the dialog: front page is %q", got)
			}
			if text := h.screenText(); !strings.Contains(text, "Statistics Dashboard") {
				t.Fatalf("the dialog stopped being drawn after the rebuild:\n%s", text)
			}

			// The dialog still owns the keyboard, so ESC closes it rather than
			// arming the list's quit hint underneath.
			h.key(tcell.KeyEscape)
			if got := h.frontPage(); got != "main" {
				t.Fatalf("ESC did not close the dialog after the rebuild, front page is %q", got)
			}
			if status := h.statusText(); strings.Contains(status, "Press ESC again to quit") {
				t.Fatalf("ESC reached the issue list instead of the dialog: %q", status)
			}
		})
	}
}

// The stats dashboard counts the issues it was given.
func TestDialogs_StatsCountsTheIssues(t *testing.T) {
	issues := append(basicIssues(),
		newIssue("tui-wip", "Work in flight", withStatus(parser.StatusInProgress)))
	h := newHarness(t, issues)

	h.typeRunes("S")
	text := h.screenText()
	if !strings.Contains(text, "Statistics Dashboard") {
		t.Fatalf("stats dashboard did not render:\n%s", text)
	}
	if !strings.Contains(text, "Total Issues: 5") {
		t.Errorf("stats reported the wrong total:\n%s", text)
	}

	h.typeRunes("S")
	if got := h.frontPage(); got != "main" {
		t.Fatalf("a second S did not close the dashboard, front page is %q", got)
	}
}

// The quick filter dialog parses its query, installs the filter, and repaints
// the list and status bar on the way out (tui-zloy).
func TestQuickFilter_AppliesTheFilter(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("f")
	if got := h.frontPage(); got != "quick_filter" {
		t.Fatalf("f opened %q, want quick_filter", got)
	}
	h.typeRunes("p1")
	h.key(tcell.KeyEnter)

	if got := h.frontPage(); got != "main" {
		t.Fatalf("Enter left %q up, want main", got)
	}

	var active string
	h.read(func() { active = h.tui.state.GetActiveFilters() })
	if active != "Priority: P1" {
		t.Fatalf("filter is %q, want %q", active, "Priority: P1")
	}

	// No further keypress: the rows are filtered as soon as the dialog closes.
	if ids := h.issueIDs(); len(ids) != 1 || ids[0] != "tui-aaa" {
		t.Fatalf("filtered list is %v, want just tui-aaa", ids)
	}
	if text := h.screenText(); !strings.Contains(text, "⊘ FILTERED") {
		t.Errorf("no FILTERED banner on screen:\n%s", text)
	}
	if text := h.statusText(); !strings.Contains(text, "[Filters: Priority: P1]") {
		t.Errorf("status bar is %q, want it to mention the active filter", text)
	}
}

// Re-opening the quick filter with an empty query clears the filter and puts
// every row back, again without needing an unrelated repopulate.
func TestQuickFilter_EmptyQueryClearsTheFilter(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("f")
	h.typeRunes("p1")
	h.key(tcell.KeyEnter)
	if ids := h.issueIDs(); len(ids) != 1 {
		t.Fatalf("test setup: filtered list is %v, want one row", ids)
	}

	h.typeRunes("f")
	h.key(tcell.KeyEnter) // empty query = clear all filters

	var hasFilters bool
	h.read(func() { hasFilters = h.tui.state.HasActiveFilters() })
	if hasFilters {
		t.Fatal("filters still active after an empty query")
	}
	if ids := h.issueIDs(); len(ids) != 3 {
		t.Fatalf("list is %v, want all three open issues back", ids)
	}
	if text := h.screenText(); strings.Contains(text, "⊘ FILTERED") {
		t.Errorf("FILTERED banner still on screen:\n%s", text)
	}
	if text := h.statusText(); strings.Contains(text, "[Filters:") {
		t.Errorf("status bar still shows a filter: %q", text)
	}
}

// Enter is the form-wide submit only while the typing field has focus. Tabbing
// to "Clear All" and pressing Enter must clear, not apply what was typed
// (tui-1gf3).
func TestQuickFilter_ClearAllButtonDiscardsTheQuery(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("f")
	h.typeRunes("p1")
	h.pressButton("Clear All")

	if got := h.frontPage(); got != "main" {
		t.Fatalf("Clear All left %q up, want main", got)
	}

	var hasFilters bool
	h.read(func() { hasFilters = h.tui.state.HasActiveFilters() })
	if hasFilters {
		var active string
		h.read(func() { active = h.tui.state.GetActiveFilters() })
		t.Fatalf("Clear All applied the typed query instead of clearing: %q", active)
	}
	if ids := h.issueIDs(); len(ids) != 3 {
		t.Fatalf("list is %v, want all three open issues", ids)
	}
}

// Same for "Cancel (ESC)": Enter on it dismisses the dialog and leaves the
// existing filter state untouched (tui-1gf3).
func TestQuickFilter_CancelButtonKeepsTheExistingFilters(t *testing.T) {
	h := newHarness(t, basicIssues())

	// Establish a filter the cancelled dialog must not disturb.
	h.typeRunes("f")
	h.typeRunes("p3")
	h.key(tcell.KeyEnter)
	if ids := h.issueIDs(); len(ids) != 1 || ids[0] != "tui-ccc" {
		t.Fatalf("test setup: filtered list is %v, want just tui-ccc", ids)
	}

	h.typeRunes("f")
	h.typeRunes("p1")
	h.pressButton("Cancel")

	if got := h.frontPage(); got != "main" {
		t.Fatalf("Cancel left %q up, want main", got)
	}

	var active string
	h.read(func() { active = h.tui.state.GetActiveFilters() })
	if active != "Priority: P3" {
		t.Fatalf("filter is %q, want the pre-existing %q", active, "Priority: P3")
	}
	if ids := h.issueIDs(); len(ids) != 1 || ids[0] != "tui-ccc" {
		t.Fatalf("list is %v, want the P3 filter still in force", ids)
	}
}

// The close dialog submits on Enter from the reason field, which is the one
// keystroke path the title advertises.
func TestCloseDialog_EnterInTheReasonFieldCloses(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.selectIssue("tui-aaa")

	h.typeRunes("x")
	h.typeRunes("done")
	h.key(tcell.KeyEnter)

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"close", "tui-aaa", "--reason", "done"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
}

// Enter on the close dialog's Cancel button must cancel. The form-wide capture
// used to close the issue no matter which button held focus (tui-1gf3).
func TestCloseDialog_CancelButtonDoesNotCloseTheIssue(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.selectIssue("tui-aaa")

	h.typeRunes("x")
	h.typeRunes("oops")
	h.pressButton("Cancel")

	if got := h.frontPage(); got != "main" {
		t.Fatalf("Cancel left %q up, want main", got)
	}
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("cancelling ran bd: %v", calls)
	}
}

// The "Close Issue" button still works when Enter reaches it directly.
func TestCloseDialog_CloseButtonClosesTheIssue(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.selectIssue("tui-aaa")

	h.typeRunes("x")
	h.typeRunes("shipped")
	h.pressButton("Close Issue")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"close", "tui-aaa", "--reason", "shipped"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
}

// Opening a dialog on a group header is a no-op rather than a crash: there is
// no issue under the cursor for the dialog to act on.
func TestDialogs_IssueScopedDialogsIgnoreAHeaderRow(t *testing.T) {
	h := newHarness(t, basicIssues())
	if id := h.currentIssueID(); id != "" {
		t.Fatalf("test setup: expected to start on a header, got %q", id)
	}

	for _, key := range []string{"e", "c", "R", "x", "D", "L", "A"} {
		h.typeRunes(key)
		if got := h.frontPage(); got != "main" {
			t.Errorf("%q opened %q from a header row", key, got)
			h.key(tcell.KeyEscape)
		}
	}
}
