package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
)

// assigneeIssues is one issue per assignee shape the UI has to tell apart.
func assigneeIssues() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-none", "Unclaimed work"),
		newIssue("tui-mine", "Claimed by me", withAssignee("andy")),
		newIssue("tui-agent", "Claimed by my agent", withAssignee("andy/claude-xyz")),
		newIssue("tui-other", "Claimed by Scott", withAssignee("scott")),
	}
}

// --- list rows -------------------------------------------------------------

// An assigned issue carries a dimmed @name in its row; an unassigned one does
// not. screenText is what actually reached the terminal, so this covers the
// tview tag handling as well as the formatting.
func TestAssignee_RendersInListRows(t *testing.T) {
	// Wide enough that the assignee suffix survives to the screen; at the
	// default width the list pane truncates it mid-name.
	h := newHarness(t, assigneeIssues(), withIdentity("andy"), withScreenSize(200, 40))

	text := h.screenText()
	for _, want := range []string{"@andy", "@andy/claude-xyz", "@scott"} {
		if !strings.Contains(text, want) {
			t.Errorf("list rows are missing %q:\n%s", want, text)
		}
	}

	// The unassigned row exists but shows no @.
	row := rowContaining(t, h, "Unclaimed work")
	if strings.Contains(row, "@") {
		t.Errorf("the unassigned row shows an assignee: %q", row)
	}
}

// Tree view renders the same suffix as list view.
func TestAssignee_RendersInTreeRows(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-top", "Top level epic", withType(parser.TypeEpic), withAssignee("andy")),
		newIssue("tui-kid", "Child of the epic", withParent("tui-top"), withAssignee("scott/claude-abc")),
	}
	h := newHarness(t, issues, withTreeView(), withIdentity("andy"), withScreenSize(200, 40))
	h.typeRunes("O") // the epic starts collapsed, hiding the child row

	text := h.screenText()
	if !strings.Contains(text, "DEPENDENCY TREE") {
		t.Fatalf("test setup: not in tree view:\n%s", text)
	}
	for _, want := range []string{"@andy", "@scott/claude-abc"} {
		if !strings.Contains(text, want) {
			t.Errorf("tree rows are missing %q:\n%s", want, text)
		}
	}
}

// A bracket in an assignee must not be read as a tview style tag, which would
// swallow it and corrupt the row (the same failure as tui-6ajo).
func TestAssignee_BracketsAreEscaped(t *testing.T) {
	h := newHarness(t, []*parser.Issue{
		newIssue("tui-odd", "Odd assignee", withAssignee("andy[bot]")),
	}, withIdentity("andy"), withScreenSize(200, 40))

	if text := h.screenText(); !strings.Contains(text, "@andy[bot]") {
		t.Errorf("the bracketed assignee did not survive rendering:\n%s", text)
	}
}

// --- filtering -------------------------------------------------------------

// The quick filter narrows to the current identity, and an agent-tagged value
// counts as its human's.
func TestAssigneeFilter_MineIncludesAgentValues(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))

	h.typeRunes("f")
	h.typeRunes("mine")
	h.key(tcell.KeyEnter)

	// Equal priority and age, so the id breaks the tie.
	if got, want := h.issueIDs(), []string{"tui-agent", "tui-mine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered list is %v, want %v", got, want)
	}
	if text := h.statusText(); !strings.Contains(text, "Assignee: mine") {
		t.Errorf("status bar is %q, want it to name the assignee filter", text)
	}
}

// "mine" matches a full git user.name under an agent tag, which is the shape
// this repository actually produces.
func TestAssigneeFilter_MineMatchesAFullName(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-mine", "Claimed", withAssignee("Andy Nutter-Upham")),
		newIssue("tui-agent", "Agent's", withAssignee("Andy Nutter-Upham/claude-xyz")),
		newIssue("tui-other", "Scott's", withAssignee("Scott")),
	}
	h := newHarness(t, issues, withIdentity("Andy Nutter-Upham"))

	h.typeRunes("f")
	h.typeRunes("mine")
	h.key(tcell.KeyEnter)

	// Equal priority and age, so the id breaks the tie.
	if got, want := h.issueIDs(), []string{"tui-agent", "tui-mine"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered list is %v, want %v", got, want)
	}
}

func TestAssigneeFilter_Unassigned(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))

	h.typeRunes("f")
	h.typeRunes("unassigned")
	h.key(tcell.KeyEnter)

	if got, want := h.issueIDs(), []string{"tui-none"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered list is %v, want %v", got, want)
	}
	if text := h.statusText(); !strings.Contains(text, "Assignee: unassigned") {
		t.Errorf("status bar is %q, want it to name the assignee filter", text)
	}
}

