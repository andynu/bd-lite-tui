package formatting

// ContainsCaseInsensitive checks if s contains substr (case-insensitive)
func ContainsCaseInsensitive(s, substr string) bool {
	s = ToLower(s)
	substr = ToLower(substr)
	return len(s) >= len(substr) && IndexCaseInsensitive(s, substr) >= 0
}

// ToLower converts ASCII A-Z to lowercase, leaving all other runes (including
// multi-byte UTF-8 such as emoji) untouched.
//
// Note: `for i, r := range s` yields byte offsets for i, so indexing a []rune by
// i corrupts any string containing multi-byte runes. Append in iteration order
// instead.
func ToLower(s string) string {
	result := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			r += 32
		}
		result = append(result, r)
	}
	return string(result)
}

// IndexCaseInsensitive finds the index of substr in s (case-insensitive)
func IndexCaseInsensitive(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// FormatIssueID returns the issue ID with or without its prefix.
// If showPrefix is true, returns the full ID (e.g., "tui-abc").
// If showPrefix is false, returns just the suffix after the hyphen (e.g., "abc").
func FormatIssueID(id string, showPrefix bool) string {
	if showPrefix {
		return id
	}
	// Find the last hyphen and return everything after it
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			return id[i+1:]
		}
	}
	// No hyphen found, return as-is
	return id
}
