package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// singleFlight mirrors the coalescing dispatcher used by refreshIssues in
// main(). It is replicated here so the concurrency invariants can be tested
// directly: the refresh logic itself is a closure inside main() and not
// otherwise reachable. If the algorithm in main() changes, update this too.
//
// gap, if non-nil, is invoked at the vulnerable point — after the inner loop
// has observed no pending request but before running is cleared — so tests can
// deterministically simulate a trigger arriving in that window. Production has
// no such hook.
func singleFlight(running, again *atomic.Bool, work, gap func()) {
	again.Store(true)
	if !running.CompareAndSwap(false, true) {
		return
	}
	go func() {
		for {
			for again.CompareAndSwap(true, false) {
				work()
			}
			if gap != nil {
				gap()
			}
			running.Store(false)
			// A request may have arrived between the failed CAS above and the
			// Store; re-acquire the slot if so, otherwise we're done.
			if !(again.Load() && running.CompareAndSwap(false, true)) {
				return
			}
		}
	}()
}

func drain(running, again *atomic.Bool) {
	for spins := 0; (running.Load() || again.Load()) && spins < 10_000_000; spins++ {
		runtime.Gosched()
	}
}

// Deterministic regression for the lost-wakeup gap: a trigger that lands after
// the dispatcher saw "nothing pending" but before it released the slot must
// still be served. Without the post-Store re-check, this run is dropped — which
// in the TUI means a file change that never reaches the screen.
func TestSingleFlight_RechecksTriggerInGap(t *testing.T) {
	var running, again atomic.Bool
	var runs atomic.Int64

	gapFired := false
	gap := func() {
		if !gapFired {
			gapFired = true
			// Simulate a trigger that already did again.Store(true) and whose
			// running.CompareAndSwap(false,true) failed (running still true here).
			again.Store(true)
		}
	}

	singleFlight(&running, &again, func() { runs.Add(1) }, gap)
	drain(&running, &again)

	if got := runs.Load(); got != 2 {
		t.Fatalf("expected work to run twice (initial + gap-injected trigger), got %d", got)
	}
}

// Stress: under heavy concurrent triggering, the final state must always be
// observed by some run (no lost wakeup) and there must be no data races.
func TestSingleFlight_NoLostWakeupStress(t *testing.T) {
	const iterations = 2000
	const writers = 8

	for iter := 0; iter < iterations; iter++ {
		var running, again atomic.Bool

		// dataVersion stands in for the latest on-disk state. Each "write" bumps
		// it and triggers a refresh; work() records the version it observed.
		var dataVersion atomic.Int64
		var lastSeen atomic.Int64
		var runs atomic.Int64

		// Serialize work() the way QueueUpdateDraw serializes onto the main
		// thread, and record the highest version any run observed.
		var applyMu sync.Mutex
		work := func() {
			applyMu.Lock()
			defer applyMu.Unlock()
			runs.Add(1)
			if v := dataVersion.Load(); v > lastSeen.Load() {
				lastSeen.Store(v)
			}
		}

		var wg sync.WaitGroup
		for w := 0; w < writers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					dataVersion.Add(1)
					singleFlight(&running, &again, work, nil)
				}
			}()
		}
		wg.Wait()

		final := dataVersion.Load()
		drain(&running, &again)

		if runs.Load() == 0 {
			t.Fatalf("iter %d: work never ran", iter)
		}
		if lastSeen.Load() != final {
			t.Fatalf("iter %d: lost wakeup — final version %d but last observed %d",
				iter, final, lastSeen.Load())
		}
	}
}
