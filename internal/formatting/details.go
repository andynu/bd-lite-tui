package formatting

import (
	"fmt"
	"strings"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/rivo/tview"
)

// formatDependencyPhrase converts a dependency type to a human-readable phrase
// from the perspective of the issue that HAS the dependency.
// In bd-lite, dependencies are stored on the issue that is affected:
// - "blocks" on issue A pointing to B means "A is blocked by B"
// - "parent-child" on A pointing to B means "A is a child of B"
func formatDependencyPhrase(depType parser.DependencyType) string {
	switch depType {
	case parser.DepBlocks:
		return "blocked by"
	case parser.DepParentChild:
		return "child of"
	case parser.DepRelated:
		return "related to"
	case parser.DepDiscoveredFrom:
		return "discovered from"
	default:
		return string(depType)
	}
}

// formatDependencyLine renders one dependency as one logical line: the phrase,
// the target ID and, when lookup resolves the target, its status and title.
// The title can make the line wrap in a narrow panel, so mapping a click to a
// dependency has to word-wrap every line above it, not count one row each.
func formatDependencyLine(dep *parser.Dependency, lookup func(id string) *parser.Issue) string {
	line := fmt.Sprintf("  • [%s]%s[-] %s",
		GetDependencyColor(dep.Type), formatDependencyPhrase(dep.Type), tview.Escape(dep.DependsOnID))
	if lookup == nil {
		return line + "\n"
	}
	target := lookup(dep.DependsOnID)
	if target == nil {
		return line + fmt.Sprintf(" [%s](unknown)[-]\n", GetMutedColor())
	}
	title := tview.Escape(target.Title)
	if target.Status == parser.StatusClosed {
		title = fmt.Sprintf("[%s]%s[-]", GetMutedColor(), title)
	}
	return line + fmt.Sprintf(" [%s](%s)[-] %s\n", GetStatusColor(target.Status), target.Status, title)
}

// DetailLinks says which logical lines of a detail rendering (its text split on
// "\n") a click acts on. They are logical lines, not screen rows: the caller
// word-wraps each line to map a clicked row back to one of these.
type DetailLinks struct {
	// IDLine is the line holding the issue's own ID.
	IDLine int

	// Dependencies maps each dependency line to the ID of the issue it names.
	Dependencies map[int]string
}

// FormatIssueDetails formats full issue metadata for display in the detail
// panel. lookup resolves dependency targets so each line can say whether the
// target still blocks; nil leaves the lines as bare IDs.
func FormatIssueDetails(issue *parser.Issue, lookup func(id string) *parser.Issue) string {
	text, _ := FormatIssueDetailsWithLinks(issue, lookup)
	return text
}

// FormatIssueDetailsWithLinks is FormatIssueDetails plus where the clickable
// lines landed. Counting newlines as the text is built keeps the answer right
// when the title or description spans several lines of its own.
func FormatIssueDetailsWithLinks(issue *parser.Issue, lookup func(id string) *parser.Issue) (string, DetailLinks) {
	var result string
	links := DetailLinks{Dependencies: map[int]string{}}

	// Header
	priorityColor := GetPriorityColor(issue.Priority)
	statusColor := GetStatusColor(issue.Status)
	typeIcon := GetTypeIcon(issue.IssueType)

	mutedColor := GetMutedColor()
	accentColor := GetAccentColor()
	emphasisColor := GetEmphasisColor()

	// User-controlled fields are escaped so brackets are rendered literally
	// rather than being parsed as tview color/region tags.
	result += fmt.Sprintf("[::b]%s %s[-::-]\n", typeIcon, tview.Escape(issue.Title))
	links.IDLine = strings.Count(result, "\n")
	result += fmt.Sprintf("[%s]ID:[-] %s [%s](click to copy)[-]  ", mutedColor, tview.Escape(issue.ID), accentColor)
	result += fmt.Sprintf("[%s]P%d[-]  ", priorityColor, issue.Priority)
	result += fmt.Sprintf("[%s]%s[-]\n\n", statusColor, issue.Status)

	// Description
	if issue.Description != "" {
		result += fmt.Sprintf("[%s::b]Description:[-::-]\n", emphasisColor)
		result += tview.Escape(issue.Description) + "\n\n"
	}

	// Dependencies
	if len(issue.Dependencies) > 0 {
		result += fmt.Sprintf("[%s::b]Dependencies:[-::-]\n", emphasisColor)
		for _, dep := range issue.Dependencies {
			links.Dependencies[strings.Count(result, "\n")] = dep.DependsOnID
			result += formatDependencyLine(dep, lookup)
		}
		result += "\n"
	}

	// Labels
	if len(issue.Labels) > 0 {
		result += fmt.Sprintf("[%s::b]Labels:[-::-] ", emphasisColor)
		for i, label := range issue.Labels {
			if i > 0 {
				result += ", "
			}
			result += fmt.Sprintf("[%s]%s[-]", accentColor, tview.Escape(label))
		}
		result += "\n\n"
	}

	// Metadata
	result += fmt.Sprintf("[%s::b]Metadata:[-::-]\n", emphasisColor)
	created := issue.CreatedAt.Format("2006-01-02 15:04")
	if issue.CreatedBy != "" {
		created += " by " + issue.CreatedBy
	}
	result += fmt.Sprintf("  Created: %s\n", created)
	result += fmt.Sprintf("  Updated: %s\n", issue.UpdatedAt.Format("2006-01-02 15:04"))

	if issue.ClosedAt != nil {
		result += fmt.Sprintf("  Closed: %s\n", issue.ClosedAt.Format("2006-01-02 15:04"))
	}

	if issue.CloseReason != "" {
		result += fmt.Sprintf("  Close Reason: %s\n", tview.Escape(issue.CloseReason))
	}

	if issue.Assignee != "" {
		result += fmt.Sprintf("  Assignee: %s\n", tview.Escape(issue.Assignee))
	}

	// Comments
	if len(issue.Comments) > 0 {
		result += fmt.Sprintf("\n[%s::b]Comments:[-::-]\n", emphasisColor)
		for _, comment := range issue.Comments {
			result += fmt.Sprintf("  [%s]%s[-] (%s):\n", accentColor, tview.Escape(comment.Author), comment.CreatedAt.Format("2006-01-02 15:04"))
			result += fmt.Sprintf("    %s\n", tview.Escape(comment.Text))
		}
	}

	return result, links
}
