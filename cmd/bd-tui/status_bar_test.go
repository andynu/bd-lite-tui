package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStatusLocation(t *testing.T) {
	const home = "/home/andy"

	cases := []struct {
		name     string
		beadsDir string
		want     string
	}{
		{
			name:     "a path under home renders with a tilde",
			beadsDir: "/home/andy/projects/bd-lite-tui/.beads",
			want:     "~/projects/bd-lite-tui",
		},
		{
			name:     "a trailing separator does not change the answer",
			beadsDir: "/home/andy/projects/bd-lite-tui/.beads/",
			want:     "~/projects/bd-lite-tui",
		},
		{
			name:     "outside home the absolute head is kept",
			beadsDir: "/srv/beads/.beads",
			want:     "/srv/beads",
		},
		{
			name:     "a repo at the top of home is just the tilde",
			beadsDir: "/home/andy/.beads",
			want:     "~",
		},
		{
			name:     "a directory not named .beads is labeled as itself",
			beadsDir: "/home/andy/work/beads-data",
			want:     "~/work/beads-data",
		},
		{
			name:     "a long path keeps whole trailing elements",
			beadsDir: "/home/andy/work/src/bd-lite-tui-statusbar/.beads",
			want:     "…/bd-lite-tui-statusbar",
		},
		{
			name:     "a long path under a short tail keeps as much as fits",
			beadsDir: "/tmp/TestStatusBarNarrowScreen1234/001/.beads",
			want:     "…/001",
		},
		{
			name:     "an element wider than the budget keeps its tail",
			beadsDir: "/var/a-single-directory-name-far-longer-than-the-budget/.beads",
			want:     "…-longer-than-the-budget",
		},
		{
			name:     "a bare relative .beads names itself",
			beadsDir: ".beads",
			want:     ".beads",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusLocation(tc.beadsDir, home)
			if got != tc.want {
				t.Errorf("statusLocation(%q) = %q, want %q", tc.beadsDir, got, tc.want)
			}
			if n := utf8.RuneCountInString(got); n > statusLocationWidth {
				t.Errorf("statusLocation(%q) is %d columns wide, over the %d budget: %q",
					tc.beadsDir, n, statusLocationWidth, got)
			}
		})
	}
}

// Whatever the path, the label never carries the whole .beads directory, which
// is what used to consume the line (tui-7j4u).
func TestStatusLocationNeverRendersTheWholeBeadsPath(t *testing.T) {
	dirs := []string{
		"/home/andy/projects/bd-lite-tui/.beads",
		"/home/andy/work/src/some/deeply/nested/checkout/.beads",
		"/tmp/TestSomethingWithAVeryLongName999/001/.beads",
	}
	for _, dir := range dirs {
		if got := statusLocation(dir, "/home/andy"); strings.Contains(got, dir) {
			t.Errorf("statusLocation(%q) still spells out the whole path: %q", dir, got)
		}
	}
}

func TestFormatStatusBarOrdersActionableFieldsFirst(t *testing.T) {
	line := formatStatusBar(statusBarFields{
		EmphasisColor: "yellow",
		Location:      "~/projects/bd-lite-tui",
		IssueCount:    3,
		Focus:         "List",
		Layout:        "Paged",
		MouseEnabled:  true,
	})

	// Everything the user acts on has to precede the decoration, because a
	// narrow terminal clips the tail. The layout field carries tview's "[]"
	// escape, so it reads as "[Paged[]" here and as "[Paged]" on screen; see
	// TestLayout_VCyclesThePin for the rendered assertion.
	order := []string{"Beads TUI", "(3 issues)", "[Focus: List]", "[Paged[]", "[Mouse: ON]", "[? help | v layout]"}
	prev := -1
	for _, want := range order {
		at := strings.Index(line, want)
		if at < 0 {
			t.Fatalf("status line is missing %q: %q", want, line)
		}
		if at < prev {
			t.Errorf("%q renders out of order in %q", want, line)
		}
		prev = at
	}
}

func TestFormatStatusBarOmitsInactiveFields(t *testing.T) {
	line := formatStatusBar(statusBarFields{Focus: "List", Layout: "Columns"})
	if strings.Contains(line, "Filters") {
		t.Errorf("unfiltered status line advertises a filter: %q", line)
	}
	if strings.Contains(line, "Showing Closed") {
		t.Errorf("status line claims closed issues are shown: %q", line)
	}
	if !strings.Contains(line, "[Mouse: OFF]") {
		t.Errorf("status line does not report mouse mode off: %q", line)
	}
}

func TestFormatStatusBarReportsFiltersBeforeMouseMode(t *testing.T) {
	line := formatStatusBar(statusBarFields{
		Focus:         "Details",
		Layout:        "Rows*",
		Filters:       "Priority: P1",
		ShowingClosed: true,
		MouseEnabled:  true,
	})

	filters := strings.Index(line, "[Filters: Priority: P1]")
	closed := strings.Index(line, "[Showing Closed]")
	mouse := strings.Index(line, "[Mouse: ON]")
	if filters < 0 || closed < 0 {
		t.Fatalf("status line is missing the filter fields: %q", line)
	}
	if filters > mouse || closed > mouse {
		t.Errorf("fields that explain missing rows render after mouse mode: %q", line)
	}
}

// A bd outside the supported range is reported right after the issue count,
// ahead of everything a narrow terminal might clip.
func TestFormatStatusBarWarnsAboutBdBeforeFocus(t *testing.T) {
	line := formatStatusBar(statusBarFields{
		IssueCount: 3,
		Focus:      "List",
		Layout:     "Columns",
		BdWarning:  "v0.2.0 untested, bd-tui knows v0.1",
	})
	count := strings.Index(line, "(3 issues)")
	bd := strings.Index(line, "bd: v0.2.0 untested, bd-tui knows v0.1")
	focus := strings.Index(line, "[Focus: List]")
	if bd < 0 {
		t.Fatalf("status line is missing the bd warning: %q", line)
	}
	if bd < count || bd > focus {
		t.Errorf("bd warning should sit between the count and focus: %q", line)
	}
}

func TestFormatStatusBarSilentWhenBdIsFine(t *testing.T) {
	line := formatStatusBar(statusBarFields{Focus: "List", Layout: "Columns"})
	if strings.Contains(line, "bd:") {
		t.Errorf("status line warns about bd with nothing to say: %q", line)
	}
}
