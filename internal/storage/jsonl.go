package storage

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// JSONLReader reads issues from a .beads/issues.jsonl file
type JSONLReader struct {
	path string
}

// NewJSONLReader creates a new JSONL reader for the given file path.
// Returns an error if the file does not exist.
func NewJSONLReader(path string) (*JSONLReader, error) {
	log.Printf("JSONL: Opening file at %s", path)

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSONL file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to stat JSONL file: %w", err)
	}

	log.Printf("JSONL: File exists, reader ready")
	return &JSONLReader{path: path}, nil
}

// LoadIssues reads all issues from the JSONL file.
// The context parameter is accepted for API compatibility but is not
// currently used since file reads are fast and non-blocking.
func (r *JSONLReader) LoadIssues(ctx context.Context) ([]*parser.Issue, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	issues, err := parser.ParseFile(r.path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSONL file: %w", err)
	}

	return issues, nil
}

// Close is a no-op for JSONL reader (no resources to release).
func (r *JSONLReader) Close() error {
	return nil
}
