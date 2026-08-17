package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/layout"
	"github.com/gdamore/tcell/v2"
)

// The list opens on the READY group header, and the first j selects the first
// issue and pushes it into the detail panel.
func TestNavigation_JAndKMoveThroughIssues(t *testing.T) {
	h := newHarness(t, basicIssues())

	if got := h.currentIssueID(); got != "" {
		t.Fatalf("expected the initial selection to be the group header, got issue %q", got)
	}

	h.typeRunes("j")
	if got := h.currentIssueID(); got != "tui-aaa" {
		t.Fatalf("after j: selected %q, want tui-aaa", got)
	}
	if detail := h.detailText(); !strings.Contains(detail, "tui-aaa") {
		t.Errorf("detail panel did not follow the selection:\n%s", detail)
	}

	h.typeRunes("jj")
	if got := h.currentIssueID(); got != "tui-ccc" {
		t.Fatalf("after jjj: selected %q, want tui-ccc", got)
	}

	h.typeRunes("k")
	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Fatalf("after k: selected %q, want tui-bbb", got)
	}
}

// G jumps to the last row, gg back to the first.
func TestNavigation_GAndGGJumpToEnds(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("G")
	if got, want := h.currentIndex(), h.rowCount()-1; got != want {
		t.Fatalf("after G: index %d, want %d (last row)", got, want)
	}

	h.typeRunes("gg")
	if got := h.currentIndex(); got != 0 {
		t.Fatalf("after gg: index %d, want 0", got)
	}
}

// Regression: the pending 'g' prefix used to survive any handler that returned
// early (j/k, digits, p, ...), so a later single 'g' was misread as gg and the
// cursor jumped to the top on its own.
func TestNavigation_PendingGDoesNotSurviveAnotherKey(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("jj") // land on tui-bbb, away from the top
	before := h.currentIndex()
	if before == 0 {
		t.Fatalf("test setup: expected to be off the top row, got index 0")
	}

	// g starts a chord; j is a different key and must cancel it.
	h.typeRunes("g")
	h.typeRunes("j")
	h.typeRunes("g")

	if got := h.currentIndex(); got == 0 {
		t.Fatalf("a lone g after an intervening key jumped to the top: the gg prefix is sticky again")
	}

	// Positive control: a real gg still works from here.
	h.typeRunes("g")
	if got := h.currentIndex(); got != 0 {
		t.Fatalf("gg did not jump to the top: index %d", got)
	}
}

// Regression: the same stickiness bug for the s chord. An unrecognized second
// key must cancel the chord rather than leave it armed for the next keypress.
func TestStatusChord_UnknownSecondKeyCancelsTheChord(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j") // select tui-aaa

	h.typeRunes("s") // arm the chord
	if status := h.statusText(); !strings.Contains(status, "Status shortcut") {
		t.Fatalf("status bar did not prompt for the second key: %q", status)
	}

	h.typeRunes("z") // not one of o/i/b/c
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("an unknown second key ran bd anyway: %v", calls)
	}
	if status := h.statusText(); strings.Contains(status, "Status shortcut") {
		t.Fatalf("chord prompt still showing after an unknown second key: %q", status)
	}

	// The chord must be disarmed: 'o' is now the plain collapse binding, not
	// "set status open".
	h.typeRunes("o")
	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("the s chord stayed armed and swallowed the next key: %v", calls)
	}
}

// t toggles between list and tree view, and the list title reports which is up.
func TestViewMode_ToggleBetweenListAndTree(t *testing.T) {
	issues := basicIssues()
	issues = append(issues, newIssue("tui-ddd", "Delta child", withParent("tui-aaa")))
	h := newHarness(t, issues)

	if title := h.listTitle(); !strings.Contains(title, "[List]") {
		t.Fatalf("expected to start in list view, title is %q", title)
	}

	h.typeRunes("t")
	if title := h.listTitle(); !strings.Contains(title, "[Tree]") {
		t.Fatalf("after t: title is %q, want tree view", title)
	}
	text := h.screenText()
	if !strings.Contains(text, "DEPENDENCY TREE") {
		t.Errorf("tree view did not render its header:\n%s", text)
	}
	// Tree rows escape the priority tag, or tview reads "[P1]" as a color name
	// and swallows it (tui-6ajo).
	if !strings.Contains(text, "tui-aaa [P1]") {
		t.Errorf("tree row does not show the priority tag:\n%s", text)
	}

	h.typeRunes("t")
	if title := h.listTitle(); !strings.Contains(title, "[List]") {
		t.Fatalf("after the second t: title is %q, want list view", title)
	}
}

