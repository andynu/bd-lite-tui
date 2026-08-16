package layout

import "testing"

func TestResolveWideUnpinnedIsColumns(t *testing.T) {
	if got := Resolve(120, Columns, false); got != Columns {
		t.Errorf("Resolve(120, _, unpinned) = %v, want Columns", got)
	}
}

func TestResolveNarrowUnpinnedIsPaged(t *testing.T) {
	if got := Resolve(40, Columns, false); got != Paged {
		t.Errorf("Resolve(40, _, unpinned) = %v, want Paged", got)
	}
}

func TestResolveNarrowOverridesPin(t *testing.T) {
	// A herdr dock pane is ~36 columns. Neither split leaves a readable
	// column there, so the threshold beats an explicit pin.
	for _, pin := range []Mode{Columns, Rows} {
		if got := Resolve(36, pin, true); got != Paged {
			t.Errorf("Resolve(36, %v, pinned) = %v, want Paged", pin, got)
		}
	}
}

func TestResolveWideHonorsPin(t *testing.T) {
	cases := []Mode{Columns, Rows, Paged}
	for _, pin := range cases {
		if got := Resolve(200, pin, true); got != pin {
			t.Errorf("Resolve(200, %v, pinned) = %v, want %v", pin, got, pin)
		}
	}
}

func TestResolveThresholdBoundary(t *testing.T) {
	if got := Resolve(NarrowThreshold, Columns, false); got != Columns {
		t.Errorf("Resolve(%d, _, unpinned) = %v, want Columns (threshold is the first wide width)",
			NarrowThreshold, got)
	}
	if got := Resolve(NarrowThreshold-1, Columns, false); got != Paged {
		t.Errorf("Resolve(%d, _, unpinned) = %v, want Paged", NarrowThreshold-1, got)
	}
}

func TestResolveDegenerateWidths(t *testing.T) {
	// tview reports 0 before the first draw; never return a split for it.
	for _, w := range []int{0, -1} {
		if got := Resolve(w, Rows, true); got != Paged {
			t.Errorf("Resolve(%d, Rows, pinned) = %v, want Paged", w, got)
		}
	}
}

func TestModeStringsAreDistinct(t *testing.T) {
	seen := map[string]Mode{}
	for _, m := range []Mode{Columns, Rows, Paged} {
		s := m.String()
		if s == "" {
			t.Errorf("Mode(%d).String() is empty", int(m))
		}
		if prev, dup := seen[s]; dup {
			t.Errorf("Mode(%d) and Mode(%d) both stringify to %q", int(prev), int(m), s)
		}
		seen[s] = m
	}
}

func TestNextCyclesThroughEveryMode(t *testing.T) {
	// `v` cycles the pin; every mode must be reachable and the cycle must close.
	start := Columns
	m := start
	for i := 0; i < 3; i++ {
		m = Next(m)
	}
	if m != start {
		t.Errorf("three Next() calls from %v landed on %v, want a closed 3-cycle", start, m)
	}

	seen := map[Mode]bool{}
	m = start
	for i := 0; i < 3; i++ {
		seen[m] = true
		m = Next(m)
	}
	if len(seen) != 3 {
		t.Errorf("cycle visited %d modes, want 3", len(seen))
	}
}
