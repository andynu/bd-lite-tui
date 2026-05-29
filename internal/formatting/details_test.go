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
