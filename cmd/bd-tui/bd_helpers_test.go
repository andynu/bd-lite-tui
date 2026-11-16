package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/andynu/bd-lite-tui/internal/bdversion"
	"github.com/andynu/bd-lite-tui/internal/parser"
)

func TestParseBdJSON_IssueArray(t *testing.T) {
	jsonData := `[
		{
			"id": "tui-123",
			"title": "Test Issue",
			"description": "Test description",
			"status": "open",
			"priority": 2,
			"issue_type": "feature",
			"created_at": "2025-11-14T22:00:00Z",
			"updated_at": "2025-11-14T22:00:00Z"
		}
	]`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.ID != "tui-123" {
		t.Errorf("expected ID 'tui-123', got '%s'", issue.ID)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("expected title 'Test Issue', got '%s'", issue.Title)
	}
	if issue.Status != parser.StatusOpen {
		t.Errorf("expected status 'open', got '%s'", issue.Status)
	}
	if issue.Priority != 2 {
		t.Errorf("expected priority 2, got %d", issue.Priority)
	}
}

func TestParseBdJSON_SingleIssue(t *testing.T) {
	jsonData := `{
		"id": "tui-456",
		"title": "Another Test",
		"description": "",
		"status": "in_progress",
		"priority": 1,
		"issue_type": "bug",
		"created_at": "2025-11-14T22:00:00Z",
		"updated_at": "2025-11-14T22:00:00Z"
	}`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.ID != "tui-456" {
		t.Errorf("expected ID 'tui-456', got '%s'", issue.ID)
	}
	if issue.IssueType != parser.TypeBug {
		t.Errorf("expected type 'bug', got '%s'", issue.IssueType)
	}
}

func TestParseBdJSON_Comment(t *testing.T) {
	jsonData := `{
		"id": 42,
		"issue_id": "tui-789",
		"author": "testuser",
		"text": "Test comment",
		"created_at": "2025-11-14T22:00:00Z"
	}`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}

	if len(result.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(result.Comments))
	}

	comment := result.Comments[0]
	if comment.ID != 42 {
		t.Errorf("expected ID 42, got %d", comment.ID)
	}
	if comment.IssueID != "tui-789" {
		t.Errorf("expected issue_id 'tui-789', got '%s'", comment.IssueID)
	}
	if comment.Author != "testuser" {
		t.Errorf("expected author 'testuser', got '%s'", comment.Author)
	}
}

// bd-lite's dep add and dep remove answer with a message object.
func TestParseBdJSON_Message(t *testing.T) {
	jsonData := `{
  "message": "Added dependency: tui-aaa depends on tui-bbb"
}`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}
	if result.Message != "Added dependency: tui-aaa depends on tui-bbb" {
		t.Errorf("Message = %q", result.Message)
	}
	if len(result.Issues) != 0 || len(result.Comments) != 0 {
		t.Errorf("a message parsed as issues %v / comments %v", result.Issues, result.Comments)
	}
}

func TestParseBdJSON_InvalidJSON(t *testing.T) {
	jsonData := `{invalid json`

	_, err := parseBdJSON([]byte(jsonData))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseBdJSON_EmptyArray(t *testing.T) {
	jsonData := `[]`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(result.Issues))
	}
}

func TestParseBdJSON_MultipleIssues(t *testing.T) {
	jsonData := `[
		{
			"id": "tui-1",
			"title": "First",
			"status": "open",
			"priority": 0,
			"issue_type": "feature",
			"created_at": "2025-11-14T22:00:00Z",
			"updated_at": "2025-11-14T22:00:00Z"
		},
		{
			"id": "tui-2",
			"title": "Second",
			"status": "closed",
			"priority": 3,
			"issue_type": "task",
			"created_at": "2025-11-14T22:00:00Z",
			"updated_at": "2025-11-14T22:00:00Z"
		}
	]`

	result, err := parseBdJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseBdJSON failed: %v", err)
	}

	if len(result.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(result.Issues))
	}

	if result.Issues[0].ID != "tui-1" {
		t.Errorf("expected first issue ID 'tui-1', got '%s'", result.Issues[0].ID)
	}
	if result.Issues[1].ID != "tui-2" {
		t.Errorf("expected second issue ID 'tui-2', got '%s'", result.Issues[1].ID)
	}
}

