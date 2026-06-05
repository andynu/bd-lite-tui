package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// atomicReplace mimics exactly what bd-lite does: write a .tmp sibling then
// rename it over the target file (changes the target inode each time).
func atomicReplace(t *testing.T, target, content string) {
	t.Helper()
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		t.Fatalf("rename: %v", err)
	}
}

// bd-lite never writes issues.jsonl in place; it writes a temp file and renames
// it over the target. The directory watch must survive repeated inode changes.
func TestWatcher_AtomicRenameRepeated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "issues.jsonl")
	atomicReplace(t, target, "initial")

	var count int32
	w, err := New(target, 50*time.Millisecond, func() { atomic.AddInt32(&count, 1) })
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()
	time.Sleep(100 * time.Millisecond)

	const rounds = 5
	for i := 0; i < rounds; i++ {
		atomicReplace(t, target, fmt.Sprintf("content %d", i))
		time.Sleep(150 * time.Millisecond) // exceed debounce so each fires
	}
	time.Sleep(200 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got < rounds {
		t.Errorf("expected >=%d onChange calls for atomic renames, got %d", rounds, got)
	}
}

// Regression for tui-uvxx: when the whole .beads directory is removed and
// recreated (git checkout/stash/worktree swap, bd init), the inotify watch was
// silently dropped and live updates stalled permanently.
func TestWatcher_SurvivesDirRecreation(t *testing.T) {
	root := t.TempDir()
	beads := filepath.Join(root, ".beads")
	target := filepath.Join(beads, "issues.jsonl")
	if err := os.Mkdir(beads, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("v0"), 0644); err != nil {
		t.Fatal(err)
	}

	var count int32
	w, err := New(target, 50*time.Millisecond, func() { atomic.AddInt32(&count, 1) })
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Stop() }()
	time.Sleep(100 * time.Millisecond)

	// Sanity: a normal write fires before the disruption.
	_ = os.WriteFile(target, []byte("v1"), 0644)
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&count) == 0 {
		t.Fatal("watcher did not fire on initial write")
	}

	// Simulate git checkout / stash removing and recreating .beads.
	if err := os.RemoveAll(beads); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := os.Mkdir(beads, 0755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond) // let the watch be re-established

	// Writes to the recreated file must still be observed.
	before := atomic.LoadInt32(&count)
	_ = os.WriteFile(target, []byte("v2"), 0644)
	time.Sleep(150 * time.Millisecond)
	atomicReplace(t, target, "v3")
	time.Sleep(150 * time.Millisecond)

	if after := atomic.LoadInt32(&count); after == before {
		t.Errorf("watcher stalled after .beads recreation: no events for post-recreation writes (errorCount=%d)", w.ErrorCount())
	}
}
