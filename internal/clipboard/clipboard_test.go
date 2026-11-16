package clipboard

import (
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type call struct {
	argv  []string
	env   []string
	input string
}

// recorder is a runner that records every invocation and delegates the outcome
// to handle, so a test can script per-command results.
type recorder struct {
	calls  []call
	handle func(argv, extraEnv []string, input string) (string, string, error)
}

func (r *recorder) run(argv, extraEnv []string, input string) (string, string, error) {
	r.calls = append(r.calls, call{argv: argv, env: extraEnv, input: input})
	if r.handle == nil {
		return "", "", nil
	}
	return r.handle(argv, extraEnv, input)
}

func (r *recorder) callsTo(name string) []call {
	var out []call
	for _, c := range r.calls {
		if c.argv[0] == name {
			out = append(out, c)
		}
	}
	return out
}

// newTestCopier builds a Copier with no terminal, so the OSC 52 fallback fails
// at the first step and these tests stay about the display helpers. The tests
// in osc52_test.go supply a terminal.
func newTestCopier(env map[string]string, tools []string, r *recorder) *Copier {
	c := newTestCopierWithTTY(env, tools, r, nil)
	c.openTTY = func() (io.WriteCloser, error) { return nil, errors.New("no such device or address") }
	return c
}

// newTestCopierWithTTY gives the Copier a terminal to write to, captured in tty.
func newTestCopierWithTTY(env map[string]string, tools []string, r *recorder, tty *fakeTTY) *Copier {
	installed := make(map[string]bool, len(tools))
	for _, t := range tools {
		installed[t] = true
	}
	return &Copier{
		getenv: func(k string) string { return env[k] },
		lookPath: func(name string) (string, error) {
			if installed[name] {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		run:     r.run,
		openTTY: func() (io.WriteCloser, error) { return tty, nil },
	}
}

// fakeTTY stands in for /dev/tty and records every Write separately, so a test
// can tell one write of a whole sequence from several partial ones.
type fakeTTY struct {
	writes [][]byte
	closed bool
}

func (f *fakeTTY) Write(p []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), p...))
	return len(p), nil
}

func (f *fakeTTY) Close() error {
	f.closed = true
	return nil
}

func (f *fakeTTY) written() []byte {
	var out []byte
	for _, w := range f.writes {
		out = append(out, w...)
	}
	return out
}

// tmuxResponder answers `tmux show-environment NAME` from a map, using tmux's
// "-NAME" convention for variables it knows to be unset.
func tmuxResponder(vars map[string]string, then func(argv, extraEnv []string, input string) (string, string, error)) func(argv, extraEnv []string, input string) (string, string, error) {
	return func(argv, extraEnv []string, input string) (string, string, error) {
		if argv[0] == "tmux" {
			name := argv[len(argv)-1]
			if v, ok := vars[name]; ok {
				return name + "=" + v + "\n", "", nil
			}
			return "-" + name + "\n", "", nil
		}
		if then == nil {
			return "", "", nil
		}
		return then(argv, extraEnv, input)
	}
}

func TestCopyPrefersWaylandHelper(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":1"},
		[]string{"wl-copy", "xclip"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if res.Helper != "wl-copy" {
		t.Errorf("Helper = %q, want wl-copy", res.Helper)
	}
	if got := r.callsTo("wl-copy"); len(got) != 1 || got[0].input != "tui-abc" {
		t.Errorf("wl-copy calls = %+v, want one call with input tui-abc", got)
	}
}

func TestCopyUsesXclipUnderX11(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"DISPLAY": ":1"}, []string{"xclip"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if res.Helper != "xclip" {
		t.Errorf("Helper = %q, want xclip", res.Helper)
	}
	if res.RecoveredFromTmux {
		t.Error("RecoveredFromTmux = true, want false when the process has a display")
	}
	got := r.callsTo("xclip")[0]
	if strings.Join(got.argv, " ") != "xclip -in -selection clipboard" {
		t.Errorf("argv = %v", got.argv)
	}
}

func TestCopyFallsBackToXsel(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"DISPLAY": ":1"}, []string{"xsel"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if res.Helper != "xsel" {
		t.Errorf("Helper = %q, want xsel", res.Helper)
	}
}