// @name narrows to one person, and picks up their agent's issues too.
func TestAssigneeFilter_NamedAssignee(t *testing.T) {
	issues := append(assigneeIssues(),
		newIssue("tui-other-agent", "Scott's agent", withAssignee("scott/claude-abc")))
	h := newHarness(t, issues, withIdentity("andy"))

	h.typeRunes("f")
	h.typeRunes("@scott")
	h.key(tcell.KeyEnter)

	if got, want := h.issueIDs(), []string{"tui-other", "tui-other-agent"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered list is %v, want %v", got, want)
	}
}

// The assignee filter composes with the others rather than replacing them.
func TestAssigneeFilter_CombinesWithPriority(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-mine-p1", "Mine and urgent", withAssignee("andy"), withPriority(1)),
		newIssue("tui-mine-p3", "Mine but minor", withAssignee("andy"), withPriority(3)),
		newIssue("tui-other-p1", "Urgent but Scott's", withAssignee("scott"), withPriority(1)),
	}
	h := newHarness(t, issues, withIdentity("andy"))

	h.typeRunes("f")
	h.typeRunes("mine p1")
	h.key(tcell.KeyEnter)

	if got, want := h.issueIDs(), []string{"tui-mine-p1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered list is %v, want %v", got, want)
	}
}

// With no git user.name there is nobody to compare against, so "mine" installs
// no filter rather than emptying the list.
func TestAssigneeFilter_MineWithoutAnIdentityIsInert(t *testing.T) {
	h := newHarness(t, assigneeIssues()) // no withIdentity

	h.typeRunes("f")
	h.typeRunes("mine")
	h.key(tcell.KeyEnter)

	var hasFilters bool
	h.read(func() { hasFilters = h.tui.state.HasActiveFilters() })
	if hasFilters {
		t.Error("'mine' installed a filter with no identity resolved")
	}
	if got := len(h.issueIDs()); got != 4 {
		t.Errorf("%d rows are visible, want all 4", got)
	}
}

// --- assign dialog ---------------------------------------------------------

// A opens the dialog, typing a name and pressing Enter runs the bd update, and
// the reload picks the new assignee up.
func TestAssignDialog_SetsTheAssignee(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-none")

	h.typeRunes("A")
	if got := h.frontPage(); got != "assign_dialog" {
		t.Fatalf("A opened %q, want assign_dialog", got)
	}
	h.typeRunes("scott")
	h.key(tcell.KeyEnter)

	if got := h.frontPage(); got != "main" {
		t.Fatalf("Enter left %q up, want main", got)
	}

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-none", "--assignee", "scott"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); !strings.Contains(status, "Assigned tui-none to scott") {
		t.Errorf("status bar did not confirm the assignment: %q", status)
	}

	h.waitFor("tui-none to show its new assignee", func() bool {
		issue := h.tui.state.GetIssueByID("tui-none")
		return issue != nil && issue.Assignee == "scott"
	})
	if status := h.statusText(); !strings.Contains(status, "Assigned tui-none to scott") {
		t.Errorf("the reload replaced the confirmation: %q", status)
	}
}

// The dialog is pre-filled with the current assignee, so an existing value can
// be edited rather than retyped.
func TestAssignDialog_PrefillsTheCurrentAssignee(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-other")

	h.typeRunes("A")
	text := h.screenText()
	if !strings.Contains(text, "Assign Issue") {
		t.Fatalf("the assign dialog did not render:\n%s", text)
	}
	if !strings.Contains(text, "scott") {
		t.Errorf("the dialog did not show the current assignee:\n%s", text)
	}
}

// Clear empties the field, which bd interprets as dropping the key.
func TestAssignDialog_ClearsTheAssignee(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-other")

	h.typeRunes("A")
	h.pressButton("Clear")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-other", "--assignee", ""}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); !strings.Contains(status, "Cleared assignee on tui-other") {
		t.Errorf("status bar did not confirm the clear: %q", status)
	}

	h.waitFor("tui-other to lose its assignee", func() bool {
		issue := h.tui.state.GetIssueByID("tui-other")
		return issue != nil && issue.Assignee == ""
	})
}

// "Assign to me" fills in git user.name without the user typing it.
func TestAssignDialog_AssignToMe(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("Andy Nutter-Upham"))
	h.selectIssue("tui-none")

	h.typeRunes("A")
	if text := h.screenText(); !strings.Contains(text, "Assign to me") {
		t.Fatalf("the assign-to-me button is missing:\n%s", text)
	}
	h.pressButton("Assign to me")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-none", "--assignee", "Andy Nutter-Upham"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
}

