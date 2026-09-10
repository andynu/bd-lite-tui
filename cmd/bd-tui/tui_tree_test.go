package main

import (
	"strings"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// An epic lists its children as blocks deps on them (bd dep add <epic> <child>),
// so the tree nests those targets under the epic instead of reading the edge as
// blocker -> blocked.
func TestTree_EpicNestsItsBlocksTargets(t *testing.T) {
	issues := []*parser.Issue{
		newIssue("tui-epic", "The epic",
			withType(parser.TypeEpic),
			withBlockedBy("tui-one"),
			withBlockedBy("tui-two")),
		newIssue("tui-one", "First child"),
		newIssue("tui-two", "Second child"),
		newIssue("tui-solo", "Unrelated issue"),
	}
	h := newHarness(t, issues, withTreeView(), withScreenSize(200, 40))

	epicRow := rowContaining(t, h, "The epic")
	if !strings.Contains(epicRow, "▶") || !strings.Contains(epicRow, "(2 children)") {
		t.Errorf("the epic row has no fold marker for its two children: %q", epicRow)
	}

	h.typeRunes("O")

	for _, title := range []string{"First child", "Second child"} {
		if row := rowContaining(t, h, title); !strings.Contains(row, "── ") {
			t.Errorf("%q is drawn as a root, not under the epic: %q", title, row)
		}
	}
	if row := rowContaining(t, h, "Unrelated issue"); strings.Contains(row, "── ") {
		t.Errorf("an issue with no deps is nested: %q", row)
	}
}
