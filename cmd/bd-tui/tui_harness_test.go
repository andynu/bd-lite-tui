package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TestMain silences the application's diagnostic logging, which is wired to the
// standard logger and would otherwise print a line per keystroke.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	log.SetFlags(0)
	os.Exit(m.Run())
}

// barrierKey is a key the application binds to nothing. The harness intercepts
// it ahead of the app's own input capture and never forwards it, using it to
// learn when the event loop has finished everything queued before it.
//
// This works because tview drains a single ordered channel of screen events:
// when the barrier reaches the capture, every key queued earlier has already
// been through the capture, the focused primitive, and a draw.
const barrierKey = tcell.KeyF64

// harnessTimeout bounds every wait in the harness so a wedged event loop fails
// the test instead of hanging the suite.
const harnessTimeout = 5 * time.Second

// bdCall is one recorded invocation of the (faked) bd CLI.
type bdCall struct {
	Args []string
}

// String renders the call the way a user would have typed it.
func (c bdCall) String() string { return "bd " + strings.Join(c.Args, " ") }

// harness drives a real tui against a tcell SimulationScreen.
//
// Everything the tests touch goes through two primitives: press, which feeds
// keys into the same channel a terminal would and blocks until they have been
// handled, and read, which runs a closure on the tview event loop goroutine so
// widget state can be inspected without racing the draw.
type harness struct {
	t *testing.T

	tui    *tui
	screen tcell.SimulationScreen

	beadsDir  string
	jsonlPath string

	// issues is the fixture backing issues.jsonl. Tests mutate it and call
	// writeJSONL to simulate bd (or another process) rewriting the file.
	issues []*parser.Issue

	// bdCalls records what the faked bd CLI was asked to do.
	bdMu    sync.Mutex
	bdCalls []bdCall
	// bdRespond, when set, overrides the default fake bd behavior.
	bdRespond func(args []string) (*BdCommandResult, error)

	mu      sync.Mutex
	barrier chan struct{}

	done chan error
}

// harnessConfig is everything the options adjust before the app is built: the
// tuiOptions themselves, plus the identity the app should resolve.
type harnessConfig struct {
	tui tuiOptions

	// identity is what resolveIdentity returns for the app under test. It
	// defaults to "" so no test inherits the developer's git user.name.
	identity string

	// width and height size the simulation screen.
	width, height int
}

// harnessOption tweaks the configuration before the app is built.
type harnessOption func(*harnessConfig)

// withTreeView starts the app in tree view.
func withTreeView() harnessOption {
	return func(c *harnessConfig) { c.tui.TreeView = true }
}

// withWatcher enables the filesystem watcher, which is off by default so most
// tests are not at the mercy of fsnotify timing.
func withWatcher() harnessOption {
	return func(c *harnessConfig) { c.tui.NoWatcher = false }
}

// withIssueFilter starts the app with the --issue single-issue restriction.
func withIssueFilter(id string) harnessOption {
	return func(c *harnessConfig) { c.tui.IssueID = id }
}

// withIdentity pins git user.name for the app under test. Tests that care about
// assignees have to ask for an identity, so the rest keep asserting against the
// unresolved-identity case rather than against whatever the developer's git
// config happens to say.
func withIdentity(name string) harnessOption {
	return func(c *harnessConfig) { c.identity = name }
}

// withScreenSize widens (or narrows) the simulation screen. The default 120
// columns leaves the list pane around 38 wide, which truncates long rows before
// they reach the screen; a test asserting on the tail of a row needs more room.
func withScreenSize(width, height int) harnessOption {
	return func(c *harnessConfig) {
		c.width = width
		c.height = height
	}
}

