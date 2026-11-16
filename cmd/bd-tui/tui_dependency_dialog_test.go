package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// bd answers dep add with a message object, not an issue. The dialog must take
// that as success: confirm, close, and reload the issue it changed.
func TestDependencyDialog_AddConfirmsAndCloses(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.selectIssue("tui-aaa")

	h.typeRunes("D")
	h.typeRunes("tui-bbb")
	h.key(tcell.KeyTab) // to the Add Dependency button
	h.key(tcell.KeyEnter)

	want := []string{"dep", "add", "tui-aaa", "tui-bbb"}
	if calls := h.calls(); len(calls) != 1 || !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd calls = %v, want one call %v", calls, want)
	}
	if got := h.frontPage(); got != "main" {
		t.Fatalf("dialog still up after a successful add: front page %q", got)
	}
	if status := h.statusText(); !strings.Contains(status, "✓ Now blocked by tui-bbb") {
		t.Errorf("no confirmation after a successful add: %q", status)
	}
	h.waitFor("tui-aaa to reload blocked by tui-bbb", func() bool {
		return h.tui.state.IsEffectivelyBlocked("tui-aaa")
	})
}

func TestDependencyDialog_RemoveConfirmsAndCloses(t *testing.T) {
	issues := basicIssues()
	issues[0] = newIssue("tui-aaa", "Alpha issue", withBlockedBy("tui-bbb"))
	h := newHarness(t, issues)
	h.selectIssue("tui-aaa")

	h.typeRunes("D")
	h.key(tcell.KeyTab) // Add Dependency
	h.key(tcell.KeyTab) // Remove: blocked by tui-bbb
	h.key(tcell.KeyEnter)

	want := []string{"dep", "remove", "tui-aaa", "tui-bbb"}
	if calls := h.calls(); len(calls) != 1 || !reflect.DeepEqual(calls[0].Args, want) {
		t.Fatalf("bd calls = %v, want one call %v", calls, want)
	}
	if got := h.frontPage(); got != "main" {
		t.Fatalf("dialog still up after a successful remove: front page %q", got)
	}
	if status := h.statusText(); !strings.Contains(status, "✓ No longer blocked by tui-bbb") {
		t.Errorf("no confirmation after a successful remove: %q", status)
	}
	h.waitFor("tui-aaa to reload unblocked", func() bool {
		return !h.tui.state.IsEffectivelyBlocked("tui-aaa")
	})
}

// bd-lite stores a self-dependency, which blocks the issue for good, so the
// dialog refuses it without calling bd.
func TestDependencyDialog_RefusesASelfDependency(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.selectIssue("tui-aaa")

	h.typeRunes("D")
	h.typeRunes("tui-aaa")
	h.key(tcell.KeyTab)
	h.key(tcell.KeyEnter)

	if calls := h.calls(); len(calls) != 0 {
		t.Fatalf("a self-dependency reached bd: %v", calls)
	}
	if got := h.frontPage(); got != "dependency_dialog" {
		t.Fatalf("dialog closed on a refused self-dependency: front page %q", got)
	}
	if status := h.statusText(); !strings.Contains(status, "cannot depend on itself") {
		t.Errorf("status does not explain the refusal: %q", status)
	}
}

// A dep command that bd rejects keeps the dialog up and says why.
func TestDependencyDialog_FailureKeepsDialogOpen(t *testing.T) {
	h := newHarness(t, basicIssues())
	h.setBdResponse(func(args []string) (*BdCommandResult, error) {
		return nil, errString("bd dep failed: no issue found matching 'tui-bbb'")
	})
	h.selectIssue("tui-aaa")

	h.typeRunes("D")
	h.typeRunes("tui-bbb")
	h.key(tcell.KeyTab)
	h.key(tcell.KeyEnter)

	if got := h.frontPage(); got != "dependency_dialog" {
		t.Fatalf("a failed add closed the dialog: front page %q", got)
	}
	if status := h.statusText(); !strings.Contains(status, "Error adding dependency") {
		t.Errorf("failed add did not report an error: %q", status)
	}
	if h.tui.state.IsEffectivelyBlocked("tui-aaa") {
		t.Errorf("tui-aaa became blocked although bd failed")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
