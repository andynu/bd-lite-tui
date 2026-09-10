package main

import "testing"

func TestDetailLineAt(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		row   int
		line  int
		ok    bool
	}{
		{"first row", "a\nb\nc", 10, 0, 0, true},
		{"one row per short line", "a\nb\nc", 10, 2, 2, true},
		{"past the end", "a\nb\nc", 10, 3, 0, false},
		// "aaaa bbbb cccc" wraps to three rows at width 5, so "next" is row 3.
		{"inside a wrapped line", "aaaa bbbb cccc\nnext", 5, 2, 0, true},
		{"after a wrapped line", "aaaa bbbb cccc\nnext", 5, 3, 1, true},
		// Style tags take no width: this is nine visible cells, one row.
		{"tags do not wrap", "[red]aaaa[-] bbbb\nnext", 9, 1, 1, true},
		{"empty line takes a row", "a\n\nb", 10, 2, 2, true},
		{"no width", "a", 0, 0, 0, false},
		{"negative row", "a", 10, -1, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line, ok := detailLineAt(tc.text, tc.width, tc.row)
			if line != tc.line || ok != tc.ok {
				t.Fatalf("detailLineAt(%q, %d, %d) = %d, %t; want %d, %t",
					tc.text, tc.width, tc.row, line, ok, tc.line, tc.ok)
			}
		})
	}
}
