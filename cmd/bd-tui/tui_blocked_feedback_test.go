package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// Clearing an explicit blocked status on an issue that still waits on an open
// blocker leaves it in BLOCKED, but the row glyph, the status bar and the
// detail panel all change to say the dependency is now what holds it up.
func TestStatusChord_OpenOnDependencyBlockedIssueNamesTheBlocker(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-blk", "Blocker still open"),
		newIssue("tui-wait", "Waiting on the blocker",
			withStatus(parser.StatusBlocked), withBlockedBy("tui-blk")),
	})
	h.selectIssue("tui-wait")

	if screen := h.screenText(); !strings.Contains(screen, "○ 📋 tui-wait") {
		t.Fatalf("an explicitly blocked row should start as ○:\n%s", screen)
	}

	h.typeRunes("s")
	h.typeRunes("o")

	if status := h.statusText(); !strings.Contains(status, "Set tui-wait to open, still blocked by tui-blk") {
		t.Fatalf("status bar did not name the remaining blocker: %q", status)
	}

	h.waitFor("tui-wait to reload as open", func() bool {
		issue := h.tui.state.GetIssueByID("tui-wait")
		return issue != nil && issue.Status == parser.StatusOpen
	})

	if status := h.statusText(); !strings.Contains(status, "Set tui-wait to open, still blocked by tui-blk") {
		t.Errorf("the reload replaced the blocker note: %q", status)
	}

	screen := h.screenText()
	if strings.Contains(screen, "○ 📋 tui-wait") || !strings.Contains(screen, "◌ 📋 tui-wait") {
		t.Errorf("row glyph did not change from explicit-blocked to dependency-blocked:\n%s", screen)
	}
	if !strings.Contains(screen, "BLOCKED (1)") {
		t.Errorf("an issue with an open blocker left the BLOCKED group:\n%s", screen)
	}
	if !strings.Contains(screen, "blocked by tui-blk (open) Blocker still open") {
		t.Errorf("detail panel does not name the blocker's status and title:\n%s", screen)
	}
}

// An issue parked by hand with nothing blocking it is ready the moment its
// blocked status is cleared.
func TestStatusChord_OpenOnParkedIssueMovesItToReady(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-go", "Already ready"),
		newIssue("tui-park", "Parked by hand", withStatus(parser.StatusBlocked)),
	})
	h.selectIssue("tui-park")

	if screen := h.screenText(); !strings.Contains(screen, "○ 📋 tui-park") {
		t.Fatalf("a parked row should start as ○:\n%s", screen)
	}

	h.typeRunes("s")
	h.typeRunes("o")

	status := h.statusText()
	if !strings.Contains(status, "Set tui-park to open") || strings.Contains(status, "still blocked") {
		t.Fatalf("status bar should confirm the change without a blocker note: %q", status)
	}

	h.waitFor("tui-park to reload as open", func() bool {
		issue := h.tui.state.GetIssueByID("tui-park")
		return issue != nil && issue.Status == parser.StatusOpen
	})

	screen := h.screenText()
	if !strings.Contains(screen, "READY (2)") || strings.Contains(screen, "BLOCKED") {
		t.Errorf("tui-park did not move from BLOCKED to READY:\n%s", screen)
	}
	if !strings.Contains(screen, "● 📋 tui-park") {
		t.Errorf("tui-park should render as ● once ready:\n%s", screen)
	}
}