// Without an identity there is nothing to assign to, so the button is not
// offered rather than assigning an empty name.
func TestAssignDialog_NoAssignToMeWithoutAnIdentity(t *testing.T) {
	h := newHarness(t, assigneeIssues()) // no withIdentity
	h.selectIssue("tui-none")

	h.typeRunes("A")
	if text := h.screenText(); strings.Contains(text, "Assign to me") {
		t.Errorf("the assign-to-me button is offered with no identity:\n%s", text)
	}
}

// A failing bd surfaces its message and leaves the dialog up so the user can
// retry rather than losing what they typed.
func TestAssignDialog_ErrorKeepsTheDialogOpen(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-none")
	h.setBdResponse(func([]string) (*BdCommandResult, error) {
		return nil, errBdFailed
	})

	h.typeRunes("A")
	h.typeRunes("scott")
	h.key(tcell.KeyEnter)

	if got := h.frontPage(); got != "assign_dialog" {
		t.Errorf("the dialog closed on a failed update, front page is %q", got)
	}
	if status := h.statusText(); !strings.Contains(status, "Error setting assignee") {
		t.Errorf("status bar did not report the failure: %q", status)
	}
}

// --- auto-claim ------------------------------------------------------------

// Si on an unclaimed issue sets the status and takes the issue, in one bd call.
func TestAutoClaim_TakesAnUnassignedIssue(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-none")

	h.typeRunes("s")
	h.typeRunes("i")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	want := []string{"update", "tui-none", "--status", "in_progress", "--assignee", "andy"}
	if !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); !strings.Contains(status, "claimed it for andy") {
		t.Errorf("status bar did not mention the claim: %q", status)
	}

	h.waitFor("tui-none to be claimed", func() bool {
		issue := h.tui.state.GetIssueByID("tui-none")
		return issue != nil && issue.Assignee == "andy" && issue.Status == parser.StatusInProgress
	})
}

// An issue someone else holds keeps its assignee. Silently taking it would be
// worse than leaving it where it is.
func TestAutoClaim_NeverStealsAnAssignedIssue(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-other")

	h.typeRunes("s")
	h.typeRunes("i")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-other", "--status", "in_progress"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	if status := h.statusText(); strings.Contains(status, "claimed") {
		t.Errorf("status bar claimed an issue that was already assigned: %q", status)
	}
}

// An issue already assigned to us is left alone too: there is nothing to claim,
// and re-sending the assignee would clobber an agent tag on it.
func TestAutoClaim_LeavesMyOwnAgentTagAlone(t *testing.T) {
	h := newHarness(t, assigneeIssues(), withIdentity("andy"))
	h.selectIssue("tui-agent")

	h.typeRunes("s")
	h.typeRunes("i")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-agent", "--status", "in_progress"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
	h.waitFor("tui-agent to keep its agent tag", func() bool {
		issue := h.tui.state.GetIssueByID("tui-agent")
		return issue != nil && issue.Assignee == "andy/claude-xyz"
	})
}

// With no git user.name there is no name to write, so the status change goes
// through on its own instead of failing or assigning an empty string.
func TestAutoClaim_SkippedWithoutAnIdentity(t *testing.T) {
	h := newHarness(t, assigneeIssues()) // no withIdentity
	h.selectIssue("tui-none")

	h.typeRunes("s")
	h.typeRunes("i")

	calls := h.calls()
	if len(calls) != 1 {
		t.Fatalf("expected one bd invocation, got %v", calls)
	}
	if want := []string{"update", "tui-none", "--status", "in_progress"}; !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
	}
}

// Only in_progress claims. The other three status chords must not assign.
func TestAutoClaim_OnlyOnInProgress(t *testing.T) {
	for _, tc := range []struct{ key, status string }{
		{"o", "open"},
		{"b", "blocked"},
		{"c", "closed"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			h := newHarness(t, assigneeIssues(), withIdentity("andy"))
			h.selectIssue("tui-none")

			h.typeRunes("s")
			h.typeRunes(tc.key)

			calls := h.calls()
			if len(calls) != 1 {
				t.Fatalf("expected one bd invocation, got %v", calls)
			}
			if want := []string{"update", "tui-none", "--status", tc.status}; !reflect.DeepEqual(calls[0].Args, want) {
				t.Fatalf("bd called with %v, want %v", calls[0].Args, want)
			}
		})
	}
}
