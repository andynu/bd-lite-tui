package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// A priority digit runs one bd update, reports it, and the scheduled reload
// brings the new value back from disk without losing the selection.
func TestPriorityKey_UpdatesReportsAndReloads(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j") // tui-aaa, P1

	h.typeRunes("3")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-aaa", "--priority", "3"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); !strings.Contains(status, "Set tui-aaa to P3") {
		t.Fatalf("status bar did not confirm the change: %q", status)
	}

	// The update is only visible once the scheduled refresh reloads the file.
	h.waitFor("the reload to pick up P3", func() bool {
		issue := h.tui.state.GetIssueByID("tui-aaa")
		return issue != nil && issue.Priority == 3
	})
	if got := h.currentIssueID(); got != "tui-aaa" {
		t.Errorf("the reload moved the selection to %q", got)
	}
	if detail := h.detailText(); !strings.Contains(detail, "P3") {
		t.Errorf("the detail panel still shows the old priority:\n%s", detail)
	}
	// The row's own priority tag has to survive tview's tag parser, which reads a
	// bare "[P3]" as a color name and swallows it (tui-6ajo). Assert on rendered
	// output, since that is the thing that regressed. The reload's state update
	// and redraw share one QueueUpdateDraw, so the wait above already covers it.
	if text := h.screenText(); !strings.Contains(text, "tui-aaa [P3]") {
		t.Errorf("the row does not show the priority tag:\n%s", text)
	}
}

// A priority digit on a group header must not run bd at all.
func TestPriorityKey_IgnoredOnAGroupHeader(t *testing.T) {
	h := newHarness(t, basicIssues())

	if id := h.currentIssueID(); id != "" {
		t.Fatalf("test setup: expected to start on a header, got %q", id)
	}
	h.typeRunes("0")

	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("bd ran for a header row: %v", calls)
	}
}

// A failing bd command surfaces its message in the status bar and schedules no
// reload.
func TestPriorityKey_ErrorSurfacesInTheStatusBar(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.setBdResponse(func([]string) (*BdCommandResult, error) {
		return nil, errors.New("bd update failed: database is locked")
	})

	h.typeRunes("j")
	h.typeRunes("2")

	status := h.statusText()
	if !strings.Contains(status, "Error updating priority") {
		t.Fatalf("status bar did not report the failure: %q", status)
	}
	if !strings.Contains(status, "database is locked") {
		t.Errorf("status bar dropped bd's own diagnostic: %q", status)
	}
}

// s followed by i sets in_progress, and the reload re-files the issue under the
// IN PROGRESS group while the cursor stays behind in READY.
func TestStatusChord_SetsInProgressAndRegroups(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("j") // tui-aaa

	h.typeRunes("s")
	h.typeRunes("i")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-aaa", "--status", "in_progress"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); !strings.Contains(status, "Set tui-aaa to in_progress") {
		t.Fatalf("status bar did not confirm the change: %q", status)
	}

	h.waitFor("tui-aaa to move to the in-progress group", func() bool {
		issue := h.tui.state.GetIssueByID("tui-aaa")
		return issue != nil && issue.Status == parser.StatusInProgress
	})
	h.waitFor("the in-progress header to render", func() bool {
		for i := 0; i < h.tui.issueList.GetItemCount(); i++ {
			text, _ := h.tui.issueList.GetItemText(i)
			if strings.Contains(text, "IN PROGRESS (1)") {
				return true
			}
		}
		return false
	})
	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Errorf("cursor is on %q, want tui-bbb, the row after tui-aaa in READY", got)
	}
}

// Every second key of the chord maps to the status bd expects.
func TestStatusChord_MapsEverySecondKey(t *testing.T) {
	for _, tc := range []struct {
		key    string
		status string
	}{
		{"o", "open"},
		{"i", "in_progress"},
		{"b", "blocked"},
		{"c", "closed"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			h := newHarness(t, basicIssues())
			h.typeRunes("j")
			h.typeRunes("s")
			h.typeRunes(tc.key)

			calls := h.calls()
			if len(calls) != 1 {
				t.Fatalf("expected one bd invocation, got %v", calls)
			}
			if want := []string{"update", "tui-aaa", "--status", tc.status}; !reflect.DeepEqual(calls[0].Args, want) {
				t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
			}
		})
	}
}

