// Package layout decides how the issue list and detail panel share the screen.
//
// The decision is a pure function of terminal width and the user's pinned
// preference so it can be tested without standing up a tview app.
package layout

// Mode is an arrangement of the issue list and the detail panel.
type Mode int

const (
	// Columns puts the list on the left and the detail on the right.
	Columns Mode = iota
	// Rows puts the list on top and the detail underneath.
	Rows
	// Paged shows exactly one of them, full screen, switched by focus.
	Paged
)

// NarrowThreshold is the first width wide enough for a side-by-side split.
// Under it the list gets fewer than 30 columns, which does not fit an issue id
// and priority tag plus any of the title.
const NarrowThreshold = 90

func (m Mode) String() string {
	switch m {
	case Columns:
		return "Columns"
	case Rows:
		return "Rows"
	case Paged:
		return "Paged"
	default:
		return "Unknown"
	}
}

// Next returns the following mode in the pin cycle, wrapping at the end.
func Next(m Mode) Mode {
	switch m {
	case Columns:
		return Rows
	case Rows:
		return Paged
	default:
		return Columns
	}
}

// Resolve picks the mode for a terminal of the given width. Below
// NarrowThreshold the result is always Paged, even against an explicit pin,
// because neither split leaves a readable column. At or above it, a pinned
// mode wins; otherwise the default is Columns.
func Resolve(width int, pin Mode, pinned bool) Mode {
	if width < NarrowThreshold {
		return Paged
	}
	if pinned {
		return pin
	}
	return Columns
}
