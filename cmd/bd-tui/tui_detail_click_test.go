package main

import (
	"strings"
	"testing"
)

// Clicking the ID line in the detail panel copies the ID. A title long enough
// to wrap pushes the ID line down a row, and the click has to follow it.
func TestDetailClick_IDLineCopiesID(t *testing.T) {
	for _, tc := range []struct{ name, title string }{
		{"short title", "Bravo issue"},
		{"wrapped title", strings.Repeat("a title long enough to wrap ", 6)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issues := basicIssues()
			issues[1].Title = tc.title
			h := newHarness(t, issues)
			h.selectIssue("tui-bbb")

			h.clickText("click to copy")

			if got := h.copied(); len(got) != 1 || got[0] != "tui-bbb" {
				t.Fatalf("clicking the ID line copied %q, want [tui-bbb]", got)
			}
			if status := h.statusText(); !strings.Contains(status, "Copied tui-bbb") {
				t.Errorf("status bar does not report the copy: %q", status)
			}
		})
	}
}
