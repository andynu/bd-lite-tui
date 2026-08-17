package clipboard

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// What is testable here is the emission: the exact bytes that leave the
// process, and the preconditions that decide whether they are worth sending.
// Whether a terminal on the far end of an SSH connection acts on them is not
// observable from a test process, so nothing below asserts that a clipboard was
// populated. See the package comment in osc52.go.

// The base64 constants are the real encodings, checked against base64(1):
//
//	printf 'tui-abc' | base64  ->  dHVpLWFiYw==
const (
	tuiABC64 = "dHVpLWFiYw=="
	// "tui-abc — dash" carries a multi-byte rune, so this covers UTF-8 input
	// surviving the encoder byte for byte.
	tuiDash64 = "dHVpLWFiYyDigJQgZGFzaA=="
)

func TestOSC52SequenceUses7BitStringTerminatorByDefault(t *testing.T) {
	got := osc52Sequence("tui-abc", terminatorST7)
	want := []byte("\x1b]52;c;" + tuiABC64 + "\x1b\\")
	if !bytes.Equal(got, want) {
		t.Fatalf("sequence = %q, want %q", got, want)
	}
	// Spelled out so a change to any single byte is visible in the diff.
	if got[0] != 0x1b || got[1] != ']' {
		t.Errorf("introducer = %q, want ESC ]", got[:2])
	}
	if tail := got[len(got)-2:]; tail[0] != 0x1b || tail[1] != '\\' {
		t.Errorf("terminator = % x, want 1b 5c", tail)
	}
}

func TestOSC52Sequence8BitStringTerminatorIsTheSingleByte9C(t *testing.T) {
	got := osc52Sequence("tui-abc", terminatorST8)
	want := append([]byte("\x1b]52;c;"+tuiABC64), 0x9c)
	if !bytes.Equal(got, want) {
		t.Fatalf("sequence = % x, want % x", got, want)
	}
	// The 8-bit form must be one byte, not the two-byte 7-bit form.
	if n := len(got) - len("\x1b]52;c;"+tuiABC64); n != 1 {
		t.Errorf("terminator length = %d bytes, want 1", n)
	}
	if bytes.HasSuffix(got, []byte{0x1b, '\\'}) {
		t.Error("8-bit form ended with ESC \\, which is the 7-bit form")
	}
}

func TestOSC52SequenceBELTerminator(t *testing.T) {
	got := osc52Sequence("tui-abc", terminatorBEL)
	want := append([]byte("\x1b]52;c;"+tuiABC64), 0x07)
	if !bytes.Equal(got, want) {
		t.Fatalf("sequence = % x, want % x", got, want)
	}
}

func TestOSC52SequenceEncodesMultibyteTextAndPads(t *testing.T) {
	got := osc52Sequence("tui-abc — dash", terminatorST7)
	want := []byte("\x1b]52;c;" + tuiDash64 + "\x1b\\")
	if !bytes.Equal(got, want) {
		t.Fatalf("sequence = %q, want %q", got, want)
	}
	// Padding must be present: a terminal decodes the payload as standard
	// base64, and an unpadded tail is a decode error there, not here.
	if !strings.Contains(string(got), "==\x1b\\") {
		t.Errorf("sequence = %q, want the base64 padding kept", got)
	}
}

func TestOSC52SequenceEncodesEmptyText(t *testing.T) {
	// An empty payload is how OSC 52 clears the clipboard, so it must still be
	// a well-formed sequence rather than nothing at all.
	got := osc52Sequence("", terminatorST7)
	want := []byte("\x1b]52;c;\x1b\\")
	if !bytes.Equal(got, want) {
		t.Fatalf("sequence = %q, want %q", got, want)
	}
}

func TestTmuxPassthroughDoublesEveryEscape(t *testing.T) {
	inner := osc52Sequence("tui-abc", terminatorST7) // ESC ] ... ESC \
	got := tmuxPassthrough(inner)
	want := []byte("\x1bPtmux;" + "\x1b\x1b]52;c;" + tuiABC64 + "\x1b\x1b\\" + "\x1b\\")
	if !bytes.Equal(got, want) {
		t.Fatalf("passthrough = %q, want %q", got, want)
	}
	// The inner sequence's two ESCs become four, and the wrapper contributes
	// two of its own (the DCS introducer and its terminator): six in all.
	if n := bytes.Count(got, []byte{0x1b}); n != 6 {
		t.Errorf("ESC count = %d, want 6 (2 inner doubled to 4, plus the wrapper's 2)", n)
	}
}

func TestTmuxPassthroughLeavesBELPayloadAlone(t *testing.T) {
	// BEL is not ESC, so only the wrapper's own escapes appear.
	got := tmuxPassthrough(osc52Sequence("tui-abc", terminatorBEL))
	want := []byte("\x1bPtmux;" + "\x1b\x1b]52;c;" + tuiABC64 + "\x07" + "\x1b\\")
	if !bytes.Equal(got, want) {
		t.Fatalf("passthrough = %q, want %q", got, want)
	}
}

