package ui

import (
	"fmt"
	"time"

	"github.com/andynu/bd-lite-tui/internal/formatting"
	"github.com/andynu/bd-lite-tui/internal/identity"
	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/andynu/bd-lite-tui/internal/state"
	"github.com/rivo/tview"
)

// PopulateIssueList clears and rebuilds the issue list from state
// Updates the provided indexToIssue map in place to avoid stale pointer issues
//
// now is the moment the relative time buckets ("Today", "This week") are drawn
// against. It is a parameter rather than a clock read so the grouping is
// reproducible in a test.
func PopulateIssueList(
	issueList *tview.List,
	appState *state.State,
	showClosedIssues bool,
	showPrefix bool,
	now time.Time,
	indexToIssue map[int]*parser.Issue,
) {
	// Clear and rebuild issue list
	issueList.Clear()

	// Clear the map in place (don't create a new one)
	for k := range indexToIssue {
		delete(indexToIssue, k)
	}
	currentIndex := 0

	// Rows name whoever filed an issue only when that is not me, so the row
	// formatter needs to know who I am.
	me := appState.Identity()

	// Show filter indicator when filters are active
	if appState.HasActiveFilters() {
		warningColor := formatting.GetWarningColor()
		emphasisColor := formatting.GetEmphasisColor()
		issueList.AddItem(fmt.Sprintf("[%s::b]⊘ FILTERED[-::-] [%s]%s[-] — press f to modify",
			warningColor, emphasisColor, appState.GetActiveFilters()), "", 0, nil)
		currentIndex++
	}

	// Check view mode.
	switch appState.GetViewMode() {
	case state.ViewTree:
		// Tree view
		accentColor := formatting.GetAccentColor()
		issueList.AddItem(fmt.Sprintf("[%s::b]DEPENDENCY TREE[-::-]", accentColor), "", 0, nil)
		currentIndex++

		treeNodes := appState.GetTreeNodes()
		for i, node := range treeNodes {
			isLast := i == len(treeNodes)-1
			renderTreeNode(issueList, appState, node, "", isLast, showPrefix, &currentIndex, indexToIssue)
		}
	case state.ViewCreated:
		groups := appState.GroupByTime(state.CreatedAt, now, showClosedIssues)
		renderTimeGroups(issueList, appState, groups, showPrefix, &currentIndex, indexToIssue)
	case state.ViewUpdated:
		groups := appState.GroupByTime(state.UpdatedAt, now, showClosedIssues)
		renderTimeGroups(issueList, appState, groups, showPrefix, &currentIndex, indexToIssue)
	default:
		// List view (original behavior)
		// Add in-progress issues first (most important)
		inProgressIssues := appState.GetInProgressIssues()
		if len(inProgressIssues) > 0 {
			inProgressColor := formatting.GetStatusColor(parser.StatusInProgress)
			issueList.AddItem(fmt.Sprintf("[%s::b]⬤ IN PROGRESS (%d)[-::-]", inProgressColor, len(inProgressIssues)), "", 0, nil)
			currentIndex++

			for _, issue := range inProgressIssues {
				text := formatIssueListItem(issue, "◆", showPrefix, me)
				issueList.AddItem(text, "", 0, nil)
				indexToIssue[currentIndex] = issue
				currentIndex++
			}
		}

		// Add ready issues
		readyIssues := appState.GetReadyIssues()
		if len(readyIssues) > 0 {
			openColor := formatting.GetStatusColor(parser.StatusOpen)
			issueList.AddItem(fmt.Sprintf("\n[%s::b]⬤ READY (%d)[-::-]", openColor, len(readyIssues)), "", 0, nil)
			currentIndex++

			for _, issue := range readyIssues {
				text := formatIssueListItem(issue, "●", showPrefix, me)
				issueList.AddItem(text, "", 0, nil)
				indexToIssue[currentIndex] = issue
				currentIndex++
			}
		}

		// Add blocked issues
		blockedIssues := appState.GetBlockedIssues()
		if len(blockedIssues) > 0 {
			blockedColor := formatting.GetStatusColor(parser.StatusBlocked)
			issueList.AddItem(fmt.Sprintf("\n[%s::b]⬤ BLOCKED (%d)[-::-]", blockedColor, len(blockedIssues)), "", 0, nil)
			currentIndex++

			for _, issue := range blockedIssues {
				text := formatIssueListItem(issue, "○", showPrefix, me)
				issueList.AddItem(text, "", 0, nil)
				indexToIssue[currentIndex] = issue
				currentIndex++
			}
		}

		// Add closed issues (only if showClosedIssues is enabled)
		if showClosedIssues {
			closedIssues := appState.GetClosedIssues()
			if len(closedIssues) > 0 {
				closedColor := formatting.GetStatusColor(parser.StatusClosed)
				issueList.AddItem(fmt.Sprintf("\n[%s::b]⬤ CLOSED (%d)[-::-]", closedColor, len(closedIssues)), "", 0, nil)
				currentIndex++

				for _, issue := range closedIssues {
					text := formatIssueListItem(issue, "✓", showPrefix, me)
					issueList.AddItem(text, "", 0, nil)
					indexToIssue[currentIndex] = issue
					currentIndex++
				}
			}
		}
	}

	// Show helpful message when no issues are visible
	if len(indexToIssue) == 0 {
		mutedColor := formatting.GetMutedColor()
		emphasisColor := formatting.GetEmphasisColor()
		if appState.HasActiveFilters() {
			issueList.AddItem(fmt.Sprintf("\n  [%s]No issues match current filters[-]", mutedColor), "", 0, nil)
			issueList.AddItem(fmt.Sprintf("  [%s]Press 'f' to modify filters[-]", emphasisColor), "", 0, nil)
		} else {
			issueList.AddItem(fmt.Sprintf("\n  [%s]No issues found[-]", mutedColor), "", 0, nil)
			issueList.AddItem(fmt.Sprintf("  [%s]Press 'a' to create an issue[-]", emphasisColor), "", 0, nil)
		}
	}
}