// C reveals closed issues, which are hidden by default.
func TestShowClosed_ToggleAddsClosedIssues(t *testing.T) {
	h := newHarness(t, basicIssues())

	if ids := h.issueIDs(); containsID(ids, "tui-zzz") {
		t.Fatalf("closed issue is visible before pressing C: %v", ids)
	}

	h.typeRunes("C")
	if ids := h.issueIDs(); !containsID(ids, "tui-zzz") {
		t.Fatalf("closed issue still hidden after C: %v", ids)
	}
	if status := h.statusText(); !strings.Contains(status, "Showing Closed") {
		t.Errorf("status bar does not report the closed rows: %q", status)
	}

	h.typeRunes("C")
	if ids := h.issueIDs(); containsID(ids, "tui-zzz") {
		t.Fatalf("closed issue still visible after toggling C off: %v", ids)
	}
}

// v cycles the layout pin, and the status bar marks a pinned layout with a
// trailing asterisk.
//
// The unpinned indicator is deliberately not asserted: tview eats the bare
// "[Columns]" as a color tag so it never reaches the screen (tui-42ws).
func TestLayout_VCyclesThePin(t *testing.T) {
	h := newHarness(t, basicIssues())

	h.typeRunes("v") // pin Columns
	if status := h.statusText(); !strings.Contains(status, layout.Columns.String()+"*") {
		t.Fatalf("after v: expected a pinned Columns layout, got %q", status)
	}

	h.typeRunes("v") // pin Rows
	if status := h.statusText(); !strings.Contains(status, layout.Rows.String()+"*") {
		t.Fatalf("after vv: expected a pinned Rows layout, got %q", status)
	}

	h.typeRunes("v") // pin Paged
	if status := h.statusText(); !strings.Contains(status, layout.Paged.String()+"*") {
		t.Fatalf("after vvv: expected a pinned Paged layout, got %q", status)
	}

	h.typeRunes("v") // back to automatic
	if status := h.statusText(); strings.Contains(status, "*") {
		t.Fatalf("after vvvv: expected the pin to be released, got %q", status)
	}
}

// Tab moves focus to the detail panel and ESC brings it back, without quitting.
func TestFocus_TabIntoDetailAndEscapeBack(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j")

	h.key(tcell.KeyTab)
	if status := h.statusText(); !strings.Contains(status, "Focus: Details") {
		t.Fatalf("Tab did not move focus into the detail panel: %q", status)
	}

	h.key(tcell.KeyEscape)
	if status := h.statusText(); !strings.Contains(status, "Focus: List") {
		t.Fatalf("ESC did not return focus to the list: %q", status)
	}

	// The app must still be alive: one ESC in the list only arms the quit hint.
	h.key(tcell.KeyEscape)
	if status := h.statusText(); !strings.Contains(status, "Press ESC again to quit") {
		t.Fatalf("a single ESC in the list should arm the quit hint, got %q", status)
	}
}

// The focus indicator reaches the screen on a narrow terminal. It used to sit
// behind the absolute .beads path, which filled the line on its own and left
// everything after "(N issues)" clipped at the right edge (tui-7j4u).
//
// 100 columns is a split layout and 80 is the paged one, where the status bar
// is the only signal for which of the two panels is mounted.
func TestStatusBar_NarrowScreenStillShowsFocus(t *testing.T) {
	for _, width := range []int{80, 100} {
		t.Run(fmt.Sprintf("%dcols", width), func(t *testing.T) {
			h := newHarness(t, basicIssues(), withScreenSize(width, 24))

			line := rowContaining(t, h, "Beads TUI")
			if strings.Contains(line, h.beadsDir) {
				t.Errorf("status bar spends the line on the absolute .beads path: %q", line)
			}
			if !strings.Contains(line, "[Focus: List]") {
				t.Errorf("focus indicator did not reach the screen at %d columns: %q", width, line)
			}

			h.key(tcell.KeyTab)
			line = rowContaining(t, h, "Beads TUI")
			if !strings.Contains(line, "[Focus: Details]") {
				t.Errorf("focus indicator did not follow Tab at %d columns: %q", width, line)
			}
		})
	}
}

// The simulation screen actually renders the issue rows, not just the model.
func TestScreen_RendersIssueRows(t *testing.T) {
	h := newHarness(t, basicIssues())

	text := h.screenText()
	for _, want := range []string{"READY (3)", "tui-aaa", "Alpha issue", "Beads TUI"} {
		if !strings.Contains(text, want) {
			t.Errorf("screen is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "tui-zzz") {
		t.Errorf("closed issue rendered without pressing C:\n%s", text)
	}
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}
