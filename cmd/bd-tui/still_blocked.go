package main

import (
	"strings"

	"github.com/andynu/bd-lite-tui/internal/state"
)

// stillBlockedNote names what still holds issueID up once its own status no
// longer does: its open blockers, else the parent it inherits blocking from.
// It is appended to the confirmation of a change to open, where it explains
// why the row stayed in BLOCKED. "" when nothing does.
func stillBlockedNote(s *state.State, issueID string) string {
	if blockers := s.OpenBlockers(issueID); len(blockers) > 0 {
		ids := make([]string, 0, len(blockers))
		for _, blocker := range blockers {
			ids = append(ids, blocker.ID)
		}
		return ", still blocked by " + strings.Join(ids, ", ")
	}
	if parent := s.BlockedParent(issueID); parent != "" {
		return ", still blocked through parent " + parent
	}
	return ""
}