// formatIssueListItem formats a single issue for the list view. me is the
// identity the TUI acts as, which decides whether the filer's name is worth a
// place on the row; see formatCreatorSuffix.
func formatIssueListItem(issue *parser.Issue, statusIcon string, showPrefix bool, me string) string {
	priorityColor := formatting.GetPriorityColor(issue.Priority)
	typeIcon := formatting.GetTypeIcon(issue.IssueType)
	displayID := formatting.FormatIssueID(issue.ID, showPrefix)
	// Escape user-controlled fields so brackets in titles/labels are not parsed
	// as tview color/region tags (which would corrupt display and search).
	// "[P1[]" is tview's escape for a literal "[P1]" — written bare, the tag
	// parser reads "P1" as a color name and swallows the whole thing (tui-6ajo).
	text := fmt.Sprintf("  [%s]%s[-] %s %s [P%d[] %s",
		priorityColor, statusIcon, typeIcon, tview.Escape(displayID), issue.Priority, tview.Escape(issue.Title))

	text += formatCreatorSuffix(issue, me)
	text += formatAssigneeSuffix(issue)

	// Add labels if present
	if len(issue.Labels) > 0 {
		mutedColor := formatting.GetMutedColor()
		text += fmt.Sprintf(" [%s]", mutedColor)
		for i, label := range issue.Labels {
			if i > 0 {
				text += " "
			}
			text += "#" + tview.Escape(label)
		}
		text += "[-]"
	}

	return text
}

// formatAssigneeSuffix renders the assignee as a dimmed "@name", or "" when the
// issue is unassigned. Assignee values are user-controlled, so they are escaped
// for the same reason titles and labels are.
func formatAssigneeSuffix(issue *parser.Issue) string {
	if issue.Assignee == "" {
		return ""
	}
	return fmt.Sprintf(" [%s]@%s[-]", formatting.GetMutedColor(), tview.Escape(issue.Assignee))
}

// formatCreatorSuffix renders who filed the issue as a dimmed "+name", and ""
// when that is me or when the issue carries no created_by.
//
// Unlike the assignee, my own name is suppressed on purpose (tui-dfqm). Every
// issue has a filer, so printing all of them spends a name's width on every row
// to repeat what is true of nearly all of them; someone else's name is the whole
// reason the field earns a place on the row. An issue with no created_by — one
// bd-lite wrote before it stamped the field — renders nothing rather than
// guessing a name onto it. With no identity resolved, every filer is a stranger
// and every name shows, which is the honest reading of not knowing who I am.
//
// created_by is user-controlled, so it is escaped for the same reason titles,
// labels and assignees are.
func formatCreatorSuffix(issue *parser.Issue, me string) string {
	if issue.CreatedBy == "" || identity.IsMine(issue.CreatedBy, me) {
		return ""
	}
	return fmt.Sprintf(" [%s]+%s[-]", formatting.GetMutedColor(), tview.Escape(issue.CreatedBy))
}

