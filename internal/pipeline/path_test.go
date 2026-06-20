package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/config"
)

func TestResolveBookPath(t *testing.T) {
	origCfg := config.Cfg
	config.Cfg = &config.Config{
		WorkspacesRoot: "/mock/workspaces",
	}
	defer func() {
		config.Cfg = origCfg
	}()

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
			expected: filepath.Join("/mock/workspaces", "my-special-book"),
		},
		{
			name:     "relative path with books prefix",
			input:    filepath.Join("books", "my-other-book"),
			expected: filepath.Join("books", "my-other-book"),
		},
		{
			name:     "nested relative path",
			input:    filepath.Join("subdir", "nested-book"),
			expected: filepath.Join("/mock/workspaces", "subdir", "nested-book"),
		},
		{
			name:     "relative traversal escape dot dot",
			input:    "..",
			expected: "/mock/workspaces",
		},
		{
			name:     "relative traversal escape dot dot slash x",
			input:    "../x",
			expected: filepath.Join("/mock/workspaces", "x"),
		},
		{
			name:     "relative traversal escape nested dot dot",
			input:    "../../xyz",
			expected: filepath.Join("/mock/workspaces", "xyz"),
		},
		{
			name:     "relative traversal nested books escape",
			input:    "books/../../xyz",
			expected: filepath.Join("/mock/workspaces", "xyz"),
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
	origCfg := config.Cfg
	config.Cfg = &config.Config{
		WorkspacesRoot: "/mock/workspaces",
	}
	defer func() {
		config.Cfg = origCfg
	}()

	expected := filepath.Join("/mock/workspaces", "public-book")
	actual := ResolveBookPath("public-book")
	if actual != expected {
		t.Errorf("ResolveBookPath(\"public-book\") = %q; expected %q", actual, expected)
	}
}

func TestResolveBookPathTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping tilde test because home dir could not be resolved")
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde exact",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde prefix",
			input:    "~/some/sub/dir",
			expected: filepath.Join(home, "some", "sub", "dir"),
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

func TestGetWorkspacesRoot(t *testing.T) {
	origCfg := config.Cfg
	defer func() {
		config.Cfg = origCfg
	}()

	// 1. Test when Cfg is not nil and WorkspacesRoot is specified
	config.Cfg = &config.Config{
		WorkspacesRoot: "/custom/root",
	}
	if got := getWorkspacesRoot(); got != "/custom/root" {
		t.Errorf("expected /custom/root, got %s", got)
	}

	// 2. Test when Cfg is not nil but WorkspacesRoot is empty
	config.Cfg = &config.Config{
		WorkspacesRoot: "",
	}
	home, err := os.UserHomeDir()
	if err == nil {
		expected := filepath.Join(home, ".local", "share", "pithos", "workspaces")
		if got := getWorkspacesRoot(); got != expected {
			t.Errorf("expected default home workspaces root %s, got %s", expected, got)
		}
	} else {
		expected := filepath.Join(".", "books")
		if got := getWorkspacesRoot(); got != expected {
			t.Errorf("expected fallback books root %s, got %s", expected, got)
		}
	}

	// 3. Test when Cfg is nil
	config.Cfg = nil
	if err == nil {
		expected := filepath.Join(home, ".local", "share", "pithos", "workspaces")
		if got := getWorkspacesRoot(); got != expected {
			t.Errorf("expected default home workspaces root %s when Cfg is nil, got %s", expected, got)
		}
	} else {
		expected := filepath.Join(".", "books")
		if got := getWorkspacesRoot(); got != expected {
			t.Errorf("expected fallback books root %s when Cfg is nil, got %s", expected, got)
		}
	}
}

func TestExpandTildeInternal(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping test because home directory is not resolvable")
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"tilde only", "~", home},
		{"tilde prefix slash", "~/abc", filepath.Join(home, "abc")},
		{"absolute path", "/abs/path", "/abs/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := expandTilde(tt.input)
			if actual != tt.expected {
				t.Errorf("expandTilde(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