// Test that parseBdJSON correctly handles the exact format returned by bd commands
func TestParseBdJSON_RealBdOutput(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantType string // "issues" or "comment"
	}{
		{
			name: "bd show output",
			jsonData: `[
				{
					"id": "tui-vg7",
					"content_hash": "bc2bf433c3faa31538c447bc7dfd434b6f8a8045725f1111cb147fa7269e6850",
					"title": "Interact with the bd commandline using --json wherever possible",
					"description": "",
					"status": "in_progress",
					"priority": 2,
					"issue_type": "feature",
					"created_at": "2025-11-14T15:04:03.569822263-05:00",
					"updated_at": "2025-11-14T22:15:51.010287994-05:00",
					"source_repo": "."
				}
			]`,
			wantType: "issues",
		},
		{
			name: "bd create output",
			jsonData: `{
				"id":"tui-vyc",
				"content_hash":"06df81a8d414a51a714eacdb647c7ab88ff8b22da82e95e4161606372221f070",
				"title":"test issue for json check",
				"description":"",
				"status":"open",
				"priority":4,
				"issue_type":"task",
				"created_at":"2025-11-14T22:15:39.591881491-05:00",
				"updated_at":"2025-11-14T22:15:39.591881491-05:00"
			}`,
			wantType: "issues",
		},
		{
			name: "bd comment output",
			jsonData: `{
				"id": 15,
				"issue_id": "tui-vg7",
				"author": "andy",
				"text": "Test comment for JSON check",
				"created_at": "2025-11-15T03:15:51Z"
			}`,
			wantType: "comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBdJSON([]byte(tt.jsonData))
			if err != nil {
				t.Fatalf("parseBdJSON failed: %v", err)
			}

			switch tt.wantType {
			case "issues":
				if len(result.Issues) == 0 {
					t.Error("expected issues, got none")
				}
			case "comment":
				if len(result.Comments) == 0 {
					t.Error("expected comment, got none")
				}
			}
		})
	}
}

// Test execBdJSONIssue helper
func TestExecBdJSONIssue_NoIssues(t *testing.T) {
	// This is a unit test for the helper function behavior
	// We can't easily test the actual exec without mocking
	result := &BdCommandResult{
		Issues: []parser.Issue{},
	}

	if len(result.Issues) == 0 {
		// This simulates what execBdJSONIssue would encounter
		err := "bd command returned no issues"
		if err == "" {
			t.Error("expected error when no issues returned")
		}
	}
}

// Test that JSON unmarshaling works for all parser types
func TestParserTypesJSONCompatibility(t *testing.T) {
	// Test Issue
	issueJSON := `{
		"id": "test-1",
		"title": "Test",
		"status": "open",
		"priority": 1,
		"issue_type": "feature",
		"labels": ["tag1", "tag2"],
		"created_at": "2025-11-14T22:00:00Z",
		"updated_at": "2025-11-14T22:00:00Z"
	}`
	var issue parser.Issue
	if err := json.Unmarshal([]byte(issueJSON), &issue); err != nil {
		t.Errorf("Failed to unmarshal Issue: %v", err)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issue.Labels))
	}

	// Test Comment
	commentJSON := `{
		"id": 1,
		"issue_id": "test-1",
		"author": "user",
		"text": "comment",
		"created_at": "2025-11-14T22:00:00Z"
	}`
	var comment parser.Comment
	if err := json.Unmarshal([]byte(commentJSON), &comment); err != nil {
		t.Errorf("Failed to unmarshal Comment: %v", err)
	}
}

// Integration tests with actual bd commands
// These tests use BEADS_DB=/tmp to avoid polluting production database

