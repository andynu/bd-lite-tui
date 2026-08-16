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

// The quick filter dialog parses its query and installs the filter.
//
// The list itself is not asserted: the dialog never repaints after applying, so
// the rows stay stale until something else repopulates (tui-zloy). Tighten this
// to assert the visible rows once that is fixed.
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

	// A repopulate (any key that rebuilds the list) then shows only the P1 row.
	h.typeRunes("p") // toggle the id prefix; harmless, and it repopulates
	if ids := h.issueIDs(); len(ids) != 1 || ids[0] != "tui-aaa" {
		t.Fatalf("filtered list is %v, want just tui-aaa", ids)
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
