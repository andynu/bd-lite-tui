package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// creatorIssues is one issue per created_by shape the rows have to tell apart.
func creatorIssues() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-old", "Filed before bd stamped a creator"),
		newIssue("tui-mine", "Filed by me", withCreatedBy("andy")),
		newIssue("tui-agent", "Filed by my agent", withCreatedBy("andy/claude-xyz")),
		newIssue("tui-other", "Filed by Scott", withCreatedBy("scott")),
	}
}

// Only someone else's name reaches the row. My own filings and the issues that
// predate created_by say nothing, which is what keeps a second name per row from
// costing width on every line (tui-dfqm).
func TestCreatedBy_OnlyNamesOtherPeople(t *testing.T) {
	// Wide enough that the suffix survives to the screen; at the default width
	// the list pane truncates it mid-name.
	h := newHarness(t, creatorIssues(), withIdentity("andy"), withScreenSize(200, 40))

	if row := rowContaining(t, h, "Filed by Scott"); !strings.Contains(row, "+scott") {
		t.Errorf("Scott's row does not name him: %q", row)
	}
	for _, title := range []string{
		"Filed before bd stamped a creator",
		"Filed by me",
		"Filed by my agent",
	} {
		if row := rowContaining(t, h, title); strings.Contains(row, "+") {
			t.Errorf("row %q names a creator it should have kept quiet about: %q", title, row)
		}
	}
}

// The creator sits next to the assignee, so a row reads "who filed it, who has
// it" in one place.
func TestCreatedBy_SitsBesideTheAssignee(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-both", "Filed by Scott, mine now",
			withCreatedBy("scott"), withAssignee("andy")),
	}, withIdentity("andy"), withScreenSize(200, 40))

	row := rowContaining(t, h, "Filed by Scott, mine now")
	creator, assignee := strings.Index(row, "+scott"), strings.Index(row, "@andy")
	if creator < 0 || assignee < 0 {
		t.Fatalf("row is missing a name: %q", row)
	}
	if creator > assignee {
		t.Errorf("the creator should precede the assignee: %q", row)
	}
}

// Tree rows carry the same suffix as list rows.
func TestCreatedBy_RendersInTreeRows(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-top", "Top level epic", withType(parser.TypeEpic), withCreatedBy("andy")),
		newIssue("tui-kid", "Child of the epic", withParent("tui-top"), withCreatedBy("scott/claude-abc")),
	}
	h := newHarness(t, issues, withTreeView(), withIdentity("andy"), withScreenSize(200, 40))
	h.typeRunes("O") // the epic starts collapsed, hiding the child row

	text := h.screenText()
	if !strings.Contains(text, "DEPENDENCY TREE") {
		t.Fatalf("test setup: not in tree view:\n%s", text)
	}
	if !strings.Contains(text, "+scott/claude-abc") {
		t.Errorf("the child row does not name its creator:\n%s", text)
	}
	if row := rowContaining(t, h, "Top level epic"); strings.Contains(row, "+") {
		t.Errorf("my own epic names me: %q", row)
	}
}

// The time-keyed views draw the same rows, so they carry the suffix too.
func TestCreatedBy_RendersInCreatedView(t *testing.T) {
	h := newHarness(t, creatorIssues(), withIdentity("andy"), withScreenSize(200, 40))
	h.typeRunes("tt") // list -> tree -> created

	text := h.screenText()
	if !strings.Contains(text, "+scott") {
		t.Errorf("the created view dropped the creator suffix:\n%s", text)
	}
}

// With no identity resolved every filer is a stranger, so every name shows
// rather than the TUI guessing that the unnamed ones are mine.
func TestCreatedBy_UnknownIdentityNamesEveryone(t *testing.T) {
	h := newHarness(t, creatorIssues(), withScreenSize(200, 40))

	text := h.screenText()
	for _, want := range []string{"+andy", "+andy/claude-xyz", "+scott"} {
		if !strings.Contains(text, want) {
			t.Errorf("rows are missing %q:\n%s", want, text)
		}
	}
	if row := rowContaining(t, h, "Filed before bd stamped a creator"); strings.Contains(row, "+") {
		t.Errorf("an issue with no created_by was given a name: %q", row)
	}
}

// A bracket in created_by must not be read as a tview style tag, which would
// swallow it and corrupt the row (the same failure as tui-6ajo).
func TestCreatedBy_BracketsAreEscaped(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-odd", "Odd creator", withCreatedBy("scott[bot]")),
	}, withIdentity("andy"), withScreenSize(200, 40))

	if text := h.screenText(); !strings.Contains(text, "+scott[bot]") {
		t.Errorf("the bracketed creator did not survive rendering:\n%s", text)
	}
}
