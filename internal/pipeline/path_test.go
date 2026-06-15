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
		{
			name:     "relative traversal escape dot dot",
			input:    "..",
			expected: "books",
		},
		{
			name:     "relative traversal escape dot dot slash x",
			input:    "../x",
			expected: filepath.Join("books", "x"),
		},
		{
			name:     "relative traversal escape nested dot dot",
			input:    "../../xyz",
			expected: filepath.Join("books", "xyz"),
		},
		{
			name:     "relative traversal nested books escape",
			input:    "books/../../xyz",
			expected: filepath.Join("books", "xyz"),
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

func TestResolveBookPathPublic(t *testing.T) {
	expected := filepath.Join("books", "public-book")
	actual := ResolveBookPath("public-book")
	if actual != expected {
		t.Errorf("ResolveBookPath(\"public-book\") = %q; expected %q", actual, expected)
	}
}
