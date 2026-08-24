package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/state"
)

// The view modes are described in four places that have drifted apart before:
// state.viewModes (the code), the `?` help screen, README.md, and CLAUDE.md.
// These tests fail when a mode is added to the code and not to the prose, which
// is the failure mode tui-mb4u was filed for.

// readDoc returns the contents of a doc file at the repository root. The tests
// run from cmd/bd-tui, so the root is two levels up.
func readDoc(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}

func TestHelpScrollPosition(t *testing.T) {
	cases := []struct {
		name                   string
		offset, visible, total int
		want                   string
	}{
		{"the whole document fits", 0, 40, 30, "All"},
		{"exactly fits", 0, 30, 30, "All"},
		{"unmeasured body", 0, 0, 200, "All"},
		{"at the top", 0, 20, 200, "Top"},
		{"one line down", 1, 20, 200, "0%"},
		{"halfway", 90, 20, 200, "50%"},
		{"last scrollable line", 180, 20, 200, "Bot"},
		{"offset past the end, before tview clamps it", 400, 20, 200, "Bot"},
		{"negative offset", -5, 20, 200, "Top"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := helpScrollPosition(tc.offset, tc.visible, tc.total); got != tc.want {
				t.Errorf("helpScrollPosition(%d, %d, %d) = %q, want %q",
					tc.offset, tc.visible, tc.total, got, tc.want)
			}
		})
	}
}

func TestHelpFooterNamesTheScrollAndCloseKeys(t *testing.T) {
	got := helpFooter(0, 20, 200)
	for _, want := range []string{"Top", "↑/↓", "PgUp/PgDn", "scroll", "ESC or ? closes"} {
		if !strings.Contains(got, want) {
			t.Errorf("help footer %q is missing %q", got, want)
		}
	}
}

func TestHelpScreenNamesEveryViewMode(t *testing.T) {
	for _, name := range state.ViewModeNames() {
		if !strings.Contains(helpText, name) {
			t.Errorf("help screen does not mention view mode %q; update the View Controls section of dialog_help.go", name)
		}
	}
}

func TestHelpScreenShowsTheViewCycleAndTheViewFlag(t *testing.T) {
	// The cycle order and the flag spelling are what a user acts on, so assert
	// on the rendered order rather than on mere presence of the words.
	cycle := strings.Join(state.ViewModeNames(), " → ")
	if !strings.Contains(helpText, cycle) {
		t.Errorf("help screen does not show the cycle %q", cycle)
	}
	if !strings.Contains(helpText, "--view") {
		t.Error("help screen does not document the --view flag")
	}
}

func TestDocsNameEveryViewMode(t *testing.T) {
	for _, doc := range []string{"README.md", "CLAUDE.md"} {
		content := readDoc(t, doc)
		for _, name := range state.ViewModeNames() {
			if !strings.Contains(content, name) {
				t.Errorf("%s does not mention view mode %q", doc, name)
			}
		}
		if !strings.Contains(content, "--view") {
			t.Errorf("%s does not document the --view flag", doc)
		}
	}
}

func TestDocsShowTheViewCycle(t *testing.T) {
	cycle := strings.Join(state.ViewModeNames(), " → ")
	for _, doc := range []string{"README.md", "CLAUDE.md"} {
		if !strings.Contains(readDoc(t, doc), cycle) {
			t.Errorf("%s does not show the view cycle %q", doc, cycle)
		}
	}
}

func TestDocsNameEveryTimeBucket(t *testing.T) {
	// The created and updated views are only useful if the reader knows which
	// buckets exist, so every label has to appear in the help screen and in the
	// README. CLAUDE.md is excluded: it documents the buckets by pointing at
	// internal/timegroup rather than listing them in the view-mode section.
	buckets := []string{"Today", "Yesterday", "This week", "Last week", "This month", "Older", "Undated"}
	sources := map[string]string{
		"help screen": helpText,
		"README.md":   readDoc(t, "README.md"),
	}
	for name, content := range sources {
		for _, bucket := range buckets {
			if !strings.Contains(content, bucket) {
				t.Errorf("%s does not name the %q time bucket", name, bucket)
			}
		}
	}
}
