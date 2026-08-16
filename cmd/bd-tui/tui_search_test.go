package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// / opens search mode, Enter runs it, and the first matching row is selected.
func TestSearch_SelectsTheFirstVisibleMatch(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("/")
	if status := h.statusText(); !strings.HasPrefix(status, "Search: ") {
		t.Fatalf("/ did not enter search mode: %q", status)
	}

	h.typeRunes("charlie")
	h.key(tcell.KeyEnter)

	if got := h.currentIssueID(); got != "tui-ccc" {
		t.Fatalf("search selected %q, want tui-ccc", got)
	}
	if status := h.statusText(); !strings.Contains(status, "1/1 matches") {
		t.Errorf("status bar did not report the match count: %q", status)
	}
}

// Regression: backspacing a multi-byte query has to drop a whole rune. Trimming
// a single byte would leave a truncated UTF-8 sequence in the query.
func TestSearch_BackspaceRemovesAWholeRune(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("/")
	h.typeRunes("café")
	if status := h.statusText(); status != "Search: café_" {
		t.Fatalf("typing multibyte text produced %q", status)
	}

	h.key(tcell.KeyBackspace2)
	status := h.statusText()
	if status != "Search: caf_" {
		t.Fatalf("backspace produced %q, want %q", status, "Search: caf_")
	}
	if strings.ContainsRune(status, utf8.RuneError) {
		t.Fatalf("backspace left a broken UTF-8 sequence: %q", status)
	}

	// Backspacing past the start is harmless.
	h.press(
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
	)
	if status := h.statusText(); status != "Search: _" {
		t.Fatalf("over-backspacing produced %q", status)
	}
}

// Multi-byte queries survive the round trip into a match.
func TestSearch_MatchesAMultibyteTitle(t *testing.T) {
	issues := basicIssues()
	issues = append(issues, newIssue("tui-uni", "Café résumé naïve"))
	h := newHarness(t, issues)

	h.typeRunes("/")
	h.typeRunes("résumé")
	h.key(tcell.KeyEnter)

	if got := h.currentIssueID(); got != "tui-uni" {
		t.Fatalf("search selected %q, want tui-uni", got)
	}
}

// Searching an id that is hidden because the issue is closed reveals it rather
// than reporting no match.
func TestSearch_RevealsAClosedIssue(t *testing.T) {
	h := newHarness(t, basicIssues())

	if ids := h.issueIDs(); containsID(ids, "tui-zzz") {
		t.Fatalf("test setup: the closed issue is already visible: %v", ids)
	}

	h.typeRunes("/")
	h.typeRunes("tui-zzz")
	h.key(tcell.KeyEnter)

	if got := h.currentIssueID(); got != "tui-zzz" {
		t.Fatalf("search selected %q, want the revealed tui-zzz", got)
	}
	status := h.statusText()
	if !strings.Contains(status, "matched tui-zzz") {
		t.Errorf("status bar did not report the match: %q", status)
	}
	// The trailing "[revealed]" note is not asserted: tview parses it as a style
	// tag and it never reaches the screen (tui-42ws).
}

// Searching an id collapsed under a parent in tree view expands the ancestors.
func TestSearch_ExpandsCollapsedAncestorsInTreeView(t *testing.T) {
	h := newHarness(t, treeIssues(), withTreeView())

	// Collapse everything, so the child is not rendered.
	h.typeRunes("Z")
	if ids := h.issueIDs(); containsID(ids, "tui-kid") {
		t.Fatalf("Z did not hide the child row: %v", ids)
	}

	h.typeRunes("/")
	h.typeRunes("tui-kid")
	h.key(tcell.KeyEnter)

	if got := h.currentIssueID(); got != "tui-kid" {
		t.Fatalf("search selected %q, want the revealed tui-kid", got)
	}
	if ids := h.issueIDs(); !containsID(ids, "tui-kid") {
		t.Errorf("the child row was selected but never rendered: %v", ids)
	}
}

// A query that matches nothing anywhere says so and leaves the selection alone.
func TestSearch_NoMatchReportsAndKeepsTheSelection(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j")
	before := h.currentIssueID()

	h.typeRunes("/")
	h.typeRunes("nothing-matches-this")
	h.key(tcell.KeyEnter)

	if status := h.statusText(); !strings.Contains(status, "No matches") {
		t.Fatalf("status bar did not report the empty result: %q", status)
	}
	if got := h.currentIssueID(); got != before {
		t.Errorf("a failed search moved the selection from %q to %q", before, got)
	}
}

// n and N walk the match list, wrapping in both directions.
func TestSearch_NextAndPreviousWrapAround(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("/")
	h.typeRunes("issue") // matches all three ready rows
	h.key(tcell.KeyEnter)

	if got := h.currentIssueID(); got != "tui-aaa" {
		t.Fatalf("first match is %q, want tui-aaa", got)
	}
	h.typeRunes("n")
	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Fatalf("after n: %q, want tui-bbb", got)
	}
	h.typeRunes("nn") // past the end, wrapping to the first
	if got := h.currentIssueID(); got != "tui-aaa" {
		t.Fatalf("after wrapping forward: %q, want tui-aaa", got)
	}
	h.typeRunes("N") // backwards past the start, wrapping to the last
	if got := h.currentIssueID(); got != "tui-ccc" {
		t.Fatalf("after wrapping backward: %q, want tui-ccc", got)
	}
}

// ESC while typing abandons the search without running it.
func TestSearch_EscapeAbandonsTheQuery(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j")
	before := h.currentIssueID()

	h.typeRunes("/")
	h.typeRunes("charlie")
	h.key(tcell.KeyEscape)

	if status := h.statusText(); strings.HasPrefix(status, "Search:") {
		t.Fatalf("ESC left search mode showing: %q", status)
	}
	if got := h.currentIssueID(); got != before {
		t.Errorf("an abandoned search still moved the selection from %q to %q", before, got)
	}
}
