package pipeline

import (
	"path/filepath"
	"strings"
)

// resolveBookPath ensures that relative output/input directories are resolved under "books/".
func resolveBookPath(path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	cleaned := filepath.Clean(path)
Loop:
	for {
		switch {
		case strings.HasPrefix(cleaned, "../"):
			cleaned = cleaned[3:]
		case cleaned == "..":
			cleaned = "."
		case strings.HasPrefix(cleaned, "./"):
			cleaned = cleaned[2:]
		case cleaned == ".":
			cleaned = ""
			break Loop
		default:
			break Loop
		}
		cleaned = filepath.Clean(cleaned)
	}
	if cleaned == "" {
		return "books"
	}
	parts := strings.Split(cleaned, string(filepath.Separator))
	if len(parts) > 0 && parts[0] == "books" {
		return cleaned
	}
	return filepath.Join("books", cleaned)
}
