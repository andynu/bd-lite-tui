package clipboard

// OSC 52 is the terminal's own clipboard protocol: the application writes an
// escape sequence carrying base64 text and the terminal emulator puts it on the
// clipboard of the machine the *user* is sitting at. That is the only route
// that works over a genuine SSH session, where the remote host either has no
// display at all or has one belonging to a screen nobody is looking at.
//
// Nothing about it is acknowledged. The terminal sends no reply and returns no
// status, so a successful write proves only that the bytes left this process.
// Every check below is therefore a precondition check: it rules out the cases
// where the bytes provably cannot arrive. What survives the checks is reported
// as unconfirmed, never as success.
//
// The sequence is written here rather than through tcell's Screen.SetClipboard,
// which is a silent no-op unless the terminfo entry carries the XT flag. Under
// TERM=tmux-256color it does not (verified against tcell v2.8.1, both the
// compiled and the infocmp-derived entry), so tcell would emit nothing and
// return no error.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxOSC52Text caps what we are willing to hand to a terminal. Terminals and
// tmux both cut long OSC 52 payloads off without saying so, and a silently
// truncated clipboard is the false success this whole path exists to avoid.
// bd-tui copies issue IDs, titles and branch names, so anything approaching
// this size is a bug rather than a copy.
const maxOSC52Text = 4096

// terminator selects how the OSC sequence is closed.
type terminator int

const (
	// terminatorST7 closes with ESC \, the 7-bit form of ECMA-48 ST. It is the
	// default because it is two ASCII bytes, so it survives a UTF-8 locale and
	// a terminal that refuses C1 controls.
	terminatorST7 terminator = iota
	// terminatorST8 closes with 0x9C, the 8-bit C1 ST. Only a terminal in
	// 8-bit-controls mode reads it as ST; under UTF-8 that byte is an illegal
	// continuation byte and the sequence never ends.
	terminatorST8
	// terminatorBEL closes with BEL (0x07), which xterm accepts for OSC and
	// which some older terminals accept only in that form.
	terminatorBEL
)

func (t terminator) bytes() []byte {
	switch t {
	case terminatorST8:
		return []byte{0x9c}
	case terminatorBEL:
		return []byte{0x07}
	default:
		return []byte{0x1b, '\\'}
	}
}

func (t terminator) String() string {
	switch t {
	case terminatorST8:
		return "st8"
	case terminatorBEL:
		return "bel"
	default:
		return "st"
	}
}

// parseTerminator reads the BD_TUI_OSC52_TERMINATOR value. An empty value is
// the default; an unrecognised one is an error rather than a silent fallback,
// because a typo would otherwise look like a working setting.
func parseTerminator(s string) (terminator, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "st", "st7":
		return terminatorST7, nil
	case "st8", "8bit":
		return terminatorST8, nil
	case "bel":
		return terminatorBEL, nil
	default:
		return terminatorST7, fmt.Errorf("BD_TUI_OSC52_TERMINATOR=%q: want st, st8, or bel", s)
	}
}

// osc52Sequence renders text as an OSC 52 clipboard-set sequence:
//
//	ESC ] 5 2 ; c ; <base64 of text> <terminator>
//
// The "c" selects the clipboard rather than the X primary selection, and the
// payload is standard base64 with padding and no line breaks.
func osc52Sequence(text string, t terminator) []byte {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := make([]byte, 0, len(encoded)+9)
	seq = append(seq, 0x1b, ']', '5', '2', ';', 'c', ';')
	seq = append(seq, encoded...)
	return append(seq, t.bytes()...)
}

// tmuxPassthrough wraps seq so tmux hands it to the outer terminal untouched
// instead of interpreting it:
//
//	ESC P t m u x ; <seq with every ESC doubled> ESC \
//
// tmux only honours this when allow-passthrough is on or all.
func tmuxPassthrough(seq []byte) []byte {
	out := make([]byte, 0, len(seq)+16)
	out = append(out, 0x1b, 'P', 't', 'm', 'u', 'x', ';')
	for _, b := range seq {
		if b == 0x1b {
			out = append(out, 0x1b)
		}
		out = append(out, b)
	}
	return append(out, 0x1b, '\\')
}

// osc52Sender writes an OSC 52 sequence to the terminal this process is
// attached to.
type osc52Sender struct {
	getenv     func(string) string
	run        runner
	openTTY    func() (io.WriteCloser, error)
	terminator terminator
}