func TestParseTerminator(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want terminator
	}{
		{"", terminatorST7},
		{"st", terminatorST7},
		{"ST7", terminatorST7},
		{"st8", terminatorST8},
		{"8bit", terminatorST8},
		{" bel ", terminatorBEL},
	} {
		got, err := parseTerminator(tc.in)
		if err != nil {
			t.Errorf("parseTerminator(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseTerminator(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseTerminator("esc"); err == nil {
		t.Error("parseTerminator(esc) error = nil, want a rejection rather than a silent default")
	}
}

func TestParseMode(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Mode
	}{
		{"", ModeAuto},
		{"auto", ModeAuto},
		{"helper", ModeHelper},
		{" OSC52 ", ModeOSC52},
	} {
		got, err := parseMode(tc.in)
		if err != nil {
			t.Errorf("parseMode(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("parseMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseMode("osc-52"); err == nil {
		t.Error("parseMode(osc-52) error = nil, want a rejection rather than a silent default")
	}
}

// osc52TmuxResponder answers the three tmux queries the sender makes.
func osc52TmuxResponder(clientTTYs []string, passthrough, setClipboard, features string) func(argv, extraEnv []string, input string) (string, string, error) {
	return func(argv, extraEnv []string, input string) (string, string, error) {
		if argv[0] != "tmux" {
			return "", "", nil
		}
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "list-clients"):
			return strings.Join(clientTTYs, "\n") + "\n", "", nil
		case strings.HasSuffix(joined, "allow-passthrough"):
			return passthrough + "\n", "", nil
		case strings.HasSuffix(joined, "set-clipboard"):
			return setClipboard + "\n", "", nil
		case strings.Contains(joined, "client_termfeatures"):
			return features + "\n", "", nil
		}
		return "", "", nil
	}
}

func newTestSender(env map[string]string, r *recorder, tty *fakeTTY) *osc52Sender {
	return &osc52Sender{
		getenv:     func(k string) string { return env[k] },
		run:        r.run,
		openTTY:    func() (io.WriteCloser, error) { return tty, nil },
		terminator: terminatorST7,
	}
}

func TestSendOutsideTmuxWritesThePlainSequenceOnce(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{}, r, tty)

	res, err := s.send("tui-abc")
	if err != nil {
		t.Fatalf("send() error = %v", err)
	}
	if res.Confirmed {
		t.Error("Confirmed = true; OSC 52 is never acknowledged, so it must report unconfirmed")
	}
	if res.Helper != "OSC 52" {
		t.Errorf("Helper = %q, want OSC 52", res.Helper)
	}
	if len(r.calls) != 0 {
		t.Errorf("ran %+v, want no tmux queries outside tmux", r.calls)
	}
	want := []byte("\x1b]52;c;" + tuiABC64 + "\x1b\\")
	if !bytes.Equal(tty.written(), want) {
		t.Errorf("wrote %q, want %q", tty.written(), want)
	}
	// tcell writes to the same terminal, so the sequence must not be split
	// across writes where its output could land in the middle.
	if len(tty.writes) != 1 {
		t.Errorf("writes = %d, want the whole sequence in one write", len(tty.writes))
	}
	if !tty.closed {
		t.Error("the terminal was left open")
	}
}

func TestSendHonoursTheConfiguredTerminator(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{}, r, tty)
	s.terminator = terminatorST8

	if _, err := s.send("tui-abc"); err != nil {
		t.Fatalf("send() error = %v", err)
	}
	if got := tty.written(); got[len(got)-1] != 0x9c {
		t.Errorf("last byte = %#x, want 0x9c", got[len(got)-1])
	}
}

func TestSendWithoutATerminalFails(t *testing.T) {
	r := &recorder{}
	s := newTestSender(map[string]string{}, r, nil)
	s.openTTY = func() (io.WriteCloser, error) { return nil, errors.New("no such device or address") }

	_, err := s.send("tui-abc")
	if err == nil {
		t.Fatal("send() error = nil, want a failure when there is no terminal")
	}
	if !strings.Contains(err.Error(), "no terminal") {
		t.Errorf("error = %q, want it to name the missing terminal", err)
	}
}

func TestSendRefusesOversizedPayload(t *testing.T) {
	r := &recorder{}
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{}, r, tty)

	_, err := s.send(strings.Repeat("x", maxOSC52Text+1))
	if err == nil {
		t.Fatal("send() error = nil; a payload a terminal will truncate must be refused, not sent")
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want nothing sent", len(tty.writes))
	}
}

