package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
)

// WorkspaceStatus provides aggregated diagnostics for a book workspace.
type WorkspaceStatus struct {
	BookName        string
	Theme           string
	Format          string
	TargetPageCount int
	TotalPages      int
	Completed       int
	Pending         int
	Generating      int
	Awaiting        int
	Unknown         int
	TotalCostUSD    float64
}

// GetWorkspaceStatus loads the manifest and compiles structural diagnostics.
func GetWorkspaceStatus(workspaceRoot, bookName string) (*WorkspaceStatus, error) {
	if bookName == "" || bookName == "." || bookName == ".." || strings.Contains(bookName, "/") || strings.Contains(bookName, "\\") {
		return nil, fmt.Errorf("invalid book name: cannot be empty, '.', '..', or contain path separators")
	}

	manifestPath := filepath.Join(workspaceRoot, bookName, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest for %s: %w", bookName, err)
	}

	ws := &WorkspaceStatus{
		BookName:        bookName,
		Theme:           m.BookProperties.Theme,
		Format:          m.BookProperties.Format,
		TargetPageCount: m.BookProperties.TargetPageCount,
		TotalPages:      len(m.Progress.Pages),
		TotalCostUSD:    m.Telemetry.TotalCostUSD,
	}

	for _, page := range m.Progress.Pages {
		switch page.Status {
		case manifest.StatusCompleted:
			// StatusCompleted is the authoritative terminal state set by the pipeline.
			// Pages undergoing illustration generation are held in StatusGeneratingImages,
			// not StatusCompleted, so a page with StatusCompleted and no ImagePath is a
			// legitimate text-only page — not an illustration still pending.
			ws.Completed++
		case manifest.StatusPending:
			ws.Pending++
		case manifest.StatusGeneratingImages:
			ws.Generating++
		case manifest.StatusAwaitingApproval:
			ws.Awaiting++
		default:
			// If there are other stuck or undefined statuses
			ws.Unknown++
		}
	}

	return ws, nil
}
