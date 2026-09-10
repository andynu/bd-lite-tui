package main

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
)

// parkedPair is a BLOCKED group of two issues parked by hand. Nothing blocks
// either, so `so` moves each one up into READY.
func parkedPair() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-park1", "First parked", withStatus(parser.StatusBlocked)),
		newIssue("tui-park2", "Second parked", withStatus(parser.StatusBlocked)),
	}
}

// waitForStatus waits for the reload that shows id at status. The selection is
// restored in the same event-loop pass that loads the state, so the cursor can
// be read as soon as this returns.
func waitForStatus(h *harness, id string, status parser.Status) {
	h.t.Helper()
	h.waitFor(id+" to reload as "+string(status), func() bool {
		issue := h.tui.state.GetIssueByID(id)
		return issue != nil && issue.Status == status
	})
}

// Unblocking the first of two parked issues moves it to READY, and the second
// comes under the cursor, ready for the next `so`.
func TestStatusChord_LeavingAGroupLeavesTheCursorInIt(t *testing.T) {
	h := newHarness(t, parkedPair())
	if got := strings.Join(h.issueIDs(), ","); got != "tui-park1,tui-park2" {
		t.Fatalf("fixture rows are %s, want tui-park1,tui-park2", got)
	}
	h.selectIssue("tui-park1")

	h.typeRunes("so")
	waitForStatus(h, "tui-park1", parser.StatusOpen)

	if got := h.currentIssueID(); got != "tui-park2" {
		t.Errorf("cursor is on %q, want tui-park2, the next row in BLOCKED", got)
	}
	h.press()
	if screen := h.screenText(); !strings.Contains(screen, "READY (1)") || !strings.Contains(screen, "BLOCKED (1)") {
		t.Errorf("tui-park1 did not move from BLOCKED to READY:\n%s", screen)
	}
}

// The last row of a group has no next row, so the cursor takes the one above.
func TestStatusChord_LeavingAsTheLastRowStepsBack(t *testing.T) {
	h := newHarness(t, parkedPair())
	h.selectIssue("tui-park2")

	h.typeRunes("so")
	waitForStatus(h, "tui-park2", parser.StatusOpen)

	if got := h.currentIssueID(); got != "tui-park1" {
		t.Errorf("cursor is on %q, want tui-park1, the row before it in BLOCKED", got)
	}
}

// With nothing left behind in the group, the cursor follows the row.
func TestStatusChord_LeavingAnEmptiedGroupFollowsTheRow(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-go", "Already ready"),
		newIssue("tui-park", "Parked by hand", withStatus(parser.StatusBlocked)),
	})
	h.selectIssue("tui-park")

	h.typeRunes("so")
	waitForStatus(h, "tui-park", parser.StatusOpen)

	if got := h.currentIssueID(); got != "tui-park" {
		t.Errorf("cursor is on %q, want tui-park followed into READY", got)
	}
}

// Clearing the status of an issue that still waits on an open blocker leaves
// its row in BLOCKED, and the cursor stays on that row rather than stepping.
func TestStatusChord_StayingInTheGroupFollowsTheRow(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-blk", "Blocker still open"),
		newIssue("tui-wait1", "First waiting", withStatus(parser.StatusBlocked), withBlockedBy("tui-blk")),
		newIssue("tui-wait2", "Second waiting", withStatus(parser.StatusBlocked), withBlockedBy("tui-blk")),
	})
	h.selectIssue("tui-wait1")

	h.typeRunes("so")
	waitForStatus(h, "tui-wait1", parser.StatusOpen)

	if got := h.currentIssueID(); got != "tui-wait1" {
		t.Errorf("cursor is on %q, want tui-wait1, whose row stayed in BLOCKED", got)
	}
}

// The watcher's refresh usually lands before the scheduled one. It has to take
// the step itself, or the cursor rides the row into READY until the scheduled
// refresh drags it back.
func TestStatusChord_WatcherRefreshTakesTheStep(t *testing.T) {
	h := newHarness(t, parkedPair(), withWatcher())
	h.selectIssue("tui-park1")

	h.typeRunes("so")
	waitForStatus(h, "tui-park1", parser.StatusOpen)

	if got := h.currentIssueID(); got != "tui-park2" {
		t.Errorf("after the first reload the cursor is on %q, want tui-park2", got)
	}
}

// olderPair puts two issues in the Updated view's OLDER bucket, newest first.
// Any write restamps updated_at, which moves the row into TODAY.
func olderPair() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-old1", "Touched in January", withUpdatedAt(time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC))),
		newIssue("tui-old2", "Touched earlier in January", withUpdatedAt(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))),
	}
}

// Groups are whatever the view draws headers for, so in the Updated view a
// status change that moves a row into TODAY leaves the cursor in its old bucket.
func TestStatusChord_UpdatedViewStepsWithinTheBucket(t *testing.T) {
	h := newHarness(t, olderPair(), withView(state.ViewUpdated), withNow(updatedViewNow))
	h.selectIssue("tui-old1")

	h.typeRunes("si")
	waitForStatus(h, "tui-old1", parser.StatusInProgress)

	if got := h.currentIssueID(); got != "tui-old2" {
		t.Errorf("cursor is on %q, want tui-old2, the next row in OLDER", got)
	}
}

// Priority changes are not status changes: the cursor follows the row even
// when the restamp carries it into another bucket.
func TestPriorityChange_FollowsTheRowOutOfItsGroup(t *testing.T) {
	h := newHarness(t, olderPair(), withView(state.ViewUpdated), withNow(updatedViewNow))
	h.selectIssue("tui-old1")

	h.typeRunes("1")
	h.waitFor("tui-old1 to reload at P1", func() bool {
		issue := h.tui.state.GetIssueByID("tui-old1")
		return issue != nil && issue.Priority == 1
	})

	if got := h.currentIssueID(); got != "tui-old1" {
		t.Errorf("cursor is on %q, want tui-old1 followed into TODAY", got)
	}
}
