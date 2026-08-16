package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
)

// Every dialog key opens its page and ESC puts the app back on "main".
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
	} {
		t.Run(tc.page, func(t *testing.T) {
			h := newHarness(t, basicIssues())
			h.typeRunes("j") // the issue-scoped dialogs need a selected issue

			h.typeRunes(tc.key)
			if got := h.frontPage(); got != tc.page {
				t.Fatalf("%q opened page %q, want %q", tc.key, got, tc.page)
			}

			h.key(tcell.KeyEscape)
			if got := h.frontPage(); got != "main" {
				t.Fatalf("ESC left page %q up, want main", got)
			}
			if calls := h.calls(); len(calls) != 0 {
				t.Errorf("dismissing the dialog ran bd: %v", calls)
			}
		})
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

// Opening a dialog on a group header is a no-op rather than a crash: there is
// no issue under the cursor for the dialog to act on.
func TestDialogs_IssueScopedDialogsIgnoreAHeaderRow(t *testing.T) {
	h := newHarness(t, basicIssues())
	if id := h.currentIssueID(); id != "" {
		t.Fatalf("test setup: expected to start on a header, got %q", id)
	}

	for _, key := range []string{"e", "c", "R", "x", "D", "L"} {
		h.typeRunes(key)
		if got := h.frontPage(); got != "main" {
			t.Errorf("%q opened %q from a header row", key, got)
			h.key(tcell.KeyEscape)
		}
	}
}
