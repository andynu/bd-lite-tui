// Package identity resolves the human whose behalf the TUI acts on, and decides
// whether an assignee value belongs to that human.
//
// Identity is deliberately human-level: it is always `git user.name`. Agents may
// layer their own tag on top by suffixing it after a slash ("andy/claude-xyz"),
// but the value must always carry the human it belongs to, so the segment before
// the first slash is what identifies the person.
//
// bd's own `bd list --assignee` matches the whole string exactly, so it cannot
// answer "is this mine" for a prefixed value. That question is answered here,
// against issues the TUI has already parsed.
package identity

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// resolveTimeout bounds the `git config` call so a wedged git cannot delay
// startup.
const resolveTimeout = 2 * time.Second

// Resolve returns `git user.name` as seen from dir, or "" when it is unset, dir
// is not inside a repository, or git is unavailable. Callers must treat "" as
// "identity unknown" and skip anything that would need it (auto-claim, the
// "mine" filter) rather than substituting a guess.
func Resolve(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	args := []string{"config", "user.name"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}

	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Human returns the person an assignee value belongs to: everything before the
// first slash, so "andy/claude-xyz" and "andy" both resolve to "andy". A value
// with no slash is already human-level and is returned unchanged.
func Human(assignee string) string {
	assignee = strings.TrimSpace(assignee)
	if i := strings.Index(assignee, "/"); i >= 0 {
		return strings.TrimSpace(assignee[:i])
	}
	return assignee
}

// IsMine reports whether assignee belongs to me. Comparison is on the human
// segment and case-insensitive, so an agent-tagged value still counts as its
// human's. An empty assignee is nobody's, and an empty me matches nothing.
func IsMine(assignee, me string) bool {
	if assignee == "" || me == "" {
		return false
	}
	return strings.EqualFold(Human(assignee), Human(me))
}