// The regression this package exists for: a tmux pane born while a client was
// attached over SSH has no DISPLAY, but the tmux session still knows it.
func TestCopyRecoversDisplayFromTmuxWhenProcessHasNone(t *testing.T) {
	r := &recorder{}
	r.handle = tmuxResponder(map[string]string{
		"DISPLAY":    ":1",
		"XAUTHORITY": "/run/user/4363/gdm/Xauthority",
	}, nil)
	c := newTestCopier(map[string]string{"TMUX": "/tmp/tmux-4363/default,22425,3"},
		[]string{"xclip"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if !res.RecoveredFromTmux {
		t.Error("RecoveredFromTmux = false, want true")
	}
	env := strings.Join(r.callsTo("xclip")[0].env, " ")
	if !strings.Contains(env, "DISPLAY=:1") {
		t.Errorf("xclip env = %q, want it to carry DISPLAY=:1", env)
	}
	if !strings.Contains(env, "XAUTHORITY=/run/user/4363/gdm/Xauthority") {
		t.Errorf("xclip env = %q, want it to carry XAUTHORITY", env)
	}
}

// A pane can also hold a DISPLAY value that has gone stale, which fails at the
// helper rather than at selection time.
func TestCopyRetriesWithTmuxDisplayWhenProcessDisplayIsStale(t *testing.T) {
	r := &recorder{}
	r.handle = tmuxResponder(map[string]string{"DISPLAY": ":1"},
		func(argv, extraEnv []string, input string) (string, string, error) {
			for _, e := range extraEnv {
				if e == "DISPLAY=:1" {
					return "", "", nil
				}
			}
			return "", "Error: Can't open display: :0\n", errors.New("exit status 1")
		})
	c := newTestCopier(map[string]string{"DISPLAY": ":0", "TMUX": "/tmp/tmux-4363/default,22425,3"},
		[]string{"xclip"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if !res.RecoveredFromTmux {
		t.Error("RecoveredFromTmux = false, want true after retry")
	}
	if n := len(r.callsTo("xclip")); n != 2 {
		t.Errorf("xclip invocations = %d, want 2 (initial failure then retry)", n)
	}
}

func TestCopySurfacesHelperStderrRatherThanExitStatus(t *testing.T) {
	r := &recorder{}
	r.handle = func(argv, extraEnv []string, input string) (string, string, error) {
		return "", "Error: Can't open display: (null)\n", errors.New("exit status 1")
	}
	c := newTestCopier(map[string]string{"DISPLAY": ":1"}, []string{"xclip"}, r)

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "Can't open display") {
		t.Errorf("error = %q, want the helper's own diagnostic", err)
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("error = %q, should not fall back to the bare exit status", err)
	}
}

func TestCopyWithoutDisplayInTmuxExplainsRecovery(t *testing.T) {
	r := &recorder{}
	r.handle = tmuxResponder(nil, nil) // tmux reports every variable unset
	c := newTestCopier(map[string]string{"TMUX": "/tmp/tmux-4363/default,22425,3"},
		[]string{"xclip"}, r)

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "tmux show-environment -s") {
		t.Errorf("error = %q, want it to name the recovery command", err)
	}
}

func TestCopyWithoutDisplayOutsideTmux(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{}, []string{"xclip"}, r)

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "DISPLAY") {
		t.Errorf("error = %q, want it to name DISPLAY", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("ran %+v, want no helper invocation without a display", r.calls)
	}
}

func TestCopyWithoutAnyHelperInstalled(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"DISPLAY": ":1"}, nil, r)

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "xclip") {
		t.Errorf("error = %q, want it to name the helpers to install", err)
	}
}

// Route selection. A display helper writes to this host's clipboard; OSC 52
// writes to the clipboard of whoever is sitting at the terminal. Which of those
// the user wants is not something the process can work out for itself, so the
// choice is BD_TUI_CLIPBOARD's and the fallback only covers the case where
// there is no local display at all.

func TestCopyViaHelperIsConfirmed(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"DISPLAY": ":1"}, []string{"xclip"}, r)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if !res.Confirmed {
		t.Error("Confirmed = false; a clean exit from xclip is a real acknowledgement")
	}
}

func TestCopyFallsBackToOSC52WhenThereIsNoDisplay(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	c := newTestCopierWithTTY(map[string]string{}, []string{"xclip"}, r, tty)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if res.Helper != "OSC 52" {
		t.Errorf("Helper = %q, want OSC 52", res.Helper)
	}
	if res.Confirmed {
		t.Error("Confirmed = true; the OSC 52 fallback cannot be observed and must not claim success")
	}
	if len(tty.writes) != 1 {
		t.Errorf("writes = %d, want the sequence sent to the terminal", len(tty.writes))
	}
}

func TestCopyInHelperModeDoesNotReachForTheTerminal(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	c := newTestCopierWithTTY(map[string]string{"BD_TUI_CLIPBOARD": "helper"}, []string{"xclip"}, r, tty)

	if _, err := c.Copy("tui-abc"); err == nil {
		t.Fatal("Copy() error = nil, want the helper failure to stand")
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want no OSC 52 fallback in helper mode", len(tty.writes))
	}
}

