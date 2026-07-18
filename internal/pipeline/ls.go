package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/registry"
)

var absFunc = filepath.Abs

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

// ListWorkspaces scans the given workspaceRoot directory and the global registry,
// parses manifest.json in each workspace, and returns a list of BookSummary.
func ListWorkspaces(workspaceRoot string) ([]BookSummary, error) {
	var paths []string
	isRegPath := make(map[string]bool)

	// 1. Load paths from global registry
	regPaths, err := registry.Load()
	if err != nil {
		return nil, err
	}
	for _, p := range regPaths {
		var resolved string
		abs, absErr := absFunc(p)
		if absErr == nil {
			resolved = filepath.Clean(abs)
		} else {
			resolved = filepath.Clean(p)
		}
		isRegPath[resolved] = true
	}
	paths = append(paths, regPaths...)

	// 2. Scan default workspaces root for subdirectories
	rootPaths, err := scanWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	paths = append(paths, rootPaths...)

	// 3. Deduplicate paths
	uniquePaths := deduplicatePaths(paths)

	var summaries []BookSummary
	var stalePaths []string

	// 4. Load manifest and compile summary for each workspace path
	for _, path := range uniquePaths {
		summary, isStale, err := loadBookSummary(path)
		if err != nil {
			return nil, err
		}
		if isStale {
			if isRegPath[path] {
				stalePaths = append(stalePaths, path)
			}
			continue
		}
		if summary != nil {
			summaries = append(summaries, *summary)
		}
	}

	// 5. Prune any stale paths from the global registry in a single batch operation
	if len(stalePaths) > 0 {
		if err := registry.RemovePaths(stalePaths); err != nil {
			return nil, err
		}
	}

	// Sort alphabetically by DirName to ensure deterministic ordering
	sort.Slice(summaries, func(i, j int) bool {
		return strings.Compare(summaries[i].DirName, summaries[j].DirName) < 0
	})

	return summaries, nil
}

func scanWorkspaceRoot(workspaceRoot string) ([]string, error) {
	if workspaceRoot == "" {
		return nil, nil
	}

	absRoot, err := absFunc(workspaceRoot)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		return nil, nil
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			paths = append(paths, filepath.Join(absRoot, entry.Name()))
		}
	}
	return paths, nil
}

func deduplicatePaths(paths []string) []string {
	uniquePathsMap := make(map[string]bool)
	var uniquePaths []string
	for _, p := range paths {
		var resolved string
		abs, err := absFunc(p)
		if err == nil {
			resolved = filepath.Clean(abs)
		} else {
			resolved = filepath.Clean(p)
		}
		if !uniquePathsMap[resolved] {
			uniquePathsMap[resolved] = true
			uniquePaths = append(uniquePaths, resolved)
		}
	}
	return uniquePaths
}

func loadBookSummary(path string) (*BookSummary, bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, true, nil
		}
		return nil, false, err
	}
	if !fi.IsDir() {
		return nil, true, nil
	}

	manifestPath := filepath.Join(path, "manifest.json")
	if _, statErr := os.Stat(manifestPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, statErr
	}

	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		if os.IsPermission(err) {
			return nil, false, err
		}
		return nil, false, nil
	}

	return &BookSummary{
		DirName:         filepath.Base(path),
		Theme:           m.BookProperties.Theme,
		Format:          m.BookProperties.Format,
		PageCount:       len(m.Progress.Pages),
		TargetPageCount: m.BookProperties.TargetPageCount,
		Milestones:      m.Kiln.Milestones,
		TotalCost:       m.Telemetry.TotalCostUSD,
	}, false, nil
}
