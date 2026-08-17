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

// t toggles between list and tree view, and the list border reports which is
// up. The mode indicator is read off the screen: as the title text it was
// "Issues [List]", but tview ate the tag and the border said only "Issues"
// (tui-42ws).
func TestViewMode_ToggleBetweenListAndTree(t *testing.T) {
	issues := basicIssues()
	issues = append(issues, newIssue("tui-ddd", "Delta child", withParent("tui-aaa")))
	h := newHarness(t, issues)

	if text := h.screenText(); !strings.Contains(text, "Issues [List]") {
		t.Fatalf("expected to start in list view, screen is:\n%s", text)
	}

	h.typeRunes("t")
	text := h.screenText()
	if !strings.Contains(text, "Issues [Tree]") {
		t.Fatalf("after t: the border does not report tree view:\n%s", text)
	}
	if !strings.Contains(text, "DEPENDENCY TREE") {
		t.Errorf("tree view did not render its header:\n%s", text)
	}
	// Tree rows escape the priority tag, or tview reads "[P1]" as a color name
	// and swallows it (tui-6ajo).
	if !strings.Contains(text, "tui-aaa [P1]") {
		t.Errorf("tree row does not show the priority tag:\n%s", text)
	}

	h.typeRunes("t")
	if text := h.screenText(); !strings.Contains(text, "Issues [List]") {
		t.Fatalf("after the second t: the border does not report list view:\n%s", text)
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

// v cycles the layout pin. The status bar names the layout in both states:
// bare while it is resolved from the terminal width, with a trailing asterisk
// once pinned.
//
// Every assertion reads the rendered screen rather than statusBar.GetText.
// The unpinned indicator was invisible for months (tui-42ws) while the text
// handed to SetText was correct the whole time: tview parsed "[Columns]" as a
// color tag and swallowed it. Only "[Columns*]" survived, because the asterisk
// made the tag invalid, which is exactly the asymmetry this test now pins down.
func TestLayout_VCyclesThePin(t *testing.T) {
	h := newHarness(t, basicIssues())

	// The harness screen is 120 columns, which resolves to Columns, and nothing
	// is pinned yet.
	if line := rowContaining(t, h, "Beads TUI"); !strings.Contains(line, "["+layout.Columns.String()+"]") {
		t.Fatalf("the automatic layout indicator did not reach the screen: %q", line)
	}

	for _, want := range []layout.Mode{layout.Columns, layout.Rows, layout.Paged} {
		h.typeRunes("v")
		line := rowContaining(t, h, "Beads TUI")
		if !strings.Contains(line, "["+want.String()+"*]") {
			t.Fatalf("expected a pinned %s layout on screen, got %q", want, line)
		}
	}

	h.typeRunes("v") // back to automatic
	line := rowContaining(t, h, "Beads TUI")
	if strings.Contains(line, "*") {
		t.Fatalf("after vvvv: expected the pin to be released, got %q", line)
	}
	if !strings.Contains(line, "["+layout.Columns.String()+"]") {
		t.Fatalf("after vvvv: expected the resolved layout on screen, got %q", line)
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

// Left arrow is the "go back" gesture out of the detail panel, same as ESC.
// The paged case is the one that matters: below layout.NarrowThreshold only
// one panel is mounted, so the list is off screen entirely until focus returns.
func TestFocus_LeftArrowReturnsToList(t *testing.T) {
	for _, width := range []int{120, 80} {
		t.Run(fmt.Sprintf("%dcols", width), func(t *testing.T) {
			h := newHarness(t, basicIssues(), withScreenSize(width, 24))
			h.typeRunes("j")

			h.key(tcell.KeyTab)
			if status := h.statusText(); !strings.Contains(status, "Focus: Details") {
				t.Fatalf("Tab did not move focus into the detail panel: %q", status)
			}

			h.key(tcell.KeyLeft)
			if status := h.statusText(); !strings.Contains(status, "Focus: List") {
				t.Fatalf("Left did not return focus to the list: %q", status)
			}
			if !strings.Contains(h.screenText(), "READY (3)") {
				t.Fatalf("the issue list is not the mounted page after Left:\n%s", h.screenText())
			}

			// Focus really is on the list, not merely reported there: j moves the
			// selection instead of being swallowed by the detail panel.
			before := h.currentIssueID()
			h.typeRunes("j")
			if after := h.currentIssueID(); after == before {
				t.Fatalf("j did not move the list selection after Left (still %q)", after)
			}
		})
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