// The motivating case: an SSH session onto a workstation that has a display of
// its own. xclip would succeed and put the text on a clipboard nobody is
// looking at, so osc52 mode must skip it outright rather than fall back to it.
func TestCopyInOSC52ModeSkipsTheHelperEvenWithADisplay(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	c := newTestCopierWithTTY(map[string]string{
		"BD_TUI_CLIPBOARD": "osc52",
		"DISPLAY":          ":0",
	}, []string{"xclip"}, r, tty)

	res, err := c.Copy("tui-abc")
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if got := r.callsTo("xclip"); len(got) != 0 {
		t.Errorf("ran %+v, want the local display helper skipped", got)
	}
	if res.Confirmed {
		t.Error("Confirmed = true, want false")
	}
	if len(tty.writes) != 1 {
		t.Errorf("writes = %d, want the sequence sent", len(tty.writes))
	}
}

func TestCopyReportsBothFailuresWhenNeitherRouteWorks(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{}, []string{"xclip"}, r) // no display, no terminal

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "DISPLAY") {
		t.Errorf("error = %q, want the helper diagnosis to lead", err)
	}
	if !strings.Contains(err.Error(), "OSC 52") {
		t.Errorf("error = %q, want it to say the OSC 52 fallback was tried too", err)
	}
}

func TestCopyRejectsAnUnreadableModeSetting(t *testing.T) {
	r := &recorder{}
	c := newTestCopier(map[string]string{"BD_TUI_CLIPBOARD": "osc-52", "DISPLAY": ":1"},
		[]string{"xclip"}, r)

	_, err := c.Copy("tui-abc")
	if err == nil {
		t.Fatal("Copy() error = nil; a misspelled setting must not silently mean auto")
	}
	if len(r.calls) != 0 {
		t.Errorf("ran %+v, want nothing attempted with an unreadable setting", r.calls)
	}
}

func TestCopyRejectsAnUnreadableTerminatorSetting(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	c := newTestCopierWithTTY(map[string]string{
		"BD_TUI_CLIPBOARD":        "osc52",
		"BD_TUI_OSC52_TERMINATOR": "escape",
	}, nil, r, tty)

	if _, err := c.Copy("tui-abc"); err == nil {
		t.Fatal("Copy() error = nil, want the bad terminator rejected")
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want nothing sent", len(tty.writes))
	}
}

func TestTmuxEnvIgnoresUnsetMarker(t *testing.T) {
	r := &recorder{}
	r.handle = tmuxResponder(map[string]string{"DISPLAY": ":1"}, nil)
	c := newTestCopier(map[string]string{"TMUX": "x"}, []string{"xclip"}, r)

	if got := c.tmuxEnv("DISPLAY"); got != ":1" {
		t.Errorf("tmuxEnv(DISPLAY) = %q, want :1", got)
	}
	// tmux prints "-WAYLAND_DISPLAY" for a variable it knows to be unset.
	if got := c.tmuxEnv("WAYLAND_DISPLAY"); got != "" {
		t.Errorf("tmuxEnv(WAYLAND_DISPLAY) = %q, want empty", got)
	}
}

// runCommand must capture both streams and report the exit error, without
// deadlocking on a child that keeps its descriptors open.
func TestRunCommandCapturesBothStreams(t *testing.T) {
	stdout, stderr, err := runCommand(
		[]string{"sh", "-c", "cat; echo to-out; echo to-err >&2; exit 3"}, nil, "fed-in")
	if err == nil {
		t.Fatal("err = nil, want the child's non-zero exit")
	}
	if !strings.Contains(stdout, "fed-in") || !strings.Contains(stdout, "to-out") {
		t.Errorf("stdout = %q, want stdin echoed plus to-out", stdout)
	}
	if !strings.Contains(stderr, "to-err") {
		t.Errorf("stderr = %q, want to-err", stderr)
	}
}

// A helper that daemonizes and holds its inherited descriptors (as xclip does
// to own the X selection) must not stall the copy.
func TestRunCommandDoesNotBlockOnDaemonizedChild(t *testing.T) {
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		// The backgrounded sleep inherits stdout/stderr and outlives the parent.
		_, _, err = runCommand([]string{"sh", "-c", "sleep 30 & exit 0"}, nil, "")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("runCommand blocked on a descriptor held by a daemonized child")
	}
	// The parent exits 0, so not waiting on the child must not read as failure.
	if err != nil {
		t.Errorf("runCommand: %v, want nil for a parent that exits 0", err)
	}
}
