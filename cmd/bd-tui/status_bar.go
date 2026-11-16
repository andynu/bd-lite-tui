package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/andynu/bd-lite-tui/internal/bdversion"
)

// statusLocationWidth is the widest the .beads location may render. The fields
// after it are the ones the user acts on, and tview clips the status line at
// the right edge rather than wrapping it, so the location has to be bounded by
// something well under a narrow terminal's width.
const statusLocationWidth = 24

// statusLocation labels the checkout the TUI is attached to.
//
// The trailing ".beads" element is dropped (it is the same for every repo, so
// it distinguishes nothing) and a path under home renders with a leading "~",
// which puts an ordinary project at ~/projects/bd-lite-tui. Anything still
// wider than statusLocationWidth keeps whole trailing elements under a leading
// ellipsis: the tail is the part that names the repository.
//
// home is a parameter rather than an environment lookup so the result is a
// function of the arguments alone.
func statusLocation(beadsDir, home string) string {
	const sep = string(filepath.Separator)

	dir := beadsDir
	if len(dir) > 1 {
		dir = strings.TrimRight(dir, sep)
	}
	if filepath.Base(dir) == ".beads" {
		dir = filepath.Dir(dir)
	}
	// A bare relative ".beads" leaves nothing above it. Name the directory
	// itself rather than rendering "." at the user.
	if dir == "" || dir == "." {
		return filepath.Base(strings.TrimRight(beadsDir, sep))
	}

	if home = strings.TrimRight(home, sep); home != "" {
		if dir == home {
			dir = "~"
		} else if strings.HasPrefix(dir, home+sep) {
			dir = "~" + dir[len(home):]
		}
	}

	if utf8.RuneCountInString(dir) <= statusLocationWidth {
		return dir
	}

	// Grow the tail one whole element at a time. Cutting mid-element would
	// leave "ects/bd-lite-tui", which reads as a different directory.
	parts := strings.Split(dir, sep)
	out := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		candidate := parts[i] + sep + out
		if utf8.RuneCountInString(candidate)+utf8.RuneCountInString("…"+sep) > statusLocationWidth {
			break
		}
		out = candidate
	}
	if utf8.RuneCountInString(out)+utf8.RuneCountInString("…"+sep) <= statusLocationWidth {
		return "…" + sep + out
	}

	// One element wider than the whole budget: keep its tail.
	runes := []rune(out)
	return "…" + string(runes[len(runes)-(statusLocationWidth-1):])
}

// statusBarFields is everything the status line reports. Declaration order is
// render order, and render order is what survives a narrow terminal: tview
// clips the tail of the line, so a field the user acts on has to come before
// one they only read.
type statusBarFields struct {
	// EmphasisColor is the theme color for the "Beads TUI" title.
	EmphasisColor string

	// Location names the checkout, already shortened by statusLocation.
	Location string

	// IssueCount is how many issues the list is showing.
	IssueCount int

	// BdWarning says what is wrong with the bd on PATH, empty when nothing
	// is. It comes first among the fields because every edit depends on bd.
	BdWarning string

	// WarningColor colors BdWarning.
	WarningColor string

	// Focus is "List" or "Details". In the Paged layout only one of the two
	// panels is mounted at all, which makes this the sole signal for which
	// screen the user is looking at.
	Focus string

	// View is the active view mode ("List", "Tree", "Created", "Updated").
	// It comes early because it decides what the rows mean: the same issue
	// set is grouped by status, by dependency, or by time.
	View string

	// Layout is the resolved layout mode, with a trailing "*" when the user
	// has pinned it with v.
	Layout string

	// Filters describes the active filter, empty when nothing is filtered.
	// It comes before the passive fields because a filter hides rows, and a
	// user wondering where an issue went needs to see it.
	Filters string

	// ShowingClosed reports the C toggle, for the same reason.
	ShowingClosed bool

	// MouseEnabled reports the m toggle. Nothing depends on it being visible,
	// so it sits at the end with the help hint.
	MouseEnabled bool
}

// bdStatusWarning is the [bd: ...] field: a compatibility problem first, else
// a newer release a bd update notice named, else nothing.
func bdStatusWarning(r bdversion.Result, latest string) string {
	if w := r.Warning(); w != "" {
		return w
	}
	if latest != "" {
		return latest + " available"
	}
	return ""
}

// formatStatusBar renders the status line.
func formatStatusBar(f statusBarFields) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[%s]Beads TUI[-] - %s (%d issues)", f.EmphasisColor, f.Location, f.IssueCount)
	if f.BdWarning != "" {
		// "[bd: ...[]" for the same tag-escape reason as the layout field below.
		fmt.Fprintf(&b, " [%s][bd: %s[][-]", f.WarningColor, f.BdWarning)
	}
	fmt.Fprintf(&b, " [Focus: %s]", f.Focus)
	if f.View != "" {
		fmt.Fprintf(&b, " [View: %s]", f.View)
	}
	// "[%s[]" is tview's escape for a literal "[Columns]". Written bare, the tag
	// parser reads "Columns" as a color name and swallows the whole thing, so
	// the indicator only ever appeared in its pinned "[Columns*]" form, where
	// the asterisk made the tag invalid (tui-42ws). The escape is applied to
	// both forms: the fields around it use the same "[Word: value]" shape and a
	// reader should not have to know which of them tview happens to accept.
	fmt.Fprintf(&b, " [%s[]", f.Layout)

	if f.Filters != "" {
		fmt.Fprintf(&b, " [Filters: %s]", f.Filters)
	}
	if f.ShowingClosed {
		b.WriteString(" [Showing Closed]")
	}

	mouse := "OFF"
	if f.MouseEnabled {
		mouse = "ON"
	}
	fmt.Fprintf(&b, " [Mouse: %s]", mouse)
	b.WriteString(" [? help | v layout]")

	return b.String()
}
