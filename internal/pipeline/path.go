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
	parts := strings.Split(cleaned, string(filepath.Separator))
	if len(parts) > 0 && parts[0] == "books" {
		return cleaned
	}
	return filepath.Join("books", cleaned)
}
