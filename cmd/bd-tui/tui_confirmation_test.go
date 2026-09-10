package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// defaultStatusPrefix opens the default status line, so seeing it again means a
// temporary message has gone.
const defaultStatusPrefix = "Beads TUI - "

// parkedBeside is one issue parked by hand next to a ready one, so `so` on the
// parked issue has something to report and the ready one is free to retitle.
func parkedBeside() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-go", "Already ready"),
		newIssue("tui-park", "Parked by hand", withStatus(parser.StatusBlocked)),
	}
}

// refreshAfterExternalWrite retitles id in issues.jsonl the way another process
// would, triggers the refresh the watcher would, and returns once it has loaded.
// The new title is the evidence: nothing else puts it in the state.
func refreshAfterExternalWrite(h *harness, id, title string) {
	h.t.Helper()
	issue := h.findIssue(id)
	if issue == nil {
		h.t.Fatalf("no fixture issue %q", id)
	}
	issue.Title = title
	h.writeJSONL()
	h.tui.refreshIssues()
	h.waitFor(id+" to reload as "+title, func() bool {
		loaded := h.tui.state.GetIssueByID(id)
		return loaded != nil && loaded.Title == title
	})
}

// A command's confirmation outlives the reload the command schedules and a
// refresh provoked by somebody else's write, then expires to the default line.
func TestConfirmation_OutlivesTheReloadsThenExpires(t *testing.T) {
	h := newHarness(t, parkedBeside())
	h.selectIssue("tui-park")

	h.typeRunes("so")
	waitForStatus(h, "tui-park", parser.StatusOpen)
	if status := h.statusText(); !strings.Contains(status, "✓ Set tui-park to open") {
		t.Fatalf("the reload replaced the confirmation: %q", status)
	}

	refreshAfterExternalWrite(h, "tui-go", "Retitled elsewhere")
	if status := h.statusText(); !strings.Contains(status, "✓ Set tui-park to open") {
		t.Fatalf("a refresh from an unrelated write replaced the confirmation: %q", status)
	}

	h.waitFor("the confirmation to expire", func() bool {
		return strings.HasPrefix(h.tui.statusBar.GetText(true), defaultStatusPrefix)
	})
}

// The watcher's refresh usually lands before the scheduled one, so the
// confirmation has to survive that one too.
func TestConfirmation_SurvivesTheWatcherRefresh(t *testing.T) {
	h := newHarness(t, parkedBeside(), withWatcher())
	h.selectIssue("tui-park")

	h.typeRunes("so")
	waitForStatus(h, "tui-park", parser.StatusOpen)
	if status := h.statusText(); !strings.Contains(status, "✓ Set tui-park to open") {
		t.Errorf("the watcher's refresh replaced the confirmation: %q", status)
	}
}

// Any other write to the status bar ends the confirmation early, and a refresh
// afterwards must not bring it back.
func TestConfirmation_IsNotRestoredOverALaterStatusLine(t *testing.T) {
	h := newHarness(t, parkedBeside())
	h.selectIssue("tui-park")

	h.typeRunes("so")
	waitForStatus(h, "tui-park", parser.StatusOpen)

	h.typeRunes("C") // repaints the default line, now with [Showing Closed]
	refreshAfterExternalWrite(h, "tui-go", "Retitled elsewhere")

	if status := h.statusText(); !strings.HasPrefix(status, defaultStatusPrefix) {
		t.Errorf("a refresh put a superseded confirmation back: %q", status)
	}
}
