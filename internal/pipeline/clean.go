package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/borch-ai/pithos/internal/manifest"
)

// CleanWorkspace resets failed page statuses and optionally deletes orphaned images.
func CleanWorkspace(workspaceRoot, bookName string, orphans, resetFailed, all bool) error {
	if bookName == "" || bookName == "." || bookName == ".." || strings.Contains(bookName, "/") || strings.Contains(bookName, "\\") {
		return fmt.Errorf("invalid book name: cannot be empty, '.', '..', or contain path separators")
	}

	manifestPath := filepath.Join(workspaceRoot, bookName, "manifest.json")
	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest for %s: %w", bookName, err)
	}

	needsSave := cleanManifestStatuses(m, resetFailed, all)

	if orphans {
		if err := cleanOrphanedImages(m, workspaceRoot, bookName); err != nil {
			return err
		}
	}

	if needsSave {
		if err := m.Save(); err != nil {
			return fmt.Errorf("failed to save manifest after cleaning: %w", err)
		}
	}

	return nil
}

func cleanManifestStatuses(m *manifest.Manifest, resetFailed, all bool) bool {
	needsSave := false
	if all {
		for i := range m.Progress.Pages {
			if m.Progress.Pages[i].Status != manifest.StatusPending {
				m.Progress.Pages[i].Status = manifest.StatusPending
				needsSave = true
			}
		}
	} else if resetFailed {
		for i := range m.Progress.Pages {
			st := m.Progress.Pages[i].Status
			if st != manifest.StatusPending && st != manifest.StatusCompleted && st != manifest.StatusAwaitingApproval {
				m.Progress.Pages[i].Status = manifest.StatusPending
				needsSave = true
			}
		}
	}
	return needsSave
}

func cleanOrphanedImages(m *manifest.Manifest, workspaceRoot, bookName string) error {
	referenced := make(map[string]bool)

	if m.Progress.CoverImagePath != "" {
		referenced[filepath.Base(m.Progress.CoverImagePath)] = true
	}
	for _, path := range m.AssetRegistry {
		if path != "" {
			referenced[filepath.Base(path)] = true
		}
	}
	for _, page := range m.Progress.Pages {
		if page.ImagePath != "" {
			referenced[filepath.Base(page.ImagePath)] = true
		}
	}

	imagesDir := filepath.Join(workspaceRoot, bookName, "images")
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read images directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !referenced[entry.Name()] {
			if err := os.Remove(filepath.Join(imagesDir, entry.Name())); err != nil {
				return fmt.Errorf("failed to delete orphaned image %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}
