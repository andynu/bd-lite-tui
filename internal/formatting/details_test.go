package formatting

import (
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/parser"
	"github.com/rivo/tview"
)

// TestFormatIssueDetailsEscapesUserContent verifies that user-controlled fields
// containing square brackets are escaped so tview does not parse them as color
// or region tags (which would corrupt the rendered detail panel).
func TestFormatIssueDetailsEscapesUserContent(t *testing.T) {
	now := time.Now()
	issue := &parser.Issue{
		ID:          "test-1",
		Title:       "Fix [red] login bug",
		Description: "See [yellow]docs",
		Status:      parser.StatusOpen,
		Priority:    1,
		IssueType:   parser.TypeBug,
		Labels:      []string{"[urgent]"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	out := FormatIssueDetails(issue, nil)

	for _, raw := range []string{"Fix [red] login bug", "See [yellow]docs", "[urgent]"} {
		escaped := tview.Escape(raw)
		if !strings.Contains(out, escaped) {
			t.Errorf("expected escaped form %q in output", escaped)
		}
		// The unescaped tag-like substring must not survive verbatim.
		if strings.Contains(out, raw) {
			t.Errorf("unescaped user content %q leaked into output", raw)
		}
	}
}

func createdByTestIssue() *parser.Issue {
	created := time.Date(2026, 7, 9, 12, 7, 0, 0, time.UTC)
	return &parser.Issue{
		ID:        "tui-phfv",
		Title:     "Surface created_by in the issue detail pane",
		Status:    parser.StatusOpen,
		Priority:  2,
		IssueType: parser.TypeFeature,
		CreatedAt: created,
		UpdatedAt: created.Add(3 * time.Minute),
	}
}

func TestFormatIssueDetails_CreatedByPresent(t *testing.T) {
	issue := createdByTestIssue()
	issue.CreatedBy = "Andy Nutter-Upham"

	out := FormatIssueDetails(issue, nil)

	want := "  Created: 2026-07-09 12:07 by Andy Nutter-Upham\n"
	if !strings.Contains(out, want) {
		t.Errorf("expected details to contain %q, got:\n%s", want, out)
	}
}

func TestFormatIssueDetails_CreatedByAbsent(t *testing.T) {
	issue := createdByTestIssue()

	out := FormatIssueDetails(issue, nil)

	// An issue with no created_by must render exactly as it did before the
	// field existed: the timestamp alone, with no trailing " by".
	want := "  Created: 2026-07-09 12:07\n"
	if !strings.Contains(out, want) {
		t.Errorf("expected details to contain %q, got:\n%s", want, out)
	}
	if strings.Contains(out, "Created: 2026-07-09 12:07 by") {
		t.Errorf("expected no 'by' suffix when CreatedBy is empty, got:\n%s", out)
	}
}

func TestFormatIssueDetails_CreatedByDoesNotDisplaceUpdated(t *testing.T) {
	issue := createdByTestIssue()
	issue.CreatedBy = "Andy Nutter-Upham"

	out := FormatIssueDetails(issue, nil)

	createdIdx := strings.Index(out, "  Created:")
	updatedIdx := strings.Index(out, "  Updated:")
	if createdIdx == -1 || updatedIdx == -1 {
		t.Fatalf("expected both Created and Updated lines, got:\n%s", out)
	}
	if createdIdx > updatedIdx {
		t.Errorf("expected Created before Updated, got:\n%s", out)
	}
	if !strings.Contains(out, "  Updated: 2026-07-09 12:10\n") {
		t.Errorf("expected unmodified Updated line, got:\n%s", out)
	}
}

// stripTags renders tview markup the way the detail panel would, leaving the
// text a user reads.
func stripTags(s string) string {
	return tview.NewTextView().SetDynamicColors(true).SetText(s).GetText(true)
}

func dependencyTestIssue() *parser.Issue {
	issue := createdByTestIssue()
	for _, target := range []string{"tui-open", "tui-done", "tui-gone"} {
		issue.Dependencies = append(issue.Dependencies, &parser.Dependency{
			IssueID: issue.ID, DependsOnID: target, Type: parser.DepBlocks,
		})
	}
	return issue
}

// A dependency line names its target's status and title, so the panel alone
// says whether the dependency still blocks. A target the lookup cannot resolve
// is marked rather than shown as if it were fine.
func TestFormatIssueDetails_DependencyLinesNameTargetStatusAndTitle(t *testing.T) {
	targets := map[string]*parser.Issue{
		"tui-open": {ID: "tui-open", Title: "Still [open] work", Status: parser.StatusOpen},
		"tui-done": {ID: "tui-done", Title: "Finished work", Status: parser.StatusClosed},
	}
	lookup := func(id string) *parser.Issue { return targets[id] }

	out := stripTags(FormatIssueDetails(dependencyTestIssue(), lookup))

	for _, want := range []string{
		"  • blocked by tui-open (open) Still [open] work\n",
		"  • blocked by tui-done (closed) Finished work\n",
		"  • blocked by tui-gone (unknown)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected details to contain %q, got:\n%s", want, out)
		}
	}
}

// With no lookup there is nothing to resolve against, so the lines keep their
// bare form instead of calling every target unknown.
func TestFormatIssueDetails_DependencyLinesWithoutLookup(t *testing.T) {
	out := stripTags(FormatIssueDetails(dependencyTestIssue(), nil))

	for _, want := range []string{
		"  • blocked by tui-open\n",
		"  • blocked by tui-done\n",
		"  • blocked by tui-gone\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected details to contain %q, got:\n%s", want, out)
		}
	}
}
