// Package identity resolves the human whose behalf the TUI acts on, and decides
// whether a name recorded on an issue (its assignee, its created_by) belongs to
// that human.
//
// Identity is deliberately human-level: `git user.name`, or $USER when git has
// no name to give. Agents may layer their own tag on top by suffixing it after a
// slash ("andy/claude-xyz"), but the value must always carry the human it
// belongs to, so the segment before the first slash is what identifies the
// person.
//
// bd's own `bd list --assignee` matches the whole string exactly, so it cannot
// answer "is this mine" for a prefixed value. That question is answered here,
// against issues the TUI has already parsed.
package identity

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// resolveTimeout bounds the `git config` call so a wedged git cannot delay
// startup.
const resolveTimeout = 2 * time.Second

// Resolve returns `git user.name` as seen from dir, falling back to $USER when
// git has no name to give: user.name unset, dir outside a repository, or git
// unavailable. That fallback is the tail of the chain bd-lite itself walks when
// it stamps created_by (BD_ACTOR, then git user.name, then $USER), so a store
// written without a git name still reads back as this human's work rather than
// as a stranger's. BD_ACTOR is not consulted: it is a per-command override for
// one bd invocation, not a description of who is sitting here, and the TUI never
// sets it.
//
// "" means neither source had a name. Callers must treat it as "identity
// unknown" and skip anything that would need it (auto-claim, the "mine" filter)
// rather than substituting a guess.
func Resolve(dir string) string {
	if name := gitUserName(dir); name != "" {
		return name
	}
	return strings.TrimSpace(os.Getenv("USER"))
}

// gitUserName returns user.name as configured for dir, or "" when git cannot
// answer.
func gitUserName(dir string) string {
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

// Human returns the person a recorded name belongs to: everything before the
// first slash, so "andy/claude-xyz" and "andy" both resolve to "andy". A value
// with no slash is already human-level and is returned unchanged.
func Human(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, "/"); i >= 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

// IsMine reports whether a name recorded on an issue — an assignee, a created_by
// — belongs to me. Comparison is on the human segment and case-insensitive, so
// an agent-tagged value still counts as its human's. An empty name is nobody's,
// and an empty me matches nothing.
func IsMine(name, me string) bool {
	if name == "" || me == "" {
		return false
	}
	return strings.EqualFold(Human(name), Human(me))
}
