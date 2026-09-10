package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andynu/bd-lite-tui/internal/clipboard"
	"github.com/andynu/bd-lite-tui/internal/config"
	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/andynu/bd-lite-tui/internal/identity"
	"github.com/andynu/bd-lite-tui/internal/layout"
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/andynu/bd-lite-tui/internal/storage"
	"github.com/andynu/bd-lite-tui/internal/theme"
	"github.com/andynu/bd-lite-tui/internal/ui"
	"github.com/andynu/bd-lite-tui/internal/watcher"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	// statusMessageDuration is how long temporary status bar messages are shown.
	statusMessageDuration = 2 * time.Second

	// refreshDelay is the delay before auto-refreshing after an update command.
	refreshDelay = 500 * time.Millisecond

	// queueUpdateTimeout is the max wait for tview QueueUpdateDraw calls.
	queueUpdateTimeout = 10 * time.Second

	// loadTimeout is the max wait for issue load operations.
	loadTimeout = 5 * time.Second

	// watcherDebounce is the file watcher debounce interval.
	watcherDebounce = 200 * time.Millisecond
)

// resolveIdentity returns the human the TUI acts as. It is a variable rather
// than a direct call so tests can pin an identity; otherwise every assertion
// about assignees would depend on whatever git user.name the developer happens
// to have configured.
var resolveIdentity = identity.Resolve

// clipboardWriter is what the TUI needs from internal/clipboard.
type clipboardWriter interface {
	Copy(text string) (clipboard.Result, error)
}

// newClipboard builds the clipboard the TUI copies to. It is a variable so
// tests can record copies instead of writing to the developer's clipboard.
var newClipboard = func() clipboardWriter { return clipboard.New() }

// tuiOptions is everything newTUI needs that main() resolves from flags, the
// environment, and the user's config. Keeping it a plain struct is what makes
// the whole event layer reachable from a test: a test fills it in with a temp
// .beads directory and a tcell SimulationScreen instead of a real terminal.
type tuiOptions struct {
	// BeadsDir is the .beads directory; collapse state is persisted there.
	BeadsDir string

	// JSONLPath is the issues.jsonl file to read and watch.
	JSONLPath string

	// IssueID, when non-empty, restricts the view to that single issue
	// (the --issue flag). The restriction survives refreshes.
	IssueID string

	// View is the view mode to start in (the --view flag). The zero value is
	// state.ViewList.
	View state.ViewMode

	// Screen, when non-nil, is used instead of a real terminal screen.
	// Tests pass a tcell SimulationScreen.
	Screen tcell.Screen

	// NoWatcher skips the filesystem watcher. Tests that drive refreshes
	// themselves use this to keep timing deterministic.
	NoWatcher bool

	// Now supplies the moment the time-keyed views bucket against. It is
	// consulted on every repopulate, so an issue crosses from "Today" into
	// "Yesterday" without a restart. nil means time.Now.
	Now func() time.Time
}

// tui is a fully wired application: widgets, state, key bindings, and the
// filesystem watcher. Run blocks on the tview event loop; Close releases the
// watcher and the JSONL reader.
type tui struct {
	opts tuiOptions

	app         *tview.Application
	pages       *tview.Pages
	statusBar   *tview.TextView
	issueList   *tview.List
	detailPanel *tview.TextView

	state        *state.State
	indexToIssue map[int]*parser.Issue

	reader      *storage.JSONLReader
	fileWatcher *watcher.Watcher

	refreshIssues     func(...string)
	saveCollapseState func()

	closeOnce sync.Once
}

