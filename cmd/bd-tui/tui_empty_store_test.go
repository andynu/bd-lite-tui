package main

import (
	"strings"
	"testing"
)

// A .beads directory with no issues.jsonl is the state of a repo where bd has
// been initialized but never written to. bd-lite reads it as an empty store, so
// the TUI opens an empty board rather than refusing to start.
func TestEmptyStore_OpensWithCreateHint(t *testing.T) {
	h := newHarness(t, basicIssues(), withoutStoreFile())

	if ids := h.issueIDs(); len(ids) != 0 {
		t.Fatalf("expected no issue rows against a missing store, got %v", ids)
	}

	screen := h.screenText()
	for _, want := range []string{"No issues found", "Press 'a' to create an issue"} {
		if !strings.Contains(screen, want) {
			t.Errorf("empty board does not show %q:\n%s", want, screen)
		}
	}
}

// The watcher registers the .beads directory rather than the file inode, so the
// first `bd create` (which is what creates issues.jsonl) arrives as an ordinary
// change event and the board fills in without a restart.
func TestEmptyStore_WatcherPicksUpFirstWrite(t *testing.T) {
	h := newHarness(t, basicIssues(), withoutStoreFile(), withWatcher())

	if ids := h.issueIDs(); len(ids) != 0 {
		t.Fatalf("expected no issue rows before the store exists, got %v", ids)
	}

	h.writeJSONL()

	h.waitFor("the created store to be loaded", func() bool {
		for i := 0; i < h.tui.issueList.GetItemCount(); i++ {
			if issue, ok := h.tui.indexToIssue[i]; ok && issue.ID == "tui-aaa" {
				return true
			}
		}
		return false
	})
}

// Filtering to a single issue is still an error when the store has no issues at
// all: --issue names something that must exist, and an empty board would look
// like the issue was found and had nothing to show.
func TestEmptyStore_IssueFilterStillFails(t *testing.T) {
	beadsDir := t.TempDir()

	_, err := newTUI(tuiOptions{
		BeadsDir:  beadsDir,
		JSONLPath: beadsDir + "/issues.jsonl",
		IssueID:   "tui-aaa",
		NoWatcher: true,
	})
	if err == nil {
		t.Fatal("expected --issue against a missing store to fail")
	}
	if !strings.Contains(err.Error(), "tui-aaa") {
		t.Errorf("error does not name the missing issue: %v", err)
	}
}
