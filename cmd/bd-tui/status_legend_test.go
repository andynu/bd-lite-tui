package main

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
)

// The status glyphs are listed in four legends: the help screen's Status Icons
// and Status Colors sections, and README's Visual Design line and Status
// Indicators section. These tests read the glyphs off rendered rows rather than
// off a constant, because the list view passes some glyphs as literals and
// the time-keyed views ask issueStatus, and a legend has to match both.

// tviewTag matches a tview color or style tag such as [limegreen] or [-::-].
var tviewTag = regexp.MustCompile(`\[[^\[\]]*\]`)

// glyphIssues holds one issue for each glyph a flat-view row can draw.
func glyphIssues() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-prg", "In progress", withStatus(parser.StatusInProgress)),
		newIssue("tui-rdy", "Ready"),
		newIssue("tui-hld", "Blocked by status", withStatus(parser.StatusBlocked)),
		newIssue("tui-dep", "Blocked by a dependency", withBlockedBy("tui-rdy")),
		newIssue("tui-cls", "Closed", withStatus(parser.StatusClosed)),
	}
}

// drawnGlyphs returns the glyph each issue row opens with, keyed by issue ID.
func drawnGlyphs(h *harness) map[string]string {
	glyphs := map[string]string{}
	h.read(func() {
		for i := 0; i < h.tui.issueList.GetItemCount(); i++ {
			issue, ok := h.tui.indexToIssue[i]
			if !ok {
				continue
			}
			text, _ := h.tui.issueList.GetItemText(i)
			if fields := strings.Fields(tviewTag.ReplaceAllString(text, "")); len(fields) > 0 {
				glyphs[issue.ID] = fields[0]
			}
		}
	})
	return glyphs
}

// linesUnder returns the lines after heading in text, skipping blank lines
// directly below it and stopping at the next blank line.
func linesUnder(t *testing.T, text, heading string) []string {
	t.Helper()
	_, after, ok := strings.Cut(text, heading+"\n")
	if !ok {
		t.Fatalf("no %q heading", heading)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimLeft(after, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		lines = append(lines, line)
	}
	return lines
}

// fieldOfEach returns field n of each line once tview tags are stripped.
func fieldOfEach(lines []string, n int) []string {
	var out []string
	for _, line := range lines {
		fields := strings.Fields(tviewTag.ReplaceAllString(line, ""))
		if len(fields) > n {
			out = append(out, fields[n])
		}
	}
	return out
}

// statusLegends returns the glyphs each legend lists, keyed by where it lives.
func statusLegends(t *testing.T) map[string][]string {
	t.Helper()
	readme := readDoc(t, "README.md")

	_, visual, ok := strings.Cut(readme, "**Status icons** - ")
	if !ok {
		t.Fatal("README.md has no **Status icons** line")
	}
	visual, _, _ = strings.Cut(visual, "\n")

	return map[string][]string{
		"help Status Icons":        fieldOfEach(linesUnder(t, helpText, "[cyan::b]Status Icons[-::-]"), 0),
		"help Status Colors":       fieldOfEach(linesUnder(t, helpText, "[cyan::b]Status Colors[-::-]"), 0),
		"README Visual Design":     fieldOfEach(strings.Split(visual, ", "), 0),
		"README Status Indicators": fieldOfEach(linesUnder(t, readme, "## Status Indicators"), 1),
	}
}

// glyphSet sorts and dedupes glyphs so two lists compare as sets.
func glyphSet(glyphs []string) string {
	seen := map[string]bool{}
	var out []string
	for _, g := range glyphs {
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func TestStatusLegendsListTheGlyphsRowsDraw(t *testing.T) {
	legends := statusLegends(t)

	for _, view := range []state.ViewMode{state.ViewList, state.ViewCreated} {
		t.Run(view.String(), func(t *testing.T) {
			h := newHarness(t, glyphIssues(), withView(view))
			h.typeRunes("C")

			drawn := drawnGlyphs(h)
			if len(drawn) != len(glyphIssues()) {
				t.Fatalf("got rows for %v, want one per fixture issue\n%s", drawn, h.screenText())
			}
			var glyphs []string
			for _, g := range drawn {
				glyphs = append(glyphs, g)
			}
			want := glyphSet(glyphs)

			for name, listed := range legends {
				if got := glyphSet(listed); got != want {
					t.Errorf("%s lists %q, but %s view rows draw %q", name, got, view, want)
				}
			}
		})
	}
}
