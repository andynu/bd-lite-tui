package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Pins the bd-lite behavior the TUI's empty-store handling assumes: a .beads
// directory with no issues.jsonl is a usable store, and the file appears on the
// first write rather than at init time.
//
// This is why bd-tui no longer exits when the file is absent. It is also why
// `bd init` is not a repair step: FindBeadsDir only returns once .beads exists,
// and init refuses that case outright ("Error: .beads directory already
// exists"), with or without --prefix.
func TestBdCreatesStoreOnFirstWrite_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH")
	}

	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatalf("failed to create .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "config.yaml"), []byte("issue-prefix: test\n"), 0644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")

	t.Chdir(dir)

	if initOut, err := exec.Command("bd", "init").CombinedOutput(); err == nil {
		t.Errorf("bd init succeeded in an existing .beads; the store may no longer need bd create to appear: %s", initOut)
	}
	if _, err := os.Stat(jsonlPath); err == nil {
		t.Error("bd init created issues.jsonl")
	}

	if out, err := exec.Command("bd", "create", "First issue").CombinedOutput(); err != nil {
		t.Fatalf("bd create failed: %v: %s", err, out)
	}
	if _, err := os.Stat(jsonlPath); err != nil {
		t.Errorf("bd create did not produce %s: %v", jsonlPath, err)
	}
}