// newHarness writes issues to a temporary .beads directory, builds the app
// against a 120x40 simulation screen, and starts its event loop. The app is
// stopped and its watcher released when the test ends.
func newHarness(t *testing.T, issues []*parser.Issue, opts ...harnessOption) *harness {
	t.Helper()

	beadsDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("creating .beads dir: %v", err)
	}

	h := &harness{
		t:         t,
		beadsDir:  beadsDir,
		jsonlPath: filepath.Join(beadsDir, "issues.jsonl"),
		issues:    issues,
		done:      make(chan error, 1),
	}
	h.writeJSONL()

	// Route every bd invocation into the fake. Restored when the test ends.
	realExec := execBdJSON
	execBdJSON = h.fakeBd
	t.Cleanup(func() { execBdJSON = realExec })

	h.screen = tcell.NewSimulationScreen("UTF-8")

	cfg := harnessConfig{
		tui: tuiOptions{
			BeadsDir:  beadsDir,
			JSONLPath: h.jsonlPath,
			Screen:    h.screen,
			NoWatcher: true,
		},
		width:  120,
		height: 40,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Pin the identity so nothing consults the developer's git config.
	realIdentity := resolveIdentity
	resolveIdentity = func(string) string { return cfg.identity }
	t.Cleanup(func() { resolveIdentity = realIdentity })

	app, err := newTUI(cfg.tui)
	if err != nil {
		t.Fatalf("newTUI: %v", err)
	}
	h.tui = app

	// SetScreen already ran Init, which forces 80x25. Widen past
	// layout.NarrowThreshold so the default resolves to the columns split
	// rather than the narrow paged layout.
	h.screen.SetSize(cfg.width, cfg.height)

	// Intercept the barrier key ahead of the application's own bindings.
	appCapture := app.app.GetInputCapture()
	app.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == barrierKey {
			h.mu.Lock()
			ch := h.barrier
			h.barrier = nil
			h.mu.Unlock()
			if ch != nil {
				close(ch)
			}
			return nil
		}
		return appCapture(event)
	})

	go func() { h.done <- app.Run() }()

	t.Cleanup(func() {
		app.app.QueueUpdate(func() { app.app.Stop() })
		select {
		case <-h.done:
		case <-time.After(harnessTimeout):
			t.Error("timed out waiting for the tview event loop to stop")
		}
		app.Close()
	})

	// A round trip through the event loop proves it is running and that the
	// first draw is done.
	h.press()

	return h
}

// press feeds keys into the event loop in order and returns once the last one
// has been fully handled.
func (h *harness) press(events ...*tcell.EventKey) {
	h.t.Helper()

	ch := make(chan struct{})
	h.mu.Lock()
	h.barrier = ch
	h.mu.Unlock()

	for _, event := range events {
		h.tui.app.QueueEvent(event)
	}
	h.tui.app.QueueEvent(tcell.NewEventKey(barrierKey, 0, tcell.ModNone))

	select {
	case <-ch:
	case <-time.After(harnessTimeout):
		h.t.Fatalf("timed out waiting for the event loop to handle %d key(s)", len(events))
	}
}

// clickAt injects a left click at a screen cell and returns once the event loop
// has handled it. tview derives MouseLeftClick from a press and a release at the
// same position, so both events are queued; the barrier key behind them is what
// makes the call synchronous, exactly as in press.
//
// One click per harness: tview turns a second click within its
// DoubleClickInterval into MouseLeftDoubleClick, which the app does not bind.
func (h *harness) clickAt(x, y int) {
	h.t.Helper()

	ch := make(chan struct{})
	h.mu.Lock()
	h.barrier = ch
	h.mu.Unlock()

	h.tui.app.QueueEvent(tcell.NewEventMouse(x, y, tcell.ButtonPrimary, tcell.ModNone))
	h.tui.app.QueueEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
	h.tui.app.QueueEvent(tcell.NewEventKey(barrierKey, 0, tcell.ModNone))

	select {
	case <-ch:
	case <-time.After(harnessTimeout):
		h.t.Fatalf("timed out waiting for the event loop to handle a click at %d,%d", x, y)
	}
}

// clickRowContaining left-clicks the first rendered screen row containing want.
// Locating the row on screen rather than computing it from borders and offsets
// keeps the test honest about what the user is actually pointing at.
func (h *harness) clickRowContaining(want string) {
	h.t.Helper()
	for y, line := range h.screenLines() {
		if strings.Contains(line, want) {
			h.clickAt(2, y)
			return
		}
	}
	h.t.Fatalf("no rendered row contains %q:\n%s", want, h.screenText())
}