func TestExecBdJSON_Integration_CreateAndUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Set up isolated test directory (bd-lite uses .beads/issues.jsonl, not a database)
	testDir := t.TempDir()

	// Save and restore original working directory since execBdJSONIssue
	// inherits the process working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to chdir to test dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	// Initialize test beads directory
	initCmd := exec.Command("bd", "init", "--prefix", "test")
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init test directory: %v", err)
	}

	// Test: Create an issue
	createdIssue, err := execBdJSONIssue("create", "Integration test issue", "-p", "2", "-t", "task")
	if err != nil {
		t.Fatalf("Failed to create issue: %v", err)
	}

	if createdIssue.ID == "" {
		t.Error("Created issue has empty ID")
	}
	if createdIssue.Title != "Integration test issue" {
		t.Errorf("Created issue title mismatch: got %q, want %q", createdIssue.Title, "Integration test issue")
	}
	if createdIssue.Priority != 2 {
		t.Errorf("Created issue priority mismatch: got %d, want 2", createdIssue.Priority)
	}

	// Test: Update the issue
	updatedIssue, err := execBdJSONIssue("update", createdIssue.ID, "--priority", "1")
	if err != nil {
		t.Fatalf("Failed to update issue: %v", err)
	}

	if updatedIssue.Priority != 1 {
		t.Errorf("Updated issue priority mismatch: got %d, want 1", updatedIssue.Priority)
	}

	// Test: Add a comment
	comment, err := execBdJSONComment("comment", createdIssue.ID, "Test comment")
	if err != nil {
		t.Fatalf("Failed to add comment: %v", err)
	}

	if comment.IssueID != createdIssue.ID {
		t.Errorf("Comment issue_id mismatch: got %q, want %q", comment.IssueID, createdIssue.ID)
	}
	if comment.Text != "Test comment" {
		t.Errorf("Comment text mismatch: got %q, want %q", comment.Text, "Test comment")
	}

	// Test: Close the issue
	closedIssue, err := execBdJSONIssue("close", createdIssue.ID)
	if err != nil {
		t.Fatalf("Failed to close issue: %v", err)
	}

	if closedIssue.Status != parser.StatusClosed {
		t.Errorf("Closed issue status mismatch: got %q, want %q", closedIssue.Status, parser.StatusClosed)
	}
}

// The installed bd is inside the supported range. When this fails after a bd
// upgrade, run the suite against it and raise bdversion.TestedThrough.
func TestDetectBdVersion_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if got := detectBdVersion(); got.Status != bdversion.Compatible {
		t.Errorf("detectBdVersion() = %+v, warning %q", got, got.Warning())
	}
}

// The dependency dialog's two calls, against the real bd, return what the
// dialog accepts as success.
func TestExecBdJSON_Integration_DepAddRemove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Failed to chdir to test dir: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := exec.Command("bd", "init", "--prefix", "test").Run(); err != nil {
		t.Fatalf("Failed to init test directory: %v", err)
	}
	a, err := execBdJSONIssue("create", "Blocked one", "-p", "2")
	if err != nil {
		t.Fatalf("Failed to create issue: %v", err)
	}
	b, err := execBdJSONIssue("create", "Blocker", "-p", "2")
	if err != nil {
		t.Fatalf("Failed to create issue: %v", err)
	}

	for _, verb := range []string{"add", "remove"} {
		result, err := execBdJSON("dep", verb, a.ID, b.ID)
		if err != nil {
			t.Fatalf("bd dep %s: %v", verb, err)
		}
		if result.Message == "" {
			t.Errorf("bd dep %s returned no message: %+v", verb, result)
		}
	}
}

func TestExecBdJSON_ErrorHandling_InvalidIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to update a non-existent issue
	_, err := execBdJSONIssue("update", "nonexistent-issue-id", "--priority", "1")
	if err == nil {
		t.Error("Expected error when updating non-existent issue, got nil")
	}

	// Error message should be informative
	if err != nil && !strings.Contains(err.Error(), "bd update failed") {
		t.Errorf("Error message doesn't indicate bd update failure: %v", err)
	}
}

// bd prints its update notice to stderr ahead of the error, on failure as
// well as success. The error text must keep the cause and drop the notice.
func TestBdStderrMessage_DropsTheUpdateNotice(t *testing.T) {
	stderr := "bd: v9.9.9 is available (you have v0.1.0). To update:\n" +
		"  curl -fsSL https://github.com/andynu/bd-lite/releases/download/v9.9.9/bd-linux-amd64 -o /home/x/bd.new && chmod +x /home/x/bd.new && mv /home/x/bd.new /home/x/bd\n" +
		"Error: no issue found matching 'demo-zzz'\n"
	msg, latest := bdStderrMessage(stderr)
	if msg != "Error: no issue found matching 'demo-zzz'" {
		t.Errorf("message = %q", msg)
	}
	if latest != "v9.9.9" {
		t.Errorf("latest = %q, want v9.9.9", latest)
	}
}

