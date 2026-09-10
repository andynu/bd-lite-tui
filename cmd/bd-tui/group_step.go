package main

import "github.com/andynu/bd-lite-tui/internal/parser"

// groupStep is a quick status change waiting for the refresh that loads it.
// When that refresh files the issue under a different group header, the cursor
// goes to stepTo, a row from the group it left, instead of following it, so
// working down a group one status change at a time needs no j in between. A
// change that leaves the row in its group still follows the row.
type groupStep struct {
	issueID string
	status  parser.Status // the status bd reported writing
	stepTo  string        // "" when the row was alone in its group
}

// landed reports whether loaded reflects the change: the issue carries the
// status bd wrote, or is gone. A refresh that loaded before bd wrote still sees
// the old status and leaves the step for the next one.
func (g *groupStep) landed(loaded *parser.Issue) bool {
	return loaded == nil || loaded.Status == g.status
}

// groupNeighbor names the row that comes under the cursor when the issue at idx
// leaves its group: the next row in the group, else the previous one. A group's
// rows are contiguous and its header has no entry in indexToIssue, so an
// adjacent index holding an issue is always in the same group.
func groupNeighbor(indexToIssue map[int]*parser.Issue, idx int) string {
	if _, ok := indexToIssue[idx]; !ok {
		return ""
	}
	if next, ok := indexToIssue[idx+1]; ok {
		return next.ID
	}
	if prev, ok := indexToIssue[idx-1]; ok {
		return prev.ID
	}
	return ""
}

// selectionIndex finds the row the cursor belongs on after a rebuild: issueID's
// own, unless stepTo now sits under a different header, which means issueID has
// left the group the two shared. ok is false when issueID is no longer listed.
func selectionIndex(indexToIssue map[int]*parser.Issue, issueID, stepTo string) (int, bool) {
	idx, ok := issueIndex(indexToIssue, issueID)
	if !ok {
		return 0, false
	}
	if stepTo == "" {
		return idx, true
	}
	stepIdx, ok := issueIndex(indexToIssue, stepTo)
	if !ok || groupHeader(indexToIssue, stepIdx) == groupHeader(indexToIssue, idx) {
		return idx, true
	}
	return stepIdx, true
}

func issueIndex(indexToIssue map[int]*parser.Issue, id string) (int, bool) {
	for idx, issue := range indexToIssue {
		if issue.ID == id {
			return idx, true
		}
	}
	return 0, false
}

// groupHeader returns the index of the nearest row above idx that carries no
// issue, which is the header of idx's group, or -1 when there is none.
func groupHeader(indexToIssue map[int]*parser.Issue, idx int) int {
	for ; idx >= 0; idx-- {
		if _, ok := indexToIssue[idx]; !ok {
			return idx
		}
	}
	return -1
}