// typeRunes presses each rune of s in turn, the way a user types.
func (h *harness) typeRunes(s string) {
	h.t.Helper()
	events := make([]*tcell.EventKey, 0, len(s))
	for _, r := range s {
		events = append(events, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.press(events...)
}

// key presses a single non-rune key.
func (h *harness) key(k tcell.Key) {
	h.t.Helper()
	h.press(tcell.NewEventKey(k, 0, tcell.ModNone))
}

// read runs fn on the tview event loop goroutine, so widget state can be
// inspected without racing a draw. QueueUpdate returns only after fn has run,
// so callers can read values fn assigned.
func (h *harness) read(fn func()) {
	h.tui.app.QueueUpdate(fn)
}

// selectIssue moves the cursor onto the row for id. Tests that care about
// acting on a particular issue use this instead of counting j presses, which
// would break whenever the fixture or the grouping changes.
func (h *harness) selectIssue(id string) {
	h.t.Helper()

	var found bool
	h.read(func() {
		for i := 0; i < h.tui.issueList.GetItemCount(); i++ {
			if issue, ok := h.tui.indexToIssue[i]; ok && issue.ID == id {
				h.tui.issueList.SetCurrentItem(i)
				found = true
				return
			}
		}
	})
	if !found {
		h.t.Fatalf("no row for issue %q; rows are %v", id, h.issueIDs())
	}
	h.press() // let the selection change settle through the event loop
}

// pressButton tabs through the form on the front page until the button whose
// label contains label has focus, then activates it. Matching on the label
// rather than a tab count keeps the test readable and order-independent.
func (h *harness) pressButton(label string) {
	h.t.Helper()

	// One pass per focusable element, with headroom, so a form that never
	// focuses the button fails rather than looping forever.
	for i := 0; i < 32; i++ {
		var focused string
		h.read(func() {
			if button, ok := h.tui.app.GetFocus().(*tview.Button); ok {
				focused = button.GetLabel()
			}
		})
		if focused != "" && strings.Contains(focused, label) {
			h.key(tcell.KeyEnter)
			return
		}
		h.key(tcell.KeyTab)
	}
	h.t.Fatalf("no form button matching %q after tabbing through the form", label)
}

// rowContaining returns the rendered screen line containing want, failing the
// test when no line does.
func rowContaining(t *testing.T, h *harness, want string) string {
	t.Helper()
	for _, line := range h.screenLines() {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no screen line contains %q:\n%s", want, h.screenText())
	return ""
}

// errBdFailed is the canned failure for tests that exercise an error path.
var errBdFailed = errors.New("bd update failed: database is locked")

// waitFor polls cond on the event loop until it holds, failing the test with
// desc if it does not settle in time.
func (h *harness) waitFor(desc string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(harnessTimeout)
	for {
		var ok bool
		h.read(func() { ok = cond() })
		if ok {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for %s", desc)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// --- state accessors -------------------------------------------------------

// statusText returns the status bar contents with color tags stripped.
//
// Use screenText (or rowContaining) for anything in square brackets: this is
// the text the app handed to SetText, and a "[Word]" that tview swallows as a
// style tag reads identically here whether or not it is escaped (tui-42ws).
func (h *harness) statusText() string {
	var text string
	h.read(func() { text = h.tui.statusBar.GetText(true) })
	return text
}

// detailText returns the detail panel contents with color tags stripped.
func (h *harness) detailText() string {
	var text string
	h.read(func() { text = h.tui.detailPanel.GetText(true) })
	return text
}

// currentIndex returns the selected row index (rows include group headers).
func (h *harness) currentIndex() int {
	var idx int
	h.read(func() { idx = h.tui.issueList.GetCurrentItem() })
	return idx
}

// currentIssueID returns the id of the selected issue, or "" on a header row.
func (h *harness) currentIssueID() string {
	var id string
	h.read(func() {
		if issue, ok := h.tui.indexToIssue[h.tui.issueList.GetCurrentItem()]; ok {
			id = issue.ID
		}
	})
	return id
}

// issueIDs returns the ids of every issue row, in display order.
func (h *harness) issueIDs() []string {
	var ids []string
	h.read(func() {
		for i := 0; i < h.tui.issueList.GetItemCount(); i++ {
			if issue, ok := h.tui.indexToIssue[i]; ok {
				ids = append(ids, issue.ID)
			}
		}
	})
	return ids
}

// rowCount returns the number of rows in the list, headers included.
func (h *harness) rowCount() int {
	var n int
	h.read(func() { n = h.tui.issueList.GetItemCount() })
	return n
}

// frontPage returns the name of the topmost page ("main" when no dialog is up).
func (h *harness) frontPage() string {
	var name string
	h.read(func() { name, _ = h.tui.pages.GetFrontPage() })
	return name
}

// screenLines renders the simulation screen as text, one string per row with
// trailing blanks trimmed.
//
// The cell after a double-width glyph is skipped: tcell's simulation screen
// never overwrites it, so it still holds whatever was drawn there in an earlier
// frame and would otherwise show up as garbage inside the row.
func (h *harness) screenLines() []string {
	var lines []string
	h.read(func() {
		cells, width, height := h.screen.GetContents()
		lines = make([]string, 0, height)
		for y := 0; y < height; y++ {
			var row strings.Builder
			for x := 0; x < width; x++ {
				runes := cells[y*width+x].Runes
				if len(runes) == 0 {
					row.WriteRune(' ')
					continue
				}
				text := string(runes)
				row.WriteString(text)
				if w := tview.TaggedStringWidth(text); w > 1 {
					x += w - 1
				}
			}
			lines = append(lines, strings.TrimRight(row.String(), " "))
		}
	})
	return lines
}

// screenText is screenLines joined by newlines, for substring assertions.
func (h *harness) screenText() string {
	return strings.Join(h.screenLines(), "\n")
}

// --- fake bd ---------------------------------------------------------------

// fakeBd stands in for the bd CLI. By default it applies `update` commands to
// the fixture, rewrites issues.jsonl, and echoes the updated issue back, which
// is enough for the TUI's success path (status message plus scheduled reload).
func (h *harness) fakeBd(args ...string) (*BdCommandResult, error) {
	h.bdMu.Lock()
	h.bdCalls = append(h.bdCalls, bdCall{Args: append([]string(nil), args...)})
	respond := h.bdRespond
	h.bdMu.Unlock()

	if respond != nil {
		return respond(args)
	}

	if len(args) < 2 || args[0] != "update" {
		return nil, fmt.Errorf("fake bd: unsupported command %q", strings.Join(args, " "))
	}

	issue := h.findIssue(args[1])
	if issue == nil {
		return nil, fmt.Errorf("issue not found: %s", args[1])
	}

	for i := 2; i+1 < len(args); i += 2 {
		switch args[i] {
		case "--status":
			issue.Status = parser.Status(args[i+1])
		case "--priority":
			var p int
			if _, err := fmt.Sscanf(args[i+1], "%d", &p); err != nil {
				return nil, fmt.Errorf("fake bd: bad priority %q", args[i+1])
			}
			issue.Priority = p
		case "--title":
			issue.Title = args[i+1]
		case "--labels":
			issue.Labels = strings.Split(args[i+1], ",")
		case "--assignee":
			// bd drops the key entirely when handed an empty value.
			issue.Assignee = args[i+1]
		}
	}
	h.writeJSONL()

	return &BdCommandResult{Issues: []parser.Issue{*issue}}, nil
}

// calls returns the recorded bd invocations.
func (h *harness) calls() []bdCall {
	h.bdMu.Lock()
	defer h.bdMu.Unlock()
	return append([]bdCall(nil), h.bdCalls...)
}

// setBdResponse installs a custom fake bd, e.g. one that always fails.
func (h *harness) setBdResponse(fn func(args []string) (*BdCommandResult, error)) {
	h.bdMu.Lock()
	h.bdRespond = fn
	h.bdMu.Unlock()
}

func (h *harness) findIssue(id string) *parser.Issue {
	for _, issue := range h.issues {
		if issue.ID == id {
			return issue
		}
	}
	return nil
}

// writeJSONL rewrites issues.jsonl from the fixture. Writing to a temp file and
// renaming mirrors how bd replaces the file, and keeps readers from seeing a
// half-written line.
func (h *harness) writeJSONL() {
	h.t.Helper()

	var buf strings.Builder
	for _, issue := range h.issues {
		line, err := json.Marshal(issue)
		if err != nil {
			h.t.Fatalf("marshaling %s: %v", issue.ID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	tmp := h.jsonlPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(buf.String()), 0o644); err != nil {
		h.t.Fatalf("writing issues.jsonl: %v", err)
	}
	if err := os.Rename(tmp, h.jsonlPath); err != nil {
		h.t.Fatalf("renaming issues.jsonl: %v", err)
	}
}

// --- fixtures --------------------------------------------------------------

// fixtureTime is a fixed timestamp so fixtures serialize deterministically.
var fixtureTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// newIssue builds a fixture issue. Defaults are open/P2/task; apply the
// with* helpers to change them.
func newIssue(id, title string, mods ...func(*parser.Issue)) *parser.Issue {
	issue := &parser.Issue{
		ID:        id,
		Title:     title,
		Status:    parser.StatusOpen,
		Priority:  2,
		IssueType: parser.TypeTask,
		CreatedAt: fixtureTime,
		UpdatedAt: fixtureTime,
	}
	for _, mod := range mods {
		mod(issue)
	}
	return issue
}

func withStatus(s parser.Status) func(*parser.Issue) {
	return func(i *parser.Issue) { i.Status = s }
}

func withPriority(p int) func(*parser.Issue) {
	return func(i *parser.Issue) { i.Priority = p }
}

func withType(t parser.IssueType) func(*parser.Issue) {
	return func(i *parser.Issue) { i.IssueType = t }
}

func withLabels(labels ...string) func(*parser.Issue) {
	return func(i *parser.Issue) { i.Labels = labels }
}

func withAssignee(assignee string) func(*parser.Issue) {
	return func(i *parser.Issue) { i.Assignee = assignee }
}

// withBlockedBy makes the issue depend on blockerID, so it lands in the blocked
// group while the blocker is open.
func withBlockedBy(blockerID string) func(*parser.Issue) {
	return func(i *parser.Issue) {
		i.Dependencies = append(i.Dependencies, &parser.Dependency{
			IssueID:     i.ID,
			DependsOnID: blockerID,
			Type:        parser.DepBlocks,
			CreatedAt:   fixtureTime,
		})
	}
}

// withParent makes the issue a child of parentID in the dependency tree.
func withParent(parentID string) func(*parser.Issue) {
	return func(i *parser.Issue) {
		i.Dependencies = append(i.Dependencies, &parser.Dependency{
			IssueID:     i.ID,
			DependsOnID: parentID,
			Type:        parser.DepParentChild,
			CreatedAt:   fixtureTime,
		})
	}
}

// treeIssues is a fixture with a real parent-child edge, so tree view has
// something to collapse.
func treeIssues() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-top", "Top level epic", withType(parser.TypeEpic)),
		newIssue("tui-kid", "Child of the epic", withParent("tui-top")),
		newIssue("tui-solo", "Unrelated issue"),
	}
}

// basicIssues is the default fixture: three ready issues and one closed one.
// There is deliberately no in-progress issue, so row 0 is the READY header and
// row 1 is the first issue.
func basicIssues() []*parser.Issue {
	return []*parser.Issue{
		newIssue("tui-aaa", "Alpha issue", withPriority(1), withType(parser.TypeBug)),
		newIssue("tui-bbb", "Bravo issue", withLabels("ui")),
		newIssue("tui-ccc", "Charlie issue", withPriority(3)),
		newIssue("tui-zzz", "Zulu closed issue", withStatus(parser.StatusClosed)),
	}
}