// newTUI builds the application without starting its event loop.
func newTUI(opts tuiOptions) (*tui, error) {
	// Open JSONL reader
	jsonlReader, err := storage.NewJSONLReader(opts.JSONLPath)
	if err != nil {
		return nil, fmt.Errorf("opening JSONL file: %w", err)
	}

	// Initialize state
	appState := state.New()

	// Resolve who we are once, from git. An unset user.name leaves this empty,
	// which disables auto-claim and the "mine" filter rather than guessing at an
	// identity and writing a wrong name into someone's issue.
	me := resolveIdentity(opts.BeadsDir)
	appState.SetIdentity(me)
	if me == "" {
		log.Printf("IDENTITY: git user.name is unset; auto-claim and 'mine' filtering are disabled")
	} else {
		log.Printf("IDENTITY: acting as %q", me)
	}

	// Set initial view mode from command line
	appState.SetViewMode(opts.View)

	// Create TUI application
	app := tview.NewApplication()
	if opts.Screen != nil {
		app.SetScreen(opts.Screen)
	}

	// Apply theme background and foreground colors
	currentTheme := theme.Current()
	tview.Styles.PrimitiveBackgroundColor = currentTheme.AppBackground()
	tview.Styles.PrimaryTextColor = currentTheme.AppForeground()
	tview.Styles.ContrastBackgroundColor = currentTheme.InputFieldBackground()
	tview.Styles.MoreContrastBackgroundColor = currentTheme.InputFieldBackground()

	// Status bar
	statusBar := tview.NewTextView().
		SetDynamicColors(true)

	// Issue list
	issueList := tview.NewList().
		ShowSecondaryText(false).
		SetSelectedBackgroundColor(currentTheme.SelectionBg()).
		SetSelectedTextColor(currentTheme.SelectionFg())
	issueList.SetBorder(true).SetTitle("Issues")

	// Track mapping from list index to issue
	indexToIssue := make(map[int]*parser.Issue)

	t := &tui{
		opts:         opts,
		app:          app,
		statusBar:    statusBar,
		issueList:    issueList,
		state:        appState,
		indexToIssue: indexToIssue,
		reader:       jsonlReader,
	}

	// Vim navigation state
	var lastKeyWasG bool
	var searchMode bool
	var searchQuery string
	var searchMatches []int
	var currentSearchIndex int

	// Two-character shortcut state
	var lastKeyWasS bool // For status shortcuts (So, Si, Sb, Sc)

	// ESC to quit state (double-press within 1 second)
	var lastEscapeTime time.Time

	// Mouse mode state (default: enabled)
	var mouseEnabled = true

	// Panel focus state (true = detail panel, false = issue list)
	var detailPanelFocused bool

	// Show closed issues in list view (default: false)
	var showClosedIssues bool

	// Layout state. currentLayout is what is actually mounted; layout.Resolve
	// derives it from the terminal width, so a narrow pane pages between the
	// list and the detail instead of splitting. layoutPin is the user's
	// override from `v`, honored only at widths wide enough for a split.
	var currentLayout = layout.Columns
	var layoutPin = layout.Columns
	var layoutPinned bool

	// Last terminal width seen by the draw hook, so key handlers can re-resolve
	// the layout without waiting for the next frame.
	var lastScreenWidth int

	// Show issue ID prefix (default: true)
	var showPrefix = true

	// Track currently displayed issue in detail panel (for clipboard copy)
	var currentDetailIssue *parser.Issue

	// currentDetailLinks says which lines of the detail text a click acts on.
	var currentDetailLinks formatting.DetailLinks

	// Helper functions for themed messages
	successMsg := func(msg string) string {
		return fmt.Sprintf("[%s]%s[-]", formatting.GetSuccessColor(), msg)
	}
	errorMsg := func(msg string) string {
		return fmt.Sprintf("[%s]%s[-]", formatting.GetErrorColor(), msg)
	}
	warningMsg := func(msg string) string {
		return fmt.Sprintf("[%s]%s[-]", formatting.GetWarningColor(), msg)
	}
	_ = func(msg string) string { // emphasisMsg - reserved for future use
		return fmt.Sprintf("[%s]%s[-]", formatting.GetEmphasisColor(), msg)
	}

	// Helper function to generate issue list title with view mode indicator
	getIssueListTitle := func() string {
		mode := appState.GetViewMode()
		next := state.NextViewMode(mode)
		// Show position indicator if on an issue
		posStr := ""
		if issueList.GetItemCount() > 0 {
			currentIdx := issueList.GetCurrentItem()
			if _, ok := indexToIssue[currentIdx]; ok {
				// Count which issue number this is (1-based)
				issueNum := 0
				for i := 0; i <= currentIdx; i++ {
					if _, ok := indexToIssue[i]; ok {
						issueNum++
					}
				}
				posStr = fmt.Sprintf(" %d/%d", issueNum, len(indexToIssue))
			}
		}
		// "[%s[]" renders a literal "[List]" / "[Tree]"; bare, tview reads the
		// word as a color name and eats the whole tag (tui-42ws).
		return fmt.Sprintf("Issues [%s[]%s (t:%s)", mode, posStr, next)
	}

	// The .beads path is shortened once: it never changes, and at full length it
	// filled the status line on its own and pushed the layout and focus
	// indicators off the right edge (tui-7j4u).
	homeDir, _ := os.UserHomeDir()
	beadsLabel := statusLocation(opts.BeadsDir, homeDir)

	// Helper function to generate status bar text
	getStatusBarText := func() string {
		focusStr := "List"
		if detailPanelFocused {
			focusStr = "Details"
		}

		// Count visible issues after filtering
		visibleCount := len(appState.GetReadyIssues()) + len(appState.GetBlockedIssues()) + len(appState.GetInProgressIssues())
		if showClosedIssues {
			visibleCount += len(appState.GetClosedIssues())
		}

		filters := ""
		if appState.HasActiveFilters() {
			filters = appState.GetActiveFilters()
		}

		// A trailing * means the layout is pinned rather than resolved from width.
		layoutStr := currentLayout.String()
		if layoutPinned {
			layoutStr += "*"
		}

		return formatStatusBar(statusBarFields{
			EmphasisColor: formatting.GetEmphasisColor(),
			Location:      beadsLabel,
			IssueCount:    visibleCount,
			Focus:         focusStr,
			View:          appState.GetViewMode().String(),
			Layout:        layoutStr,
			Filters:       filters,
			ShowingClosed: showClosedIssues,
			MouseEnabled:  mouseEnabled,
		})
	}

	// Helper function to populate issue list from state
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	populateIssueList := func() {
		ui.PopulateIssueList(issueList, appState, showClosedIssues, showPrefix, now(), indexToIssue)
	}

	// safeQueueUpdateDraw wraps app.QueueUpdateDraw with timeout protection
	// to prevent hanging if the tview event loop becomes unresponsive
	safeQueueUpdateDraw := func(f func()) {
		done := make(chan struct{})
		go func() {
			app.QueueUpdateDraw(f)
			close(done)
		}()

		select {
		case <-done:
			// Success - update queued normally
		case <-time.After(queueUpdateTimeout):
			log.Printf("WARNING: QueueUpdateDraw timed out after 10s")
		}
	}

	// showTemporaryStatus displays a message in the status bar that auto-clears
	// after the given duration, reverting to the default status bar text.
	showTemporaryStatus := func(msg string, duration time.Duration) {
		statusBar.SetText(msg)
		time.AfterFunc(duration, func() {
			safeQueueUpdateDraw(func() {
				statusBar.SetText(getStatusBarText())
			})
		})
	}

	// clip copies to the system clipboard. It resolves the display itself, so it
	// still works in a tmux pane that was started without one.
	clip := newClipboard()

	// copyToClipboard copies text and reports the outcome in the status bar.
	// description is the outcome phrase, e.g. "tui-abc to clipboard".
	//
	// There are three outcomes, not two. A display helper acknowledges the copy,
	// so a checkmark is earned. The OSC 52 route hands the sequence to the
	// terminal and hears nothing back, so it gets an arrow and the word
	// unverified: the text has probably arrived, but nothing here saw it.
	copyToClipboard := func(text, description string) {
		res, err := clip.Copy(text)
		if err != nil {
			// The message carries the helper's own diagnostic (and, in tmux, how
			// to recover a missing display), so show it rather than an exit code.
			log.Printf("CLIPBOARD ERROR: copying %q: %v", text, err)
			showTemporaryStatus(errorMsg(fmt.Sprintf("Copy failed: %v", err)), statusMessageDuration)
			return
		}
		log.Printf("CLIPBOARD: sent %q via %s (confirmed: %t, display recovered from tmux: %t)",
			text, res.Helper, res.Confirmed, res.RecoveredFromTmux)
		if !res.Confirmed {
			showTemporaryStatus(
				warningMsg(fmt.Sprintf("→ Sent %s via %s (unverified)", description, res.Helper)),
				statusMessageDuration)
			return
		}
		showTemporaryStatus(successMsg(fmt.Sprintf("✓ Copied %s", description)), statusMessageDuration)
	}

	// Single-flight guards for refreshes. Rather than serialize refreshes behind
	// a mutex held across UI queueing (which could stall every refresh for up to
	// queueUpdateTimeout and leak goroutines), overlapping requests coalesce:
	// refreshRunning marks an in-flight refresh and refreshAgain records that
	// another was requested while one was running.
	var refreshRunning atomic.Bool
	var refreshAgain atomic.Bool

	// Refresh timer for single-flight pattern (prevent timer pile-up)
	var refreshTimer *time.Timer
	var refreshTimerMutex sync.Mutex

	// Forward declare refreshIssues for use in scheduleRefresh
	var refreshIssues func(...string)

	// scheduleRefresh schedules a delayed refresh, cancelling any pending refresh
	// This prevents timer pile-up when user performs rapid actions
	scheduleRefresh := func(issueID string) {
		refreshTimerMutex.Lock()
		defer refreshTimerMutex.Unlock()

		// Cancel existing timer if present
		if refreshTimer != nil {
			refreshTimer.Stop()
		}

		// Schedule new refresh
		refreshTimer = time.AfterFunc(refreshDelay, func() {
			log.Printf("SCHEDULE: Delayed refresh starting for issue: %s", issueID)
			refreshIssues(issueID)
		})
		log.Printf("SCHEDULE: Refresh scheduled in 500ms for issue: %s", issueID)
	}

	// doRefresh performs one load+apply cycle. The JSONL load runs on the calling
	// (background) goroutine; state mutation and the UI rebuild happen together
	// inside a single QueueUpdateDraw on the main thread. Doing both on the main
	// thread means background loads never race with main-thread reads, and no
	// lock is held across UI queueing. targetIssueID selects which issue to keep
	// selected; empty means auto-preserve the current selection.
	doRefresh := func(targetIssueID string) {
		log.Printf("REFRESH: Starting issue load")

		// Load issues from JSONL with timeout (off the main thread).
		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		issues, err := jsonlReader.LoadIssues(ctx)
		cancel()
		if err != nil {
			log.Printf("REFRESH ERROR: Failed to load issues: %v", err)
			errText := fmt.Sprintf("Error loading issues: %v", err)
			app.QueueUpdateDraw(func() {
				statusBar.SetText(errorMsg(errText))
			})
			return
		}
		log.Printf("REFRESH: Loaded %d issues from JSONL", len(issues))

		// Honor the --issue single-issue filter across refreshes. Without this,
		// any watcher event or manual refresh would reload the full issue set and
		// silently discard the filter the user launched with.
		if opts.IssueID != "" {
			filtered := make([]*parser.Issue, 0, 1)
			for _, issue := range issues {
				if issue.ID == opts.IssueID {
					filtered = append(filtered, issue)
					break
				}
			}
			issues = filtered
		}

		// Apply state + UI together on the main thread.
		app.QueueUpdateDraw(func() {
			log.Printf("REFRESH: UI update executing")

			// Resolve the selection to preserve, reading UI state on the main
			// thread (before the list is rebuilt).
			previousIndex := issueList.GetCurrentItem()
			target := targetIssueID
			if target == "" {
				if currentIssue, ok := indexToIssue[previousIndex]; ok {
					target = currentIssue.ID
				}
			}

			appState.LoadIssues(issues)
			statusBar.SetText(getStatusBarText())
			populateIssueList()

			// Restore selection.
			if target != "" {
				found := false
				for idx, issue := range indexToIssue {
					if issue.ID == target {
						issueList.SetCurrentItem(idx)
						found = true
						break
					}
				}
				// If the issue is no longer in the list (e.g., it was closed),
				// keep the cursor at the same position or the last item.
				if !found {
					if itemCount := issueList.GetItemCount(); itemCount > 0 {
						if previousIndex >= itemCount {
							previousIndex = itemCount - 1
						}
						issueList.SetCurrentItem(previousIndex)
					}
				}
			}

			log.Printf("REFRESH: UI update complete")
		})
		log.Printf("REFRESH: Issue refresh complete")
	}

	// refreshIssues triggers a refresh. It is non-blocking and single-flight:
	// concurrent callers (file watcher, manual 'r', post-update scheduleRefresh)
	// coalesce into at most one in-flight refresh plus one queued re-run, so
	// bursts of changes collapse instead of piling up goroutines.
	//
	// preserveIssueID: if provided, the next refresh restores selection to it;
	// coalesced re-runs fall back to auto-preserving the current selection
	// (which, after an in-place update, is the same issue).
	refreshIssues = func(preserveIssueID ...string) {
		targetID := ""
		if len(preserveIssueID) > 0 {
			targetID = preserveIssueID[0]
		}

		refreshAgain.Store(true)
		if !refreshRunning.CompareAndSwap(false, true) {
			// A refresh is already running; it will pick up the request above.
			return
		}

		go func(firstTarget string) {
			for {
				for refreshAgain.CompareAndSwap(true, false) {
					doRefresh(firstTarget)
					firstTarget = "" // coalesced re-runs auto-preserve selection
				}
				refreshRunning.Store(false)
				// A request may have arrived between the failed CAS above and the
				// Store; re-acquire the slot if so, otherwise we're done.
				if !(refreshAgain.Load() && refreshRunning.CompareAndSwap(false, true)) {
					return
				}
			}
		}(targetID)
	}
	t.refreshIssues = refreshIssues

	// Initial load (before app starts, no QueueUpdateDraw)
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	issues, err := jsonlReader.LoadIssues(ctx)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("loading issues: %w", err)
	}
	appState.LoadIssues(issues)

	// Load collapse state from disk (persisted between sessions)
	collapseState, err := config.LoadCollapseState(opts.BeadsDir)
	if err != nil {
		log.Printf("Warning: failed to load collapse state: %v", err)
	} else {
		appState.SetCollapsedNodes(collapseState.CollapsedNodes)
		log.Printf("Loaded collapse state: %d nodes", len(collapseState.CollapsedNodes))
	}

	// Helper function to save collapse state (called on toggle and exit)
	saveCollapseState := func() {
		st := &config.CollapseState{
			CollapsedNodes: appState.GetCollapsedNodes(),
		}
		if err := config.SaveCollapseState(opts.BeadsDir, st); err != nil {
			log.Printf("Warning: failed to save collapse state: %v", err)
		} else {
			log.Printf("Saved collapse state: %d nodes", len(st.CollapsedNodes))
		}
	}
	t.saveCollapseState = saveCollapseState

	// Filter by issue ID if specified
	if opts.IssueID != "" {
		filtered := make([]*parser.Issue, 0)
		for _, issue := range issues {
			if issue.ID == opts.IssueID {
				filtered = append(filtered, issue)
				break
			}
		}
		if len(filtered) == 0 {
			return nil, fmt.Errorf("issue %s not found", opts.IssueID)
		}
		appState.LoadIssues(filtered)
	}

	statusBar.SetText(getStatusBarText())
	populateIssueList()

	// Set up filesystem watcher on the JSONL file
	if !opts.NoWatcher {
		log.Printf("Setting up file watcher on: %s", opts.JSONLPath)
		fileWatcher, err := watcher.New(opts.JSONLPath, watcherDebounce, func() {
			log.Printf("WATCHER: File change detected, triggering refresh")
			refreshIssues()
		})
		if err != nil {
			log.Printf("WATCHER ERROR: Failed to create watcher: %v", err)
			fmt.Fprintf(os.Stderr, "Warning: failed to set up file watcher: %v\n", err)
			fmt.Fprintf(os.Stderr, "Live updates will not work. Press 'r' to manually refresh.\n")
		} else {
			if err := fileWatcher.Start(); err != nil {
				log.Printf("WATCHER ERROR: Failed to start watcher: %v", err)
				fmt.Fprintf(os.Stderr, "Warning: failed to start file watcher: %v\n", err)
			} else {
				log.Printf("WATCHER: File watcher started successfully")
			}
			t.fileWatcher = fileWatcher
		}
	}

	// Detail panel
	detailPanel := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	detailPanel.SetBorder(true).SetTitle("Details")
	detailPanel.SetText(fmt.Sprintf("[%s]Navigate to an issue to view details[-]", formatting.GetEmphasisColor()))
	t.detailPanel = detailPanel

	// Rebuilds the main page from the current layout. Assigned once `pages`
	// exists; declared here because updatePanelFocus below has to call it.
	var remountLayout func()

	// Reports whether a modal dialog is in front of the main page. Assigned
	// alongside remountLayout, for the same reason, and nil until then — the
	// first updatePanelFocus call runs before there are any pages at all.
	var dialogOpen func() bool

	// Helper function to update panel focus indicators
	updatePanelFocus := func() {
		if detailPanelFocused {
			issueList.SetBorderColor(tcell.ColorGray)
			issueList.SetTitle(getIssueListTitle())
			detailPanel.SetBorderColor(tcell.ColorYellow)
			detailPanel.SetTitle("Details [FOCUSED - Use Ctrl-d/u to scroll, ESC to return]")
		} else {
			issueList.SetBorderColor(tcell.ColorDefault)
			issueList.SetTitle(getIssueListTitle())
			detailPanel.SetBorderColor(tcell.ColorGray)
			detailPanel.SetTitle("Details [Press Tab or Enter to focus]")
		}
		// In Paged mode focus decides which panel is mounted at all, not just
		// which border is highlighted, so the page is rebuilt before focus
		// lands. Focusing an unmounted primitive is a no-op.
		if currentLayout == layout.Paged && remountLayout != nil {
			remountLayout()
		}
		// A modal owns the keyboard for as long as it is up. Pulling focus down
		// to a panel behind it would leave the dialog on screen with its keys
		// going elsewhere, so the panel focus is only asserted when the main
		// page is the front one (tui-bl1e). The dialog gets focus back from
		// tview instead: Pages.Focus delegates to its topmost visible page.
		if dialogOpen == nil || !dialogOpen() {
			if detailPanelFocused {
				app.SetFocus(detailPanel)
			} else {
				app.SetFocus(issueList)
			}
		}
		statusBar.SetText(getStatusBarText())
	}
	// Set initial focus state
	updatePanelFocus()

	// Function to show issue details
	showIssueDetails := func(issue *parser.Issue) {
		currentDetailIssue = issue
		details, links := formatting.FormatIssueDetailsWithLinks(issue, appState.GetIssueByID)
		currentDetailLinks = links
		detailPanel.SetText(details)
		detailPanel.ScrollToBeginning()
	}

	// Set up change handler to auto-show details on selection change
	issueList.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		// Check if the selected item is an issue (not a header)
		if issue, ok := indexToIssue[index]; ok {
			showIssueDetails(issue)
		}
		// Update title to reflect current position
		issueList.SetTitle(getIssueListTitle())
	})

	// A left click on a row is the mouse equivalent of Enter. In Paged mode only
	// one panel is mounted, so a click that merely moved the selection changed
	// nothing the user could see; it now turns the page to the detail as well
	// (tui-n0hc). In Columns/Rows both panels are already on screen, so the click
	// keeps its old meaning and focus stays on the list.
	//
	// tview's List runs this before assigning its own currentItem, so the
	// selection is moved here explicitly: SetCurrentItem fires the changed
	// handler that repaints the detail, and leaves the List's later assignment a
	// no-op. Enter reaches this handler only on a header row (the app's input
	// capture swallows it on an issue), which the indexToIssue guard drops.
	issueList.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if _, ok := indexToIssue[index]; !ok {
			return
		}
		issueList.SetCurrentItem(index)
		if currentLayout != layout.Paged || detailPanelFocused {
			return
		}
		detailPanelFocused = true
		updatePanelFocus()
	})

	// selectAdjacentIssue moves the selection to the next issue row in the
	// direction of delta (+1 down, -1 up), stepping over the group headers that
	// would otherwise leave the detail panel showing the previous issue. The
	// SetCurrentItem call fires the changed handler above, which repaints the
	// detail, so this works whether or not the list is the mounted panel — which
	// is what makes it usable from the detail panel in Paged mode.
	//
	// It stops at the ends rather than wrapping: with the list off screen there
	// is nothing to show that the cursor jumped back to the other end.
	selectAdjacentIssue := func(delta int) bool {
		if delta == 0 {
			return false
		}
		count := issueList.GetItemCount()
		for idx := issueList.GetCurrentItem() + delta; idx >= 0 && idx < count; idx += delta {
			if _, ok := indexToIssue[idx]; ok {
				issueList.SetCurrentItem(idx)
				return true
			}
		}
		return false
	}

	// Layout builder function
	buildLayout := func() *tview.Flex {
		var contentFlex *tview.Flex

		switch currentLayout {
		case layout.Paged:
			// One panel at a time, full screen, chosen by focus. Tab/Enter page
			// into the detail, Tab/ESC page back to the list.
			if detailPanelFocused {
				contentFlex = tview.NewFlex().
					AddItem(detailPanel, 0, 1, true)
			} else {
				contentFlex = tview.NewFlex().
					AddItem(issueList, 0, 1, true)
			}
		case layout.Rows:
			// List on top (40%), details on bottom (60%)
			contentFlex = tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(issueList, 0, 40, !detailPanelFocused).
				AddItem(detailPanel, 0, 60, detailPanelFocused)
		default:
			// List on left (1 part), details on right (2 parts)
			contentFlex = tview.NewFlex().
				AddItem(issueList, 0, 1, !detailPanelFocused).
				AddItem(detailPanel, 0, 2, detailPanelFocused)
		}

		return tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(statusBar, 1, 0, false).
			AddItem(contentFlex, 0, 1, true)
	}

	flex := buildLayout()

	// Pages for modal dialogs
	pages := tview.NewPages().
		AddPage("main", flex, true, true)
	t.pages = pages

	dialogOpen = func() bool {
		name, _ := pages.GetFrontPage()
		return name != "main"
	}

	// tview cannot swap a Flex's children in place, so a layout change rebuilds
	// the main page. Modal pages sit above "main" and are left untouched — but
	// AddPage appends, so a re-added "main" would land on top of them, draw over
	// the dialog and become GetFrontPage() while the dialog still held the keys
	// (tui-bl1e). SendToBack puts it back underneath, and takes focus with it:
	// it ends by re-delegating focus to the topmost visible page.
	remountLayout = func() {
		pages.RemovePage("main")
		pages.AddPage("main", buildLayout(), true, true)
		pages.SendToBack("main")
	}

	// Re-resolve the layout whenever the terminal width crosses the threshold.
	// The rebuild is queued rather than run here: calling QueueUpdateDraw
	// synchronously from inside a draw callback deadlocks, because the event
	// loop that would service the queue is the one blocked on this call.
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, _ := screen.Size()
		lastScreenWidth = width
		want := layout.Resolve(width, layoutPin, layoutPinned)
		if want == currentLayout {
			return false
		}
		currentLayout = want
		go app.QueueUpdateDraw(func() {
			remountLayout()
			updatePanelFocus()
		})
		return false
	})

	// revealIssue rebuilds the list so target has a row and returns that row, or
	// -1 if it still has none. An issue can be hidden by being closed (only the
	// flat views show closed issues, and only with C on), by a filter, or by a
	// folded ancestor in tree view; revealed reports whether any of that changed.
	revealIssue := func(target *parser.Issue) (row int, revealed bool) {
		if target.Status == parser.StatusClosed {
			if appState.GetViewMode() == state.ViewTree {
				appState.SetViewMode(state.ViewList)
				issueList.SetTitle(getIssueListTitle())
				revealed = true
			}
			if !showClosedIssues {
				showClosedIssues = true
				revealed = true
			}
		}

		if appState.HasActiveFilters() && !appState.IssuePassesFilters(target) {
			appState.ClearAllFilters()
			revealed = true
		}

		if appState.GetViewMode() == state.ViewTree && appState.ExpandAncestors(target.ID) {
			revealed = true
		}

		populateIssueList()

		for idx, iss := range indexToIssue {
			if iss.ID == target.ID {
				return idx, revealed
			}
		}
		return -1, revealed
	}

	// Helper function to perform search
	performSearch := func(query string) {
		searchMatches = nil
		currentSearchIndex = -1

		if query == "" {
			return
		}

		emphasisColor := formatting.GetEmphasisColor()
		errorColor := formatting.GetErrorColor()

		// Search through all currently-rendered rows (matches ID, title, labels -
		// everything shown in the row text).
		for i := 0; i < issueList.GetItemCount(); i++ {
			mainText, _ := issueList.GetItemText(i)
			// Simple case-insensitive substring search
			if len(mainText) > 0 && formatting.ContainsCaseInsensitive(mainText, query) {
				searchMatches = append(searchMatches, i)
			}
		}

		// Jump to first match if any
		if len(searchMatches) > 0 {
			currentSearchIndex = 0
			issueList.SetCurrentItem(searchMatches[0])
			statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [%d/%d matches] [Press n/N for next/prev, ESC to exit search]",
				emphasisColor, query, 1, len(searchMatches)))
			return
		}

		// No visible match. The target may exist but be hidden - collapsed under a
		// parent in tree view, filtered out, or closed. Find it in the full issue
		// set (ID-first), then reveal and select it.
		target := appState.FindIssueByQuery(query)
		if target == nil {
			statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [No matches]", errorColor, query))
			return
		}

		matchIndex, revealed := revealIssue(target)
		if matchIndex >= 0 {
			searchMatches = []int{matchIndex}
			currentSearchIndex = 0
			issueList.SetCurrentItem(matchIndex)
			revealNote := ""
			if revealed {
				// "[revealed[]" renders a literal "[revealed]" (tui-42ws).
				revealNote = " [revealed[]"
			}
			statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [matched %s]%s [ESC to exit search]",
				emphasisColor, query, target.ID, revealNote))
		} else {
			statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [matched %s but it could not be displayed]",
				errorColor, query, target.ID))
		}
	}

	// Helper function for next search result
	nextSearchMatch := func() {
		if len(searchMatches) == 0 {
			return
		}
		currentSearchIndex = (currentSearchIndex + 1) % len(searchMatches)
		issueList.SetCurrentItem(searchMatches[currentSearchIndex])
		statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [%d/%d matches] [Press n/N for next/prev, ESC to exit search]",
			formatting.GetEmphasisColor(), searchQuery, currentSearchIndex+1, len(searchMatches)))
	}

	// Helper function for previous search result
	prevSearchMatch := func() {
		if len(searchMatches) == 0 {
			return
		}
		currentSearchIndex--
		if currentSearchIndex < 0 {
			currentSearchIndex = len(searchMatches) - 1
		}
		issueList.SetCurrentItem(searchMatches[currentSearchIndex])
		statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s [%d/%d matches] [Press n/N for next/prev, ESC to exit search]",
			formatting.GetEmphasisColor(), searchQuery, currentSearchIndex+1, len(searchMatches)))
	}

	// followDependency moves the list selection to issue id, revealing it first
	// when it is closed, filtered out or folded away. Focus and layout are left
	// alone, so in Paged mode the detail page stays up and repaints.
	followDependency := func(id string) {
		target := appState.GetIssueByID(id)
		if target == nil {
			showTemporaryStatus(errorMsg(fmt.Sprintf("%s not found", tview.Escape(id))), statusMessageDuration)
			return
		}
		row, revealed := revealIssue(target)
		if row < 0 {
			showTemporaryStatus(errorMsg(fmt.Sprintf("%s exists but could not be displayed", tview.Escape(id))), statusMessageDuration)
			return
		}
		issueList.SetCurrentItem(row)
		// SetCurrentItem repaints only when the row changes, and the rebuilt list
		// can put the target on the row the cursor already holds.
		if currentDetailIssue != target {
			showIssueDetails(target)
		}
		note := ""
		if revealed {
			// "[revealed[]" renders a literal "[revealed]".
			note = " [revealed[]"
		}
		showTemporaryStatus(successMsg(fmt.Sprintf("→ %s", tview.Escape(id)))+note, statusMessageDuration)
	}

	// A click on the ID line copies the ID; a click on a dependency line follows
	// it. The clicked row is mapped back to a line of the rendered text through
	// the panel's word wrap, because any line above it can span several rows.
	detailPanel.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action != tview.MouseLeftClick || currentDetailIssue == nil {
			return action, event
		}
		x, y := event.Position()
		panelX, panelY, width, height := detailPanel.GetInnerRect()
		if x < panelX || x >= panelX+width || y < panelY || y >= panelY+height {
			return action, event
		}
		scroll, _ := detailPanel.GetScrollOffset()
		line, ok := detailLineAt(detailPanel.GetText(false), width, y-panelY+scroll)
		if !ok {
			return action, event
		}
		if line == currentDetailLinks.IDLine {
			copyToClipboard(currentDetailIssue.ID, fmt.Sprintf("%s to clipboard", currentDetailIssue.ID))
		} else if id, ok := currentDetailLinks.Dependencies[line]; ok {
			followDependency(id)
		}
		return action, event
	})

	// Create dialog helpers for all dialog functions
	dialogHelpers := &DialogHelpers{
		App:             app,
		Pages:           pages,
		IssueList:       issueList,
		IndexToIssue:    &indexToIssue,
		StatusBar:       statusBar,
		AppState:        appState,
		RefreshIssues:   refreshIssues,
		ScheduleRefresh: scheduleRefresh,
		OnFiltersChanged: func() {
			statusBar.SetText(getStatusBarText())
			populateIssueList()
		},
	}

	// Helper function to show comment dialog
	showCommentDialog := func() {
		dialogHelpers.ShowCommentDialog()
	}

	// Helper function to show rename dialog
	showRenameDialog := func() {
		dialogHelpers.ShowRenameDialog()
	}

	// Helper function to show quick filter (keyboard-friendly).
	// The repaint happens via OnFiltersChanged once the dialog applies or
	// clears; doing it here would run before the user has typed anything.
	showQuickFilter := func() {
		dialogHelpers.ShowQuickFilter()
	}

	// Helper function to show stats dashboard
	showStatsOverlay := func() {
		dialogHelpers.ShowStatsOverlay()
	}

	// Helper function to show help screen
	showHelpScreen := func() {
		dialogHelpers.ShowHelpScreen()
	}

	// Helper function to manage dependencies
	showDependencyDialog := func() {
		dialogHelpers.ShowDependencyDialog()
	}

	// Helper function to manage labels
	showLabelDialog := func() {
		dialogHelpers.ShowLabelDialog()
	}

	// Helper function to set or clear the assignee
	showAssignDialog := func() {
		dialogHelpers.ShowAssignDialog()
	}

	// Helper function to close issue with optional reason
	showCloseIssueDialog := func() {
		dialogHelpers.ShowCloseIssueDialog()
	}

	// Helper function to reopen closed issue with optional reason
	showReopenIssueDialog := func() {
		dialogHelpers.ShowReopenIssueDialog()
	}

	// Helper function to show edit form (in-TUI editing, similar to create issue form)
	showEditForm := func() {
		dialogHelpers.ShowEditForm()
	}

	// Helper function to show issue creation dialog
	showCreateIssueDialog := func() {
		dialogHelpers.ShowCreateIssueDialog()
	}

	// Set up key bindings
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Log all keyboard events in debug mode
		log.Printf("KEY EVENT: key=%v rune=%q mod=%v searchMode=%v detailFocus=%v",
			event.Key(), event.Rune(), event.Modifiers(), searchMode, detailPanelFocused)

		// If a modal is showing (not on main page), let the modal handle all input
		currentPage, _ := pages.GetFrontPage()
		if currentPage != "main" {
			return event
		}

		// Handle search mode
		if searchMode {
			switch event.Key() {
			case tcell.KeyEscape:
				searchMode = false
				searchQuery = ""
				statusBar.SetText(getStatusBarText())
				return nil
			case tcell.KeyEnter:
				performSearch(searchQuery)
				searchMode = false
				return nil
			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(searchQuery) > 0 {
					// Trim a full rune, not a single byte, so multi-byte UTF-8
					// input isn't corrupted into invalid sequences.
					runes := []rune(searchQuery)
					searchQuery = string(runes[:len(runes)-1])
					statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s_", formatting.GetEmphasisColor(), searchQuery))
				}
				return nil
			case tcell.KeyRune:
				searchQuery += string(event.Rune())
				statusBar.SetText(fmt.Sprintf("[%s]Search:[-] %s_", formatting.GetEmphasisColor(), searchQuery))
				return nil
			}
			return nil
		}

		// Handle detail panel scrolling when focused
		if detailPanelFocused {
			switch event.Key() {
			case tcell.KeyTab:
				// Return focus to issue list (toggle behavior)
				detailPanelFocused = false
				updatePanelFocus()
				return nil
			case tcell.KeyEscape, tcell.KeyLeft:
				// Return focus to issue list (keep detail pane visible).
				// Left arrow is the "go back" gesture, which matters most in
				// Paged mode where updatePanelFocus remounts the list page.
				detailPanelFocused = false
				updatePanelFocus()
				return nil
			case tcell.KeyDown:
				// Browse to the next issue instead of scrolling the text down a
				// line, which makes the detail panel a pager over the issue set.
				// Ctrl-e/Ctrl-y still move a single line.
				selectAdjacentIssue(1)
				return nil
			case tcell.KeyUp:
				selectAdjacentIssue(-1)
				return nil
			case tcell.KeyRune:
				// j/k mirror the arrows here, the way they do in the list.
				switch event.Rune() {
				case 'j':
					selectAdjacentIssue(1)
					return nil
				case 'k':
					selectAdjacentIssue(-1)
					return nil
				}
			case tcell.KeyCtrlD:
				// Scroll down half page
				_, _, _, height := detailPanel.GetInnerRect()
				for i := 0; i < height/2; i++ {
					detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
				}
				return nil
			case tcell.KeyCtrlU:
				// Scroll up half page
				_, _, _, height := detailPanel.GetInnerRect()
				for i := 0; i < height/2; i++ {
					detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
				}
				return nil
			case tcell.KeyCtrlE:
				// Scroll down one line
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyCtrlY:
				// Scroll up one line
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyCtrlF:
				// Scroll down full page (vim style)
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyCtrlB:
				// Scroll up full page (vim style)
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyPgDn:
				// Page down
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyPgUp:
				// Page up
				detailPanel.InputHandler()(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), nil)
				return nil
			case tcell.KeyHome:
				// Jump to top
				detailPanel.ScrollToBeginning()
				return nil
			case tcell.KeyEnd:
				// Jump to end
				detailPanel.ScrollToEnd()
				return nil
			}
			// Allow other keys to pass through
			return event
		}

		// Normal mode key bindings (issue list focused)
		switch event.Key() {
		case tcell.KeyEscape:
			// Clear search matches on ESC if any exist
			if len(searchMatches) > 0 {
				searchMatches = nil
				currentSearchIndex = -1
				statusBar.SetText(getStatusBarText())
				return nil
			}

			// Double ESC to quit (vim-style)
			now := time.Now()
			if !lastEscapeTime.IsZero() && now.Sub(lastEscapeTime) < time.Second {
				// Second ESC within 1 second - quit
				saveCollapseState() // Persist before exit
				app.Stop()
				return nil
			}
			// First ESC - show hint
			lastEscapeTime = now
			statusBar.SetText(fmt.Sprintf("[%s]Press ESC again to quit (or 'q')[-]", formatting.GetEmphasisColor()))

			// Clear the hint after 1 second. Both the read and the write of
			// lastEscapeTime happen inside QueueUpdateDraw so all access to this
			// (and the other input-state vars) stays on the tview event loop
			// goroutine — avoiding a data race with the input handler.
			go func() {
				time.Sleep(time.Second)
				app.QueueUpdateDraw(func() {
					if time.Since(lastEscapeTime) >= time.Second {
						lastEscapeTime = time.Time{}
						statusBar.SetText(getStatusBarText())
					}
				})
			}()
			return nil
		case tcell.KeyTab:
			// Focus detail panel
			detailPanelFocused = true
			updatePanelFocus()
			return nil
		case tcell.KeyEnter:
			// If on an issue, show detail pane and focus it
			if _, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
				// In Paged mode updatePanelFocus remounts the page onto the
				// detail; in the split modes it just moves the highlight.
				detailPanelFocused = true
				updatePanelFocus()
				statusBar.SetText(getStatusBarText())
				return nil
			}
			return event
		case tcell.KeyCtrlB:
			// Scroll up full page (vim style)
			_, _, _, height := issueList.GetInnerRect()
			currentItem := issueList.GetCurrentItem()
			newItem := currentItem - height
			if newItem < 0 {
				newItem = 0
			}
			issueList.SetCurrentItem(newItem)
			return nil
		case tcell.KeyCtrlF:
			// Scroll down full page (vim style)
			_, _, _, height := issueList.GetInnerRect()
			currentItem := issueList.GetCurrentItem()
			maxItem := issueList.GetItemCount() - 1
			newItem := currentItem + height
			if newItem > maxItem {
				newItem = maxItem
			}
			issueList.SetCurrentItem(newItem)
			return nil
		case tcell.KeyRune:
			// Handle space bar for page down with wrapping
			if event.Rune() == ' ' {
				_, _, _, height := issueList.GetInnerRect()
				currentItem := issueList.GetCurrentItem()
				maxItem := issueList.GetItemCount() - 1
				newItem := currentItem + height
				if newItem > maxItem {
					// Wrap to top
					newItem = 0
				}
				issueList.SetCurrentItem(newItem)
				return nil
			}
			// Handle multi-key sequences FIRST before processing individual keys
			// This prevents conflicts with single-key handlers

			// Handle status shortcuts (S + second char)
			if lastKeyWasS {
				var newStatus string
				switch event.Rune() {
				case 'o':
					newStatus = "open"
				case 'i':
					newStatus = "in_progress"
				case 'b':
					newStatus = "blocked"
				case 'c':
					newStatus = "closed"
				default:
					// Invalid second key, reset and fall through
					lastKeyWasS = false
					statusBar.SetText(getStatusBarText())
					return nil
				}

				// Execute status update
				if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
					issueID := issue.ID
					args := []string{"update", issueID, "--status", newStatus}

					// Auto-claim: moving an unclaimed issue to in_progress also
					// assigns it to us, in the same bd call. Only when nobody
					// holds it — taking an issue off someone else silently would
					// be worse than leaving it as it is.
					claimed := false
					if newStatus == "in_progress" && issue.Assignee == "" && me != "" {
						args = append(args, "--assignee", me)
						claimed = true
					}

					log.Printf("BD COMMAND: Executing status update (S%c): bd %s", event.Rune(), strings.Join(args, " "))
					updatedIssue, err := execBdJSONIssue(args...)
					if err != nil {
						statusBar.SetText(errorMsg(fmt.Sprintf("Error updating status: %v", err)))
					} else {
						msg := fmt.Sprintf("✓ Set %s to %s", updatedIssue.ID, updatedIssue.Status)
						if claimed {
							msg += fmt.Sprintf(" and claimed it for %s", me)
						}
						// A status change leaves dependencies alone, so the
						// state from before the reload still answers this.
						if newStatus == "open" {
							msg += stillBlockedNote(appState, issueID)
						}
						statusBar.SetText(successMsg(msg))
						scheduleRefresh(issueID)
					}
				}
				lastKeyWasS = false
				return nil
			}

			// Reset the pending 'g' (gg) prefix on any rune other than 'g'.
			// Many rune handlers below return early, so relying on the outer
			// default case to clear it left the flag sticky after j/k/0-4/etc.,
			// causing a later single 'g' to be misread as 'gg'.
			if event.Rune() != 'g' {
				lastKeyWasG = false
			}

			// Normal single-key handling
			switch event.Rune() {
			case 'q':
				saveCollapseState() // Persist before exit
				app.Stop()
				return nil
			case 'r':
				// Manual refresh - run in goroutine to avoid blocking UI
				statusBar.SetText(fmt.Sprintf("[%s]Refreshing...[-]", formatting.GetEmphasisColor()))
				go refreshIssues()
				return nil
			case 'j':
				// Down - simulate down arrow
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				// Up - simulate up arrow
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'g':
				if lastKeyWasG {
					// gg - jump to top
					issueList.SetCurrentItem(0)
					lastKeyWasG = false
					return nil
				}
				lastKeyWasG = true
				return nil
			case 'G':
				// G - jump to bottom
				issueList.SetCurrentItem(issueList.GetItemCount() - 1)
				lastKeyWasG = false
				return nil
			case '/':
				// Start search mode
				searchMode = true
				searchQuery = ""
				statusBar.SetText(fmt.Sprintf("[%s]Search:[-] _", formatting.GetEmphasisColor()))
				return nil
			case 'n':
				// Next search result
				nextSearchMatch()
				return nil
			case 'N':
				// Previous search result
				prevSearchMatch()
				return nil
			case 't':
				// Cycle view mode: list -> tree -> created -> updated -> list
				appState.CycleViewMode()
				issueList.SetTitle(getIssueListTitle())
				statusBar.SetText(getStatusBarText())
				populateIssueList()
				return nil
			case 'o':
				// Toggle collapse for selected issue in tree view (vim-style fold)
				if appState.GetViewMode() == state.ViewTree {
					if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
						if appState.HasChildren(issue.ID) {
							isCollapsed := appState.ToggleCollapse(issue.ID)
							saveCollapseState() // Persist to disk
							populateIssueList()
							// Restore selection after repopulating
							for idx, iss := range indexToIssue {
								if iss.ID == issue.ID {
									issueList.SetCurrentItem(idx)
									break
								}
							}
							if isCollapsed {
								showTemporaryStatus(successMsg(fmt.Sprintf("✓ Collapsed %s", issue.ID)), statusMessageDuration)
							} else {
								showTemporaryStatus(successMsg(fmt.Sprintf("✓ Expanded %s", issue.ID)), statusMessageDuration)
							}
						} else {
							showTemporaryStatus(errorMsg("No children to collapse"), statusMessageDuration)
						}
					}
				}
				return nil
			case 'O':
				// Expand all nodes in tree view
				if appState.GetViewMode() == state.ViewTree {
					count := appState.ExpandAll()
					saveCollapseState()
					populateIssueList()
					if count > 0 {
						showTemporaryStatus(successMsg(fmt.Sprintf("✓ Expanded %d nodes", count)), statusMessageDuration)
					} else {
						showTemporaryStatus(successMsg("✓ All nodes already expanded"), statusMessageDuration)
					}
				}
				return nil
			case 'Z':
				// Collapse all nodes in tree view
				if appState.GetViewMode() == state.ViewTree {
					count := appState.CollapseAll()
					saveCollapseState()
					populateIssueList()
					if count > 0 {
						showTemporaryStatus(successMsg(fmt.Sprintf("✓ Collapsed %d nodes", count)), statusMessageDuration)
					} else {
						showTemporaryStatus(successMsg("✓ All nodes already collapsed"), statusMessageDuration)
					}
				}
				return nil
			case 'v':
				// Cycle the layout pin: auto -> Columns -> Rows -> Paged -> auto.
				switch {
				case !layoutPinned:
					layoutPin = layout.Columns
					layoutPinned = true
				case layoutPin == layout.Paged:
					layoutPinned = false
				default:
					layoutPin = layout.Next(layoutPin)
				}

				resolved := layout.Resolve(lastScreenWidth, layoutPin, layoutPinned)
				if resolved != currentLayout {
					currentLayout = resolved
					remountLayout()
					app.SetRoot(pages, true)
					updatePanelFocus()
				}

				// A pin the width overrides would otherwise look like a dead key.
				if layoutPinned && resolved != layoutPin {
					showTemporaryStatus(
						errorMsg(fmt.Sprintf("Pinned %s, but %d cols is too narrow - showing %s",
							layoutPin, lastScreenWidth, resolved)),
						statusMessageDuration)
				} else {
					statusBar.SetText(getStatusBarText())
				}
				return nil
			case 'C':
				// Toggle showing closed issues
				showClosedIssues = !showClosedIssues
				statusBar.SetText(getStatusBarText())
				populateIssueList()
				return nil
			case 'm':
				// Toggle mouse mode
				mouseEnabled = !mouseEnabled
				app.EnableMouse(mouseEnabled)
				statusBar.SetText(getStatusBarText())
				return nil
			case 'p':
				// Toggle issue ID prefix display
				showPrefix = !showPrefix
				populateIssueList()
				if showPrefix {
					showTemporaryStatus(successMsg("Prefix: shown"), statusMessageDuration)
				} else {
					showTemporaryStatus(successMsg("Prefix: hidden"), statusMessageDuration)
				}
				return nil
			case 'a':
				// Open issue creation dialog
				showCreateIssueDialog()
				return nil
			case 'A':
				// Set or clear the assignee
				showAssignDialog()
				return nil
			case 'e':
				// Edit issue fields
				showEditForm()
				return nil
			case 'D':
				// Open dependency management dialog
				showDependencyDialog()
				return nil
			case 'L':
				// Open label management dialog
				showLabelDialog()
				return nil
			case 'y':
				// Yank (copy) issue ID to clipboard
				if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
					copyToClipboard(issue.ID, fmt.Sprintf("%s to clipboard", issue.ID))
				}
				return nil
			case 'Y':
				// Yank (copy) issue ID with title to clipboard
				if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
					text := fmt.Sprintf("%s - %s", issue.ID, issue.Title)
					copyToClipboard(text, fmt.Sprintf("'%s' to clipboard", text))
				}
				return nil
			case 'B':
				// Copy git branch name to clipboard
				if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
					branchName := issue.ID // Simple format: just the issue ID
					copyToClipboard(branchName,
						fmt.Sprintf("branch name '%s' to clipboard", branchName))
				}
				return nil
			case 'R':
				// Rename issue (edit title)
				showRenameDialog()
				return nil
			case 'x':
				// Close issue with optional reason
				showCloseIssueDialog()
				return nil
			case 'X':
				// Reopen closed issue with optional reason
				showReopenIssueDialog()
				return nil
			case '?':
				// Show help screen
				showHelpScreen()
				return nil
			case 'f':
				// Show quick filter
				showQuickFilter()
				return nil
			case 'S':
				// Show stats dashboard
				showStatsOverlay()
				return nil
			case '0', '1', '2', '3', '4':
				// Quick priority change
				if issue, ok := indexToIssue[issueList.GetCurrentItem()]; ok {
					priority := int(event.Rune() - '0')
					issueID := issue.ID // Capture issue ID before refresh
					// Update priority via bd command with --json
					log.Printf("BD COMMAND: Executing priority update: bd update %s --priority %d", issueID, priority)
					updatedIssue, err := execBdJSONIssue("update", issueID, "--priority", fmt.Sprintf("%d", priority))
					if err != nil {
						log.Printf("BD COMMAND ERROR: Priority update failed: %v", err)
						statusBar.SetText(errorMsg(fmt.Sprintf("Error updating priority: %v", err)))
					} else {
						log.Printf("BD COMMAND: Priority update successful for %s -> P%d", updatedIssue.ID, updatedIssue.Priority)
						statusBar.SetText(successMsg(fmt.Sprintf("✓ Set %s to P%d", updatedIssue.ID, updatedIssue.Priority)))
						// Refresh issues after a short delay, preserving selection
						log.Printf("BD COMMAND: Scheduling refresh in 500ms")
						scheduleRefresh(issueID)
					}
				}
				return nil
			case 's':
				// Initiate status shortcut sequence
				lastKeyWasS = true
				statusBar.SetText(fmt.Sprintf("[%s]Status shortcut: o/i/b/c[-]", formatting.GetEmphasisColor()))
				// Reset after 2 seconds if no second key
				time.AfterFunc(statusMessageDuration, func() {
					safeQueueUpdateDraw(func() {
						if lastKeyWasS {
							lastKeyWasS = false
							statusBar.SetText(getStatusBarText())
						}
					})
				})
				return nil
			case 'c':
				// Add comment to issue
				showCommentDialog()
				return nil
			default:
				// Reset all multi-key flags if any other key is pressed
				lastKeyWasG = false
				lastKeyWasS = false
			}
		default:
			lastKeyWasG = false
		}
		return event
	})

	// Enable mouse by default (can be toggled with 'm' key)
	app.EnableMouse(mouseEnabled)

	// Set root and ensure issue list has focus initially
	app.SetRoot(pages, true)
	app.SetFocus(issueList)

	return t, nil
}

// Run starts the tview event loop and blocks until the application stops.
func (t *tui) Run() error {
	log.Printf("APP: Starting tview application main loop")
	if err := t.app.Run(); err != nil {
		log.Printf("APP ERROR: Application crashed: %v", err)
		return err
	}
	log.Printf("APP: Application exited normally")
	return nil
}

// Shutdown persists collapse state and stops the event loop. It is safe to
// call from a signal handler.
func (t *tui) Shutdown() {
	t.saveCollapseState()
	t.app.Stop()
}

// Close releases the file watcher and the JSONL reader. It is idempotent.
func (t *tui) Close() {
	t.closeOnce.Do(func() {
		if t.fileWatcher != nil {
			log.Printf("WATCHER: Stopping file watcher")
			_ = t.fileWatcher.Stop()
		}
		if t.reader != nil {
			_ = t.reader.Close()
		}
	})
}
