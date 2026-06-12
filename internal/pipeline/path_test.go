package pipeline

import (
	"path/filepath"
	"testing"
)

func TestResolveBookPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "absolute path",
			input:    "/absolute/path/to/book",
			expected: "/absolute/path/to/book",
		},
		{
			name:     "relative path without books prefix",
			input:    "my-special-book",
			expected: filepath.Join("books", "my-special-book"),
		},
		{
			name:     "relative path with books prefix",
			input:    filepath.Join("books", "my-other-book"),
			expected: filepath.Join("books", "my-other-book"),
		},
		{
			name:     "nested relative path",
			input:    filepath.Join("subdir", "nested-book"),
			expected: filepath.Join("books", "subdir", "nested-book"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := resolveBookPath(tt.input)
			if actual != tt.expected {
				t.Errorf("resolveBookPath(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
