package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// Parser reads and parses JSONL files containing bd-lite issues
type Parser struct {
	path string
}

// New creates a new parser for the given JSONL file path
func New(path string) *Parser {
	return &Parser{path: path}
}

// ParseAll reads all issues from the JSONL file
func (p *Parser) ParseAll() ([]*Issue, error) {
	file, err := os.Open(p.path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// The JSONL store is append-only with last-write-wins semantics: an issue ID
	// may appear on multiple lines, and the latest line is the current state.
	// Dedupe by ID, keeping each issue at its first-seen position but replacing
	// its data with the most recent record. Without this, re-written issues would
	// appear as duplicate rows (and be double-counted in the categorized lists).
	var issues []*Issue
	indexByID := make(map[string]int)
	scanner := bufio.NewScanner(file)
	// Allow long lines (issues with large descriptions/many comments) beyond the
	// default 64KiB token limit.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		var issue Issue
		if err := json.Unmarshal(line, &issue); err != nil {
			return nil, fmt.Errorf("invalid JSON at line %d: %w", lineNum, err)
		}

		issueCopy := issue
		if issue.ID != "" {
			if idx, seen := indexByID[issue.ID]; seen {
				issues[idx] = &issueCopy
				continue
			}
			indexByID[issue.ID] = len(issues)
		}
		issues = append(issues, &issueCopy)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return issues, nil
}

// ParseFile is a convenience function to parse a JSONL file
func ParseFile(path string) ([]*Issue, error) {
	p := New(path)
	return p.ParseAll()
}
