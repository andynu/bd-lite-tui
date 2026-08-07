// Package clipboard copies text to the system clipboard.
//
// It replaces github.com/atotto/clipboard, which discards the helper's stderr
// and so reports "exit status 1" for every failure, and which cannot cope with
// a process whose environment has no DISPLAY. Both matter inside tmux: a pane
// keeps the environment it was born with, so a pane created while a client was
// attached over SSH has no DISPLAY even after the session is reattached
// locally. tmux still knows the current value, so we ask it rather than fail.
package clipboard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// commandTimeout bounds a single helper invocation. Clipboard helpers are
// local and instantaneous; anything slower is wedged and must not block the UI.
const commandTimeout = 5 * time.Second

// Result describes how a successful copy was carried out. Callers use it to
// report which helper ran and whether the display had to be recovered.
type Result struct {
	// Helper is the executable that accepted the text, e.g. "xclip".
	Helper string
	// RecoveredFromTmux reports that the display came from the tmux session
	// environment because the process environment had none (or had a stale one).
	RecoveredFromTmux bool
}

// runner executes argv with extraEnv appended to the current environment,
// feeding input on stdin. It returns the command's stdout and stderr.
type runner func(argv, extraEnv []string, input string) (stdout, stderr string, err error)

// Copier copies text to the system clipboard.
type Copier struct {
	getenv   func(string) string
	lookPath func(string) (string, error)
	run      runner
}

// New returns a Copier backed by the real process environment.
func New() *Copier {
	return &Copier{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		run:      runCommand,
	}
}

// display is the environment a clipboard helper needs to reach the display server.
type display struct {
	wayland    string
	x11        string
	xauthority string
	fromTmux   bool
}

func (d display) present() bool { return d.wayland != "" || d.x11 != "" }

// env renders the display as environment overrides for a helper process.
func (d display) env() []string {
	var env []string
	if d.wayland != "" {
		env = append(env, "WAYLAND_DISPLAY="+d.wayland)
	}
	if d.x11 != "" {
		env = append(env, "DISPLAY="+d.x11)
	}
	if d.xauthority != "" {
		env = append(env, "XAUTHORITY="+d.xauthority)
	}
	return env
}

// Copy writes text to the system clipboard.
//
// If the process has no display and we are inside tmux, the display is
// recovered from the tmux session environment. If a copy fails with the
// process display, it is retried once with tmux's, which covers a pane
// holding a DISPLAY value that has since gone stale.
func (c *Copier) Copy(text string) (Result, error) {
	d := c.resolveDisplay()

	res, err := c.copyWith(d, text)
	if err == nil {
		return res, nil
	}

	if !d.fromTmux && c.inTmux() {
		if recovered := c.tmuxDisplay(); recovered.present() {
			if res, retryErr := c.copyWith(recovered, text); retryErr == nil {
				return res, nil
			}
		}
	}
	return Result{}, err
}

// copyWith runs the best available helper for the given display.
func (c *Copier) copyWith(d display, text string) (Result, error) {
	argv, err := c.helper(d)
	if err != nil {
		return Result{}, err
	}
	if _, stderr, runErr := c.run(argv, d.env(), text); runErr != nil {
		return Result{}, helperError(argv[0], stderr, runErr)
	}
	return Result{Helper: argv[0], RecoveredFromTmux: d.fromTmux}, nil
}

// resolveDisplay reads the display from the process environment, falling back
// to the tmux session environment when the process has none.
func (c *Copier) resolveDisplay() display {
	d := display{
		wayland:    c.getenv("WAYLAND_DISPLAY"),
		x11:        c.getenv("DISPLAY"),
		xauthority: c.getenv("XAUTHORITY"),
	}
	if d.present() || !c.inTmux() {
		return d
	}
	if recovered := c.tmuxDisplay(); recovered.present() {
		return recovered
	}
	return d
}

func (c *Copier) inTmux() bool { return c.getenv("TMUX") != "" }

// tmuxDisplay reads the display out of the tmux session environment.
func (c *Copier) tmuxDisplay() display {
	d := display{
		wayland:    c.tmuxEnv("WAYLAND_DISPLAY"),
		x11:        c.tmuxEnv("DISPLAY"),
		xauthority: c.tmuxEnv("XAUTHORITY"),
	}
	d.fromTmux = d.present()
	return d
}

// tmuxEnv reads one variable from the tmux session environment. tmux prints
// "NAME=value", or "-NAME" when the variable is explicitly unset.
func (c *Copier) tmuxEnv(name string) string {
	stdout, _, err := c.run([]string{"tmux", "show-environment", name}, nil, "")
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(stdout)
	value, ok := strings.CutPrefix(line, name+"=")
	if !ok {
		return ""
	}
	return value
}

// helper picks the clipboard helper matching the available display.
func (c *Copier) helper(d display) ([]string, error) {
	if d.wayland != "" {
		if _, err := c.lookPath("wl-copy"); err == nil {
			return []string{"wl-copy"}, nil
		}
	}
	if d.x11 != "" {
		if _, err := c.lookPath("xclip"); err == nil {
			return []string{"xclip", "-in", "-selection", "clipboard"}, nil
		}
		if _, err := c.lookPath("xsel"); err == nil {
			return []string{"xsel", "--input", "--clipboard"}, nil
		}
	}
	if !d.present() {
		return nil, c.noDisplayError()
	}
	return nil, errors.New("no clipboard helper installed (need xclip, xsel, or wl-clipboard)")
}

// noDisplayError explains a missing display and, inside tmux, how to fix it.
func (c *Copier) noDisplayError() error {
	if c.inTmux() {
		return errors.New(`no display: DISPLAY is unset in this pane and tmux has none. ` +
			`This pane predates the current session; run: eval "$(tmux show-environment -s)"`)
	}
	return errors.New("no display: DISPLAY and WAYLAND_DISPLAY are both unset")
}

// helperError surfaces the helper's own diagnostic instead of a bare exit code.
func helperError(name, stderr string, err error) error {
	if msg := firstLine(stderr); msg != "" {
		return fmt.Errorf("%s: %s", name, msg)
	}
	return fmt.Errorf("%s: %w", name, err)
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// runCommand executes argv with input on stdin.
//
// stdout and stderr are captured through temporary files rather than pipes.
// xclip daemonizes to own the X selection and keeps the inherited descriptors
// open, so exec's pipe-draining goroutines would not finish until the
// clipboard was replaced, hanging the copy indefinitely. A regular file is
// handed to the child as a descriptor, so Wait returns as soon as the parent
// process exits.
func runCommand(argv, extraEnv []string, input string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	cmd.Stdin = strings.NewReader(input)

	stdout, cleanupOut, err := tempCapture("stdout")
	if err != nil {
		return "", "", err
	}
	defer cleanupOut()
	stderr, cleanupErr, err := tempCapture("stderr")
	if err != nil {
		return "", "", err
	}
	defer cleanupErr()
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", "", fmt.Errorf("%s timed out after %s", argv[0], commandTimeout)
	}
	return readAll(stdout), readAll(stderr), runErr
}

func tempCapture(suffix string) (*os.File, func(), error) {
	f, err := os.CreateTemp("", "bd-tui-clip-*."+suffix)
	if err != nil {
		return nil, nil, fmt.Errorf("capturing %s: %w", suffix, err)
	}
	return f, func() {
		name := f.Name()
		f.Close()
		os.Remove(name)
	}, nil
}

func readAll(f *os.File) string {
	if _, err := f.Seek(0, 0); err != nil {
		return ""
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return ""
	}
	return buf.String()
}