// The check the ticket exists for: tmux can have an attached client with no
// terminal behind it, and then the sequence has nowhere to arrive. Exit status
// from tmux would not reveal that.
func TestSendRefusesWhenNoTmuxClientHasATTY(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"", ""}, "on", "on", "clipboard")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "/tmp/tmux-4363/default,22425,3"}, r, tty)

	_, err := s.send("tui-abc")
	if err == nil {
		t.Fatal("send() error = nil, want a refusal when no client has a tty")
	}
	if !strings.Contains(err.Error(), "no attached client with a terminal") {
		t.Errorf("error = %q, want it to name the empty client_tty", err)
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want nothing sent", len(tty.writes))
	}
}

func TestSendAcceptsATmuxClientWithATTYAmongEmptyOnes(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"", "/dev/pts/7"}, "on", "on", "clipboard")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	if _, err := s.send("tui-abc"); err != nil {
		t.Fatalf("send() error = %v, want the client with a tty to count", err)
	}
	if len(tty.writes) != 1 {
		t.Errorf("writes = %d, want the sequence sent", len(tty.writes))
	}
}

func TestSendUsesPassthroughWhenTmuxAllowsIt(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "on", "off", "")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	res, err := s.send("tui-abc")
	if err != nil {
		t.Fatalf("send() error = %v", err)
	}
	want := tmuxPassthrough(osc52Sequence("tui-abc", terminatorST7))
	if !bytes.Equal(tty.written(), want) {
		t.Errorf("wrote %q, want the passthrough-wrapped sequence %q", tty.written(), want)
	}
	if !strings.Contains(res.Helper, "passthrough") {
		t.Errorf("Helper = %q, want it to name the passthrough route", res.Helper)
	}
	if res.Confirmed {
		t.Error("Confirmed = true, want false")
	}
}

func TestSendUsesPassthroughWhenSetToAll(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "all", "off", "")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	if _, err := s.send("tui-abc"); err != nil {
		t.Fatalf("send() error = %v", err)
	}
	if !bytes.HasPrefix(tty.written(), []byte("\x1bPtmux;")) {
		t.Errorf("wrote %q, want a passthrough wrapper", tty.written())
	}
}

func TestSendFallsBackToTmuxOwnClipboardHandling(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "off", "external", "256,clipboard,RGB")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	res, err := s.send("tui-abc")
	if err != nil {
		t.Fatalf("send() error = %v", err)
	}
	want := osc52Sequence("tui-abc", terminatorST7)
	if !bytes.Equal(tty.written(), want) {
		t.Errorf("wrote %q, want the bare sequence for tmux to relay %q", tty.written(), want)
	}
	if res.Helper != "OSC 52 via tmux" {
		t.Errorf("Helper = %q, want OSC 52 via tmux", res.Helper)
	}
}

func TestSendRefusesWhenTmuxWouldSwallowTheSequence(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "off", "off", "256,clipboard")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	_, err := s.send("tui-abc")
	if err == nil {
		t.Fatal("send() error = nil, want a refusal when tmux discards the sequence")
	}
	if !strings.Contains(err.Error(), "set-clipboard") {
		t.Errorf("error = %q, want it to name the option to change", err)
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want nothing sent", len(tty.writes))
	}
}

// tmux relays a clipboard set outward only when the client's terminfo carries
// Ms, reported as the "clipboard" feature. Without it the text stops at the
// tmux buffer, which is the silent half-success this refuses.
func TestSendRefusesWhenTmuxClientCannotForwardClipboard(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "off", "on", "256,RGB,mouse")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	_, err := s.send("tui-abc")
	if err == nil {
		t.Fatal("send() error = nil, want a refusal when the client cannot forward")
	}
	if !strings.Contains(err.Error(), "terminal-features") {
		t.Errorf("error = %q, want it to name the fix", err)
	}
	if len(tty.writes) != 0 {
		t.Errorf("wrote %d times, want nothing sent", len(tty.writes))
	}
}

// An unknown feature list means we could not tell, which is not the same as
// knowing the feature is missing, so the sequence still goes.
func TestSendProceedsWhenTmuxReportsNoFeatureList(t *testing.T) {
	r := &recorder{}
	r.handle = osc52TmuxResponder([]string{"/dev/pts/7"}, "off", "on", "")
	tty := &fakeTTY{}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, tty)

	if _, err := s.send("tui-abc"); err != nil {
		t.Fatalf("send() error = %v, want an unknown feature list to be non-fatal", err)
	}
	if len(tty.writes) != 1 {
		t.Errorf("writes = %d, want the sequence sent", len(tty.writes))
	}
}

func TestAttachedClientTTYSurfacesTmuxStderr(t *testing.T) {
	r := &recorder{}
	r.handle = func(argv, extraEnv []string, input string) (string, string, error) {
		return "", "no server running on /tmp/tmux-4363/default\n", errors.New("exit status 1")
	}
	s := newTestSender(map[string]string{"TMUX": "x"}, r, &fakeTTY{})

	_, err := s.send("tui-abc")
	if err == nil {
		t.Fatal("send() error = nil, want the tmux failure")
	}
	if !strings.Contains(err.Error(), "no server running") {
		t.Errorf("error = %q, want tmux's own diagnostic", err)
	}
}