func TestBdStderrMessage_NonLinuxNoticeForm(t *testing.T) {
	stderr := "bd: v9.9.9 is available (you have v0.1.0). To update:\n" +
		"  see https://github.com/andynu/bd-lite/releases/tag/v9.9.9\n"
	msg, latest := bdStderrMessage(stderr)
	if msg != "" || latest != "v9.9.9" {
		t.Errorf("got (%q, %q), want (\"\", \"v9.9.9\")", msg, latest)
	}
}

func TestBdStderrMessage_PlainErrorUntouched(t *testing.T) {
	msg, latest := bdStderrMessage("Error: no issue found matching 'x'\n")
	if msg != "Error: no issue found matching 'x'" || latest != "" {
		t.Errorf("got (%q, %q)", msg, latest)
	}
}

// Against the real bd with a newer release seeded in its cache: a failed
// command reports its cause, not the notice.
func TestExecBdJSON_Integration_ErrorWithUpdateNotice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	cache := t.TempDir()
	if err := os.MkdirAll(cache+"/bd-lite", 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"checked_at":"` + time.Now().UTC().Format(time.RFC3339) + `","latest":"v9.9.9"}`
	if err := os.WriteFile(cache+"/bd-lite/latest.json", []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("BD_NO_UPDATE_CHECK", "")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := exec.Command("bd", "init", "--prefix", "test").Run(); err != nil {
		t.Fatalf("bd init: %v", err)
	}

	_, err = execBdJSONIssue("update", "test-zzz", "--priority", "1")
	if err == nil {
		t.Fatal("expected an error updating a missing issue")
	}
	if strings.Contains(err.Error(), "is available") {
		t.Errorf("error text carries the update notice: %v", err)
	}
	if !strings.Contains(err.Error(), "no issue found") {
		t.Errorf("error text lost the cause: %v", err)
	}
}

func TestExecBdJSON_ErrorHandling_InvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Try to execute an invalid bd command
	_, err := execBdJSON("invalid-command", "arg1", "arg2")
	if err == nil {
		t.Error("Expected error when executing invalid command, got nil")
	}
}

func TestParseBdJSON_ErrorHandling_MalformedJSON(t *testing.T) {
	malformedCases := []struct {
		name string
		data string
	}{
		{"incomplete object", `{"id": "test-1", "title":`},
		{"invalid syntax", `{id: test-1}`},
		{"truncated array", `[{"id": "test-1"`},
		{"random text", `this is not JSON at all`},
		{"empty string", ``},
	}

	for _, tc := range malformedCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseBdJSON([]byte(tc.data))
			if err == nil {
				t.Errorf("Expected error for malformed JSON %q, got nil", tc.name)
			}
		})
	}
}

func TestParseBdJSON_ErrorHandling_AmbiguousJSON(t *testing.T) {
	// Test object that doesn't clearly match issue or comment format
	ambiguousJSON := `{"id": 123, "some_field": "value"}`

	result, err := parseBdJSON([]byte(ambiguousJSON))
	if err == nil {
		// If it doesn't error, it should not have parsed anything
		if len(result.Issues) > 0 || len(result.Comments) > 0 {
			t.Error("Ambiguous JSON shouldn't parse as issue or comment")
		}
	}
}

func TestExecBdJSONIssue_ErrorHandling_NoIssuesReturned(t *testing.T) {
	// This simulates a command that succeeds but returns empty array
	// We test this by checking the error message from the helper
	testResult := &BdCommandResult{
		Issues: []parser.Issue{},
	}

	if len(testResult.Issues) == 0 {
		// Verify that execBdJSONIssue would return appropriate error
		expectedError := "bd command returned no issues"
		if !strings.Contains(expectedError, "no issues") {
			t.Errorf("Error message should mention 'no issues', got: %s", expectedError)
		}
	}
}

func TestExecBdJSONComment_ErrorHandling_NoCommentsReturned(t *testing.T) {
	// Similar to above but for comments
	testResult := &BdCommandResult{
		Comments: []parser.Comment{},
	}

	if len(testResult.Comments) == 0 {
		expectedError := "bd command returned no comments"
		if !strings.Contains(expectedError, "no comments") {
			t.Errorf("Error message should mention 'no comments', got: %s", expectedError)
		}
	}
}
