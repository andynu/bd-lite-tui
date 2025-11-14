package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

func TestNewJSONLReader(t *testing.T) {
	// Create a temp JSONL file
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	if reader.path != jsonlPath {
		t.Errorf("Expected path %q, got %q", jsonlPath, reader.path)
	}
}

func TestNewJSONLReader_NonexistentFile(t *testing.T) {
	_, err := NewJSONLReader("/nonexistent/path/issues.jsonl")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
}

func TestLoadIssues_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	ctx := context.Background()
	issues, err := reader.LoadIssues(ctx)
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(issues))
	}
}

func TestLoadIssues_BasicIssues(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	jsonlContent := `{"id":"test-123","title":"Test Issue","description":"Test description","status":"open","priority":1,"issue_type":"feature","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
{"id":"test-456","title":"Another Issue","description":"Another description","status":"in_progress","priority":2,"issue_type":"bug","created_at":"2025-01-02T00:00:00Z","updated_at":"2025-01-02T00:00:00Z"}
`
	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	ctx := context.Background()
	issues, err := reader.LoadIssues(ctx)
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("Expected 2 issues, got %d", len(issues))
	}

	// Check first issue
	issue := issues[0]
	if issue.ID != "test-123" {
		t.Errorf("Expected ID 'test-123', got '%s'", issue.ID)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("Expected title 'Test Issue', got '%s'", issue.Title)
	}
	if issue.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", issue.Description)
	}
	if issue.Status != parser.StatusOpen {
		t.Errorf("Expected status 'open', got '%s'", issue.Status)
	}
	if issue.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", issue.Priority)
	}
	if issue.IssueType != parser.TypeFeature {
		t.Errorf("Expected type 'feature', got '%s'", issue.IssueType)
	}

	// Check second issue
	issue2 := issues[1]
	if issue2.ID != "test-456" {
		t.Errorf("Expected ID 'test-456', got '%s'", issue2.ID)
	}
	if issue2.Status != parser.StatusInProgress {
		t.Errorf("Expected status 'in_progress', got '%s'", issue2.Status)
	}
}

func TestLoadIssues_WithDependenciesLabelsComments(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")

	jsonlContent := `{"id":"test-1","title":"Issue 1","status":"open","priority":1,"issue_type":"feature","created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z","labels":["bug","urgent"],"dependencies":[{"issue_id":"test-1","depends_on_id":"test-2","type":"blocks","created_at":"2025-01-01T00:00:00Z"}],"comments":[{"id":1,"issue_id":"test-1","author":"alice","text":"This is a comment","created_at":"2025-01-01T01:00:00Z"}]}
`
	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	ctx := context.Background()
	issues, err := reader.LoadIssues(ctx)
	if err != nil {
		t.Fatalf("LoadIssues failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]

	// Check labels
	if len(issue.Labels) != 2 {
		t.Fatalf("Expected 2 labels, got %d", len(issue.Labels))
	}
	if issue.Labels[0] != "bug" {
		t.Errorf("Expected first label 'bug', got '%s'", issue.Labels[0])
	}
	if issue.Labels[1] != "urgent" {
		t.Errorf("Expected second label 'urgent', got '%s'", issue.Labels[1])
	}

	// Check dependencies
	if len(issue.Dependencies) != 1 {
		t.Fatalf("Expected 1 dependency, got %d", len(issue.Dependencies))
	}
	dep := issue.Dependencies[0]
	if dep.DependsOnID != "test-2" {
		t.Errorf("Expected dependency on 'test-2', got '%s'", dep.DependsOnID)
	}
	if dep.Type != parser.DepBlocks {
		t.Errorf("Expected type 'blocks', got '%s'", dep.Type)
	}

	// Check comments
	if len(issue.Comments) != 1 {
		t.Fatalf("Expected 1 comment, got %d", len(issue.Comments))
	}
	comment := issue.Comments[0]
	if comment.Author != "alice" {
		t.Errorf("Expected author 'alice', got '%s'", comment.Author)
	}
	if comment.Text != "This is a comment" {
		t.Errorf("Expected text 'This is a comment', got '%s'", comment.Text)
	}
}

func TestLoadIssues_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = reader.LoadIssues(ctx)
	if err == nil {
		t.Fatal("Expected error with cancelled context")
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	reader, err := NewJSONLReader(jsonlPath)
	if err != nil {
		t.Fatalf("NewJSONLReader failed: %v", err)
	}

	// Close should succeed (no-op)
	if err := reader.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Close again should be safe
	if err := reader.Close(); err != nil {
		t.Errorf("Second Close failed: %v", err)
	}
}