// issueStatus returns the glyph and color for an issue's effective status.
//
// The status-grouped list view knows an issue's status from the group it is
// rendering, but the tree and the time-keyed views interleave statuses, so they
// derive it per row. Effective blocking is what matters: an issue held up by an
// open dependency shows as blocked even though its own status is "open".
func issueStatus(appState *state.State, issue *parser.Issue) (icon, color string) {
	switch {
	case issue.Status == parser.StatusClosed:
		return "✓", formatting.GetStatusColor(parser.StatusClosed)
	case issue.Status == parser.StatusInProgress:
		return "◆", formatting.GetStatusColor(parser.StatusInProgress)
	case appState.IsEffectivelyBlocked(issue.ID):
		return "○", formatting.GetStatusColor(parser.StatusBlocked)
	default:
		return "●", formatting.GetStatusColor(parser.StatusOpen)
	}
}

// renderTreeNode recursively renders a tree node and its children
func renderTreeNode(
	issueList *tview.List,
	appState *state.State,
	node *state.TreeNode,
	prefix string,
	isLast bool,
	showPrefix bool,
	currentIndex *int,
	indexToIssue map[int]*parser.Issue,
) {
	issue := node.Issue
	hasChildren := len(node.Children) > 0
	isCollapsed := appState.IsCollapsed(issue.ID)

	// Determine branch characters
	var branch, continuation string
	if node.Depth == 0 {
		branch = ""
		continuation = ""
	} else {
		if isLast {
			branch = "└── "
			continuation = "    "
		} else {
			branch = "├── "
			continuation = "│   "
		}
	}

	statusIcon, statusColor := issueStatus(appState, issue)

	// Add collapse indicator for parent nodes
	collapseIndicator := ""
	if hasChildren {
		if isCollapsed {
			collapseIndicator = "▶ " // Collapsed - can expand
		} else {
			collapseIndicator = "▼ " // Expanded - can collapse
		}
	} else {
		collapseIndicator = "  " // Leaf node - no indicator (maintain alignment)
	}

	// Format issue line
	priorityColor := formatting.GetPriorityColor(issue.Priority)
	typeIcon := formatting.GetTypeIcon(issue.IssueType)
	displayID := formatting.FormatIssueID(issue.ID, showPrefix)
	// "[P%d[]" renders a literal "[P1]"; see formatIssueListItem (tui-6ajo).
	text := fmt.Sprintf("%s%s%s[%s]%s[-] %s [%s]%s[-] [P%d[] %s",
		prefix, branch, collapseIndicator, statusColor, statusIcon, typeIcon, priorityColor, tview.Escape(displayID), issue.Priority, tview.Escape(issue.Title))

	text += formatCreatorSuffix(issue, appState.Identity())
	text += formatAssigneeSuffix(issue)

	// Add child count for collapsed nodes
	if hasChildren && isCollapsed {
		mutedColor := formatting.GetMutedColor()
		text += fmt.Sprintf(" [%s](%d children)[-]", mutedColor, len(node.Children))
	}

	// Add labels if present
	if len(issue.Labels) > 0 {
		mutedColor := formatting.GetMutedColor()
		text += fmt.Sprintf(" [%s]", mutedColor)
		for i, label := range issue.Labels {
			if i > 0 {
				text += " "
			}
			text += "#" + tview.Escape(label)
		}
		text += "[-]"
	}

	issueList.AddItem(text, "", 0, nil)
	indexToIssue[*currentIndex] = issue
	*currentIndex++

	// Render children only if not collapsed
	if !isCollapsed {
		for i, child := range node.Children {
			isLastChild := i == len(node.Children)-1
			newPrefix := prefix + continuation
			renderTreeNode(issueList, appState, child, newPrefix, isLastChild, showPrefix, currentIndex, indexToIssue)
		}
	}
}
