package pipeline

import (
	"path/filepath"
	"strings"
)

// resolveBookPath ensures that relative output/input directories are resolved under "books/".
// It strips leading traversal components ("../", "..\\", etc.) using filepath.Separator so the
// behaviour is correct on both Unix and Windows.
func resolveBookPath(path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	sep := string(filepath.Separator)
	dotdotSep := ".." + sep
	dotSep := "." + sep
	cleaned := filepath.Clean(path)
Loop:
	for {
		switch {
		case strings.HasPrefix(cleaned, dotdotSep):
			cleaned = cleaned[len(dotdotSep):]
		case cleaned == "..":
			cleaned = "."
		case strings.HasPrefix(cleaned, dotSep):
			cleaned = cleaned[len(dotSep):]
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
	parts := strings.Split(cleaned, sep)
	if len(parts) > 0 && parts[0] == "books" {
		return cleaned
	}
	return filepath.Join("books", cleaned)
}

// ResolveBookPath resolves output/input directories under "books/" for CLI usage.
func ResolveBookPath(path string) string {
	return resolveBookPath(path)
}