// send writes the sequence and reports an unconfirmed Result. The error paths
// are the ones where the sequence provably could not have arrived; a nil error
// means only that the bytes were written to a terminal.
func (s *osc52Sender) send(text string) (Result, error) {
	if len(text) > maxOSC52Text {
		return Result{}, fmt.Errorf("OSC 52: %d bytes exceeds the %d-byte limit; "+
			"terminals truncate longer payloads without reporting it", len(text), maxOSC52Text)
	}

	seq := osc52Sequence(text, s.terminator)
	route := "OSC 52"
	if s.getenv("TMUX") != "" {
		wrapped, tmuxRoute, err := s.forTmux(seq)
		if err != nil {
			return Result{}, err
		}
		seq, route = wrapped, tmuxRoute
	}

	tty, err := s.openTTY()
	if err != nil {
		return Result{}, fmt.Errorf("OSC 52: no terminal to write to: %w", err)
	}
	defer tty.Close()

	// One Write for the whole sequence. tcell owns this terminal too, and a
	// sequence split across writes could be interleaved with its output.
	if _, err := tty.Write(seq); err != nil {
		return Result{}, fmt.Errorf("OSC 52: writing to the terminal: %w", err)
	}
	return Result{Helper: route}, nil
}

// forTmux adapts the sequence to the tmux in front of us, and refuses the cases
// where tmux would provably swallow it.
func (s *osc52Sender) forTmux(seq []byte) ([]byte, string, error) {
	if _, err := s.attachedClientTTY(); err != nil {
		return nil, "", err
	}

	// Passthrough is the stronger route: tmux forwards the bytes verbatim, so
	// its own clipboard settings and terminfo probing stop mattering.
	switch s.tmuxOption("allow-passthrough") {
	case "on", "all":
		return tmuxPassthrough(seq), "OSC 52 via tmux passthrough", nil
	}

	switch s.tmuxOption("set-clipboard") {
	case "on", "external":
		// tmux forwards a clipboard set outward only when the attached client's
		// terminfo carries Ms, which tmux reports as the "clipboard" feature.
		// Without it the sequence stops at the tmux buffer.
		if features := s.tmuxClientFeatures(); features != "" && !strings.Contains(features, "clipboard") {
			return nil, "", fmt.Errorf("tmux will not forward the clipboard: the attached client's "+
				"terminal (features: %s) has no Ms/clipboard capability. "+
				`Run: tmux set -ga terminal-features ",$TERM:clipboard"`, features)
		}
		return seq, "OSC 52 via tmux", nil
	}

	return nil, "", errors.New("tmux would discard the sequence: set-clipboard is off and " +
		"allow-passthrough is off. Run: tmux set -g set-clipboard on")
}

// attachedClientTTY returns the tty of an attached tmux client.
//
// This is the check the OSC 52 path exists for. `tmux load-buffer -w` exits 0
// even when no attached client can receive the forwarded sequence, so its exit
// status is not evidence of anything. A client with an empty client_tty (a
// control-mode client, or one whose terminal has gone) is a client the
// clipboard cannot reach.
func (s *osc52Sender) attachedClientTTY() (string, error) {
	stdout, stderr, err := s.run([]string{"tmux", "list-clients", "-F", "#{client_tty}"}, nil, "")
	if err != nil {
		if msg := firstLine(stderr); msg != "" {
			return "", fmt.Errorf("tmux list-clients: %s", msg)
		}
		return "", fmt.Errorf("tmux list-clients: %w", err)
	}
	for _, line := range strings.Split(stdout, "\n") {
		if tty := strings.TrimSpace(line); tty != "" {
			return tty, nil
		}
	}
	return "", errors.New("tmux has no attached client with a terminal, so an OSC 52 sequence " +
		"has nowhere to arrive; attach a client and copy again")
}

// tmuxOption reads a global tmux option. An empty answer means tmux could not
// tell us, which includes a tmux too old to have the option at all.
func (s *osc52Sender) tmuxOption(name string) string {
	stdout, _, err := s.run([]string{"tmux", "show-options", "-gv", name}, nil, "")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stdout)
}

// tmuxClientFeatures reports the attached client's terminal features. An empty
// answer means we could not tell, not that the feature is missing.
func (s *osc52Sender) tmuxClientFeatures() string {
	stdout, _, err := s.run([]string{"tmux", "display-message", "-p", "#{client_termfeatures}"}, nil, "")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stdout)
}

// openControllingTTY opens the process's controlling terminal for writing.
// Opening it is itself the test for whether there is a terminal at all: a
// process whose output is a pipe or a file has no /dev/tty to open.
func openControllingTTY() (io.WriteCloser, error) {
	return os.OpenFile("/dev/tty", os.O_WRONLY, 0)
}
