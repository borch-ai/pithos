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
			if st != manifest.StatusPending && st != manifest.StatusCompleted && st != manifest.StatusAwaitingApproval && st != manifest.StatusGeneratingImages {
				m.Progress.Pages[i].Status = manifest.StatusPending
				needsSave = true
			}
		}
	}
	return needsSave
}

// isImageFile returns true for known raster image extensions used by pithos.
func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

// referencedImageNames builds the set of image basenames referenced in the manifest.
func referencedImageNames(m *manifest.Manifest) map[string]bool {
	refs := make(map[string]bool)
	if m.Progress.CoverImagePath != "" {
		refs[filepath.Base(m.Progress.CoverImagePath)] = true
	}
	for _, path := range m.AssetRegistry {
		if path != "" {
			refs[filepath.Base(path)] = true
		}
	}
	for _, page := range m.Progress.Pages {
		if page.ImagePath != "" {
			refs[filepath.Base(page.ImagePath)] = true
		}
	}
	return refs
}

func cleanOrphanedImages(m *manifest.Manifest, workspaceRoot, bookName string) error {
	referenced := referencedImageNames(m)

	imagesDir := filepath.Join(workspaceRoot, bookName, "images")
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read images directory: %w", err)
	}

	for _, entry := range entries {
		if err := maybeDeleteOrphan(imagesDir, entry, referenced); err != nil {
			return err
		}
	}
	return nil
}

// maybeDeleteOrphan removes a single directory entry if it is an unreferenced image file.
func maybeDeleteOrphan(imagesDir string, entry os.DirEntry, referenced map[string]bool) error {
	if entry.IsDir() {
		return nil
	}
	name := entry.Name()
	// Skip dotfiles (e.g. .gitkeep) and non-image files.
	if strings.HasPrefix(name, ".") || !isImageFile(name) {
		return nil
	}
	if referenced[name] {
		return nil
	}
	if err := os.Remove(filepath.Join(imagesDir, name)); err != nil {
		return fmt.Errorf("failed to delete orphaned image %s: %w", name, err)
	}
	return nil
}
