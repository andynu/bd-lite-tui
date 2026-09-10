package main

import (
	"strings"

	"github.com/rivo/tview"
)

// detailLineAt maps a content row of the detail panel (0 is the first row of
// text, scroll already added) to the logical line of text drawn there, where
// logical lines are text split on "\n". Every line above the row is
// word-wrapped at width, since a long title, description or dependency line
// spans several rows. tview.WordWrap uses the same line breaker and tag parsing
// as the TextView, so the rows agree with the screen.
//
// ok is false for a row past the end of the text or a panel with no width.
func detailLineAt(text string, width, row int) (line int, ok bool) {
	if width <= 0 || row < 0 {
		return 0, false
	}
	for i, logical := range strings.Split(text, "\n") {
		rows := len(tview.WordWrap(logical, width))
		if rows == 0 {
			rows = 1 // an empty line still takes a row
		}
		if row < rows {
			return i, true
		}
		row -= rows
	}
	return 0, false
}
