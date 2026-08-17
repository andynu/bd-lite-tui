package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The message is checked here rather than through the tcell harness because the
// missing-store branch runs in main() and calls os.Exit before newTUI is ever
// reached, so no SimulationScreen test can drive it.
func TestMissingStoreMessage(t *testing.T) {
	path := "/tmp/example/.beads/issues.jsonl"
	msg := missingStoreMessage(path)

	if !strings.Contains(msg, path) {
		t.Errorf("message does not name the missing path %q:\n%s", path, msg)
	}
	if !strings.Contains(msg, `bd create "First issue"`) {
		t.Errorf("message does not suggest bd create:\n%s", msg)
	}

	// The bug this replaced: bd-lite has no export command, so the old advice
	// ("bd export -o .beads/issues.jsonl") produced `unknown command "export"`.
	for _, absent := range []string{"bd export", "bd sync", "bd status"} {
		if strings.Contains(msg, absent) {
			t.Errorf("message suggests %q, which is not a bd-lite command:\n%s", absent, msg)
		}
	}
}

// Pins the bd-lite behavior the message asserts. If init ever learns to repair a
// half-built .beads, this fails and the wording should be revisited.
func TestMissingStoreRecovery_Integration(t *testing.T) {
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

	// The advice the ticket originally proposed, and why it is not in the message.
	initOut, err := exec.Command("bd", "init").CombinedOutput()
	if err == nil {
		t.Errorf("bd init succeeded in an existing .beads; the message's claim is now stale: %s", initOut)
	}
	if _, err := os.Stat(jsonlPath); err == nil {
		t.Error("bd init created issues.jsonl; the message should recommend it again")
	}

	// The advice the message actually gives.
	if out, err := exec.Command("bd", "create", "First issue").CombinedOutput(); err != nil {
		t.Fatalf("bd create failed: %v: %s", err, out)
	}
	if _, err := os.Stat(jsonlPath); err != nil {
		t.Errorf("bd create did not produce %s: %v", jsonlPath, err)
	}
}
