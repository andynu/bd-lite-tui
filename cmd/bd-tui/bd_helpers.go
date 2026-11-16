package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/andynu/bd-lite-tui/internal/bdversion"
	"github.com/andynu/bd-lite-tui/internal/parser"
)

// BdCommandResult represents the result of executing a bd command with --json
type BdCommandResult struct {
	Issues   []parser.Issue   `json:"issues,omitempty"`
	Comments []parser.Comment `json:"comments,omitempty"`
	// Message is the whole answer for commands that change state without
	// returning an issue: bd-lite's dep add and dep remove.
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// execBdJSON runs a bd command and parses its JSON response. It is a variable
// rather than a plain function so tests can substitute a fake bd: every issue
// mutation in the TUI funnels through here, so replacing it isolates the whole
// UI from the real CLI and from the real issues.jsonl.
var execBdJSON = execBdJSONCmd

// execBdJSONCmd executes a bd command with --json flag and parses the response.
// It handles both single object and array responses from bd commands.
//
// Example usage:
//
//	result, err := execBdJSON("update", "tui-123", "--priority", "1")
//	if err != nil { ... }
//	if len(result.Issues) > 0 {
//	  updatedIssue := result.Issues[0]
//	}
func execBdJSONCmd(args ...string) (*BdCommandResult, error) {
	// Add --json flag if not already present
	hasJSON := false
	for _, arg := range args {
		if arg == "--json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		args = append(args, "--json")
	}

	// Create context with timeout to prevent hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execute command with timeout, capturing stdout and stderr separately
	// This is important because bd may write warnings to stderr (e.g., deprecation
	// warnings, daemon warnings) which would corrupt the JSON output if combined
	cmd := exec.CommandContext(ctx, "bd", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// Check for timeout error specifically
	if ctx.Err() == context.DeadlineExceeded {
		cmdStr := "bd " + strings.Join(args, " ")
		return nil, fmt.Errorf("bd command timed out after 10s: %s", cmdStr)
	}

	if err != nil {
		// Try to parse error from JSON output first (check stdout)
		var result BdCommandResult
		if jsonErr := json.Unmarshal(stdout.Bytes(), &result); jsonErr == nil && result.Error != "" {
			return nil, fmt.Errorf("bd %s failed: %s", args[0], result.Error)
		}
		// Fall back to stderr, then stdout
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput == "" {
			errOutput = strings.TrimSpace(stdout.String())
		}
		if errOutput == "" {
			return nil, fmt.Errorf("bd %s command failed: %v", args[0], err)
		}
		return nil, fmt.Errorf("bd %s failed: %s", args[0], errOutput)
	}

	// Parse JSON response from stdout only
	result, parseErr := parseBdJSON(stdout.Bytes())
	if parseErr != nil {
		// Provide helpful error with snippet of output
		outputPreview := stdout.String()
		if len(outputPreview) > 200 {
			outputPreview = outputPreview[:200] + "..."
		}
		return nil, fmt.Errorf("failed to parse JSON from bd %s: %v (output: %s)", args[0], parseErr, outputPreview)
	}

	return result, nil
}

// detectBdVersion asks the bd on PATH for its version. The update check is
// switched off so startup makes no network call, and the timeout keeps a
// wedged bd from holding the TUI back.
func detectBdVersion() bdversion.Result {
	if _, err := exec.LookPath("bd"); err != nil {
		return bdversion.Classify("", bdversion.ErrNotFound)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bd", "--version")
	cmd.Env = append(os.Environ(), "BD_NO_UPDATE_CHECK=1")
	out, err := cmd.Output()
	return bdversion.Classify(string(out), err)
}

// parseBdJSON parses bd command JSON output, handling multiple response formats:
// - Array of issues: [{"id":"tui-123",...}]
// - Single issue: {"id":"tui-123",...}
// - Single comment: {"id":15,"issue_id":"tui-123",...}
// - Message: {"message":"Added dependency: tui-123 depends on tui-456"}
func parseBdJSON(data []byte) (*BdCommandResult, error) {
	result := &BdCommandResult{}

	// Try parsing as array of issues first (most common)
	var issues []parser.Issue
	if err := json.Unmarshal(data, &issues); err == nil {
		result.Issues = issues
		return result, nil
	}

	// Try parsing as single issue
	var issue parser.Issue
	if err := json.Unmarshal(data, &issue); err == nil {
		// Check if this looks like an issue (has ID and Title fields)
		if issue.ID != "" && issue.Title != "" {
			result.Issues = []parser.Issue{issue}
			return result, nil
		}
	}

	// Try parsing as single comment
	var comment parser.Comment
	if err := json.Unmarshal(data, &comment); err == nil {
		// Check if this looks like a comment (has ID and IssueID)
		if comment.ID > 0 && comment.IssueID != "" {
			result.Comments = []parser.Comment{comment}
			return result, nil
		}
	}

	// Try parsing as a message: {"message":"Added dependency: ..."}
	var message struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &message); err == nil && message.Message != "" {
		result.Message = message.Message
		return result, nil
	}

	return nil, fmt.Errorf("unable to parse JSON as issue array, issue, comment, or message")
}

// execBdJSONIssue is a convenience wrapper that executes a bd command and returns
// the first issue from the result, or an error if no issues were returned.
func execBdJSONIssue(args ...string) (*parser.Issue, error) {
	result, err := execBdJSON(args...)
	if err != nil {
		return nil, err
	}

	if len(result.Issues) == 0 {
		cmdName := "unknown"
		if len(args) > 0 {
			cmdName = args[0]
		}
		return nil, fmt.Errorf("bd %s returned no issues (expected an issue in response)", cmdName)
	}

	return &result.Issues[0], nil
}

// execBdJSONComment is a convenience wrapper that executes a bd command and returns
// the first comment from the result, or an error if no comments were returned.
func execBdJSONComment(args ...string) (*parser.Comment, error) {
	result, err := execBdJSON(args...)
	if err != nil {
		return nil, err
	}

	if len(result.Comments) == 0 {
		cmdName := "unknown"
		if len(args) > 0 {
			cmdName = args[0]
		}
		return nil, fmt.Errorf("bd %s returned no comments (expected a comment in response)", cmdName)
	}

	return &result.Comments[0], nil
}
