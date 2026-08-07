package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// These tests drive the real clipboard helpers against the live display and
// therefore replace whatever is currently on the system clipboard. They only
// run on request:
//
//	BD_TUI_CLIPBOARD_INTEGRATION=1 go test ./internal/clipboard/ -run EndToEnd
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("BD_TUI_CLIPBOARD_INTEGRATION") == "" {
		t.Skip("set BD_TUI_CLIPBOARD_INTEGRATION=1 to run (replaces the system clipboard)")
	}
	if _, err := exec.LookPath("xclip"); err != nil {
		t.Skip("xclip not installed")
	}
}

// readClipboard reads the clipboard back through xclip, using the display the
// Copier would have resolved, so it works even when the test process has none.
func readClipboard(t *testing.T, d display) string {
	t.Helper()
	cmd := exec.Command("xclip", "-out", "-selection", "clipboard")
	cmd.Env = append(os.Environ(), d.env()...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reading clipboard back: %v", err)
	}
	return string(out)
}

func TestCopyEndToEnd(t *testing.T) {
	requireIntegration(t)

	c := New()
	if !c.resolveDisplay().present() {
		t.Skip("no display available")
	}
	want := fmt.Sprintf("bd-tui-e2e-%d", time.Now().UnixNano())

	res, err := c.Copy(want)
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := readClipboard(t, c.resolveDisplay()); got != want {
		t.Errorf("clipboard = %q, want %q (helper %s)", got, want, res.Helper)
	}
}

// The regression: a tmux pane whose environment carries no display at all.
// Before the fix this produced a bare "exit status 1" from xclip.
func TestCopyEndToEndRecoversDisplayInPaneWithoutOne(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("TMUX") == "" {
		t.Skip("not running inside tmux")
	}

	c := New()
	real := c.getenv
	c.getenv = func(k string) string {
		switch k {
		case "DISPLAY", "WAYLAND_DISPLAY", "XAUTHORITY":
			return "" // as if the pane were born without a display
		default:
			return real(k)
		}
	}

	want := fmt.Sprintf("bd-tui-e2e-recovered-%d", time.Now().UnixNano())
	res, err := c.Copy(want)
	if err != nil {
		t.Fatalf("Copy() error = %v, want recovery via the tmux session environment", err)
	}
	if !res.RecoveredFromTmux {
		t.Error("RecoveredFromTmux = false, want true")
	}
	if got := readClipboard(t, c.tmuxDisplay()); got != want {
		t.Errorf("clipboard = %q, want %q", got, want)
	}
}

// Outside tmux with no display there is nothing to recover, so the error must
// say so plainly instead of surfacing an exit code.
func TestCopyEndToEndWithoutDisplayReportsWhy(t *testing.T) {
	requireIntegration(t)

	c := New()
	c.getenv = func(string) string { return "" }

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("error = %q, want a diagnosis rather than an exit code", err)
	}
	t.Logf("reported: %v", err)
}
