package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/config"
)

// getWorkspacesRoot returns the workspaces root path, falling back to ~/.local/share/pithos/workspaces.
func getWorkspacesRoot() string {
	if config.Cfg != nil && config.Cfg.WorkspacesRoot != "" {
		return config.Cfg.WorkspacesRoot
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "pithos", "workspaces")
	}
	return filepath.Join(".", "books")
}

// expandTilde resolves paths starting with ~/ or ~ to the user's home directory.
func expandTilde(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// resolveBookPath ensures that relative output/input directories are resolved under getWorkspacesRoot().
// If the path starts explicitly with the "books" directory component, it resolves relative to the current working directory.
// It strips leading traversal components ("../", "..\\", etc.) using filepath.Separator so the
// behaviour is correct on both Unix and Windows.
func resolveBookPath(path string) string {
	if path == "" {
		return ""
	}

	// Support tilde expansion
	path = expandTilde(path)

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	sep := string(filepath.Separator)
	dotdotSep := ".." + sep
	cleaned := filepath.Clean(path)
Loop:
	for {
		switch {
		case strings.HasPrefix(cleaned, dotdotSep):
			cleaned = cleaned[len(dotdotSep):]
		case cleaned == "..":
			cleaned = "."
		case cleaned == ".":
			cleaned = ""
			break Loop
		default:
			break Loop
		}
		cleaned = filepath.Clean(cleaned)
	}

	if cleaned == "" {
		return getWorkspacesRoot()
	}

	parts := strings.Split(cleaned, sep)
	if len(parts) > 0 && parts[0] == "books" {
		return cleaned
	}

	return filepath.Join(getWorkspacesRoot(), cleaned)
}

// ResolveBookPath resolves output/input directories under getWorkspacesRoot() or books/ for CLI usage.
func ResolveBookPath(path string) string {
	return resolveBookPath(path)
}