// r reloads from disk, picking up an issue another process appended.
func TestManualRefresh_PicksUpAnExternalWrite(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("jj") // tui-bbb

	h.issues = append(h.issues, newIssue("tui-new", "Newly filed issue"))
	h.writeJSONL()

	h.typeRunes("r")

	h.waitFor("the new issue to appear", func() bool {
		return h.tui.state.GetIssueByID("tui-new") != nil
	})
	if ids := h.issueIDs(); !containsID(ids, "tui-new") {
		t.Fatalf("the new issue is not in the list: %v", ids)
	}
	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Errorf("the refresh moved the selection to %q, want tui-bbb", got)
	}
}

// A refresh whose selected issue has disappeared keeps the cursor in range
// instead of leaving it pointing past the end of the list.
func TestManualRefresh_SelectedIssueRemovedKeepsCursorInRange(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.typeRunes("jjj") // tui-ccc, the last ready row

	h.issues = h.issues[:len(h.issues)-2] // drop tui-ccc and the closed one
	h.writeJSONL()
	h.typeRunes("r")

	h.waitFor("tui-ccc to disappear", func() bool {
		return h.tui.state.GetIssueByID("tui-ccc") == nil
	})

	idx, count := h.currentIndex(), h.rowCount()
	if idx < 0 || idx >= count {
		t.Fatalf("selection %d is outside the %d rows left after the refresh", idx, count)
	}
}

// The filesystem watcher reloads the list when another process rewrites
// issues.jsonl, without a keypress.
func TestWatcher_ReloadsOnAnExternalWrite(t *testing.T) {
	h := newHarness(t, basicIssues(), withWatcher())
	h.typeRunes("jj") // tui-bbb

	h.issues = append(h.issues, newIssue("tui-watch", "Written by another process"))
	h.writeJSONL()

	h.waitFor("the watcher to reload the file", func() bool {
		return h.tui.state.GetIssueByID("tui-watch") != nil
	})
	if got := h.currentIssueID(); got != "tui-bbb" {
		t.Errorf("the watcher refresh moved the selection to %q, want tui-bbb", got)
	}
}

// A burst of refresh requests coalesces, and whatever is on disk when the dust
// settles is what ends up on screen. This drives the real single-flight
// dispatcher; TestSingleFlight_* covers the lost-wakeup window that only a
// hook in a replica of the algorithm can hit deterministically.
func TestRefresh_BurstCoalescesAndTheLastWriteWins(t *testing.T) {
	h := newHarness(t, basicIssues())

	const writes = 50
	for i := 0; i < writes; i++ {
		h.issues = append(h.issues, newIssue(fmt.Sprintf("tui-b%02d", i), "Burst issue"))
		h.writeJSONL()
		h.tui.refreshIssues()
	}

	last := fmt.Sprintf("tui-b%02d", writes-1)
	h.waitFor("the final write to reach the list", func() bool {
		return h.tui.state.GetIssueByID(last) != nil
	})
	want := len(basicIssues()) - 1 + writes // the fixture's closed issue stays hidden
	if ids := h.issueIDs(); len(ids) != want {
		t.Fatalf("the coalesced refresh settled on %d rows, want %d", len(ids), want)
	}
}

// Regression: --issue restricts the view to one issue, and that restriction has
// to survive every later reload rather than quietly reverting to the full set.
func TestIssueFilter_SurvivesARefresh(t *testing.T) {
	h := newHarness(t, basicIssues(), withIssueFilter("tui-bbb"))

	if ids := h.issueIDs(); !reflect.DeepEqual(ids, []string{"tui-bbb"}) {
		t.Fatalf("--issue did not restrict the initial view: %v", ids)
	}

	h.issues = append(h.issues, newIssue("tui-new", "Newly filed issue"))
	h.writeJSONL()
	h.typeRunes("r")

	// Give the refresh a chance to (wrongly) widen the list.
	h.waitFor("the refresh to run", func() bool {
		return h.tui.state.GetIssueByID("tui-bbb") != nil
	})
	if ids := h.issueIDs(); !reflect.DeepEqual(ids, []string{"tui-bbb"}) {
		t.Fatalf("the refresh discarded the --issue filter: %v", ids)
	}
}
