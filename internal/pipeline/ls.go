package pipeline

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
)

// BookSummary holds summary details of a book workspace for listing.
type BookSummary struct {
	DirName         string
	Theme           string
	Format          string
	PageCount       int
	TargetPageCount int
	Milestones      []string
	TotalCost       float64
}

// ListWorkspaces scans the given workspaceRoot directory, parses manifest.json in each subdirectory,
// and returns a list of BookSummary.
func ListWorkspaces(workspaceRoot string) ([]BookSummary, error) {
	entries, err := os.ReadDir(workspaceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var summaries []BookSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(workspaceRoot, entry.Name(), "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		m, err := manifest.LoadManifest(manifestPath)
		if err != nil {
			// Skip corrupt manifests so one failure does not block listing other workspaces
			continue
		}

		summaries = append(summaries, BookSummary{
			DirName:         entry.Name(),
			Theme:           m.BookProperties.Theme,
			Format:          m.BookProperties.Format,
			PageCount:       len(m.Progress.Pages),
			TargetPageCount: m.BookProperties.TargetPageCount,
			Milestones:      m.Kiln.Milestones,
			TotalCost:       m.Telemetry.TotalCostUSD,
		})
	}

	// Sort alphabetically by DirName to ensure deterministic ordering
	sort.Slice(summaries, func(i, j int) bool {
		return strings.Compare(summaries[i].DirName, summaries[j].DirName) < 0
	})

	return summaries, nil
}
