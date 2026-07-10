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

	out := FormatIssueDetails(issue)

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

	out := FormatIssueDetails(issue)

	want := "  Created: 2026-07-09 12:07 by Andy Nutter-Upham\n"
	if !strings.Contains(out, want) {
		t.Errorf("expected details to contain %q, got:\n%s", want, out)
	}
}

func TestFormatIssueDetails_CreatedByAbsent(t *testing.T) {
	issue := createdByTestIssue()

	out := FormatIssueDetails(issue)

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

	out := FormatIssueDetails(issue)

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
