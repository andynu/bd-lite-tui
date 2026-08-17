package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/andynu/bd-lite-tui/internal/parser"
)

// JSONLReader reads issues from a .beads/issues.jsonl file
type JSONLReader struct {
	path string
}

// NewJSONLReader creates a new JSONL reader for the given file path.
//
// A missing file is not an error: bd-lite treats a .beads with no issues.jsonl
// as an empty store (`bd list` prints "No issues found", `bd create` writes the
// file on first use), and the reader matches that. Every other stat failure
// (permissions, a directory in the way) still fails here, where the message can
// name the path.
func NewJSONLReader(path string) (*JSONLReader, error) {
	log.Printf("JSONL: Opening file at %s", path)

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			log.Printf("JSONL: %s does not exist yet; starting from an empty store", path)
			return &JSONLReader{path: path}, nil
		}
		return nil, fmt.Errorf("failed to stat JSONL file: %w", err)
	}

	log.Printf("JSONL: File exists, reader ready")
	return &JSONLReader{path: path}, nil
}

// LoadIssues reads all issues from the JSONL file, returning no issues when the
// file does not exist. The absence is re-checked on every load rather than
// remembered from construction, so the first `bd create` (which creates the
// file) is picked up by the watcher like any other write.
//
// The context parameter is accepted for API compatibility but is not
// currently used since file reads are fast and non-blocking.
func (r *JSONLReader) LoadIssues(ctx context.Context) ([]*parser.Issue, error) {
	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	issues, err := parser.ParseFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to parse JSONL file: %w", err)
	}

	return issues, nil
}

// Close is a no-op for JSONL reader (no resources to release).
func (r *JSONLReader) Close() error {
	return nil
}
