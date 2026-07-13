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
	if bookName == "" || bookName == "." || bookName == ".." || strings.ContainsAny(bookName, "/\\") || filepath.VolumeName(bookName) != "" {
		return fmt.Errorf("invalid book name: cannot be empty, '.', '..', contain path separators, or contain a volume name")
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
			p := &m.Progress.Pages[i]
			if p.Status != manifest.StatusPending || p.ImagePath != "" {
				p.Status = manifest.StatusPending
				p.ImagePath = ""
				needsSave = true
			}
		}
	} else if resetFailed {
		for i := range m.Progress.Pages {
			p := &m.Progress.Pages[i]
			isFailedStatus := p.Status != manifest.StatusPending &&
				p.Status != manifest.StatusCompleted &&
				p.Status != manifest.StatusAwaitingApproval &&
				p.Status != manifest.StatusGeneratingImages
			isCorruptCompleted := p.Status == manifest.StatusCompleted && p.ImagePath == ""
			if isFailedStatus || isCorruptCompleted {
				p.Status = manifest.StatusPending
				p.ImagePath = ""
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

// imageBasenameInDir adds path's basename to refs when path is relative
// and its directory component is "images" or "." (representing a bare basename).
// Absolute paths or paths in other directories are excluded to avoid incorrectly
// shielding unrelated files from orphan cleanup.
func imageBasenameInDir(refs map[string]bool, path string) {
	if path == "" || filepath.IsAbs(path) {
		return
	}
	dir := filepath.Dir(path)
	if dir == "images" || dir == "." {
		refs[filepath.Base(path)] = true
	}
}

// referencedImageNames builds the set of image basenames in the images/ directory
// that are referenced by the manifest. Only relative paths under images/ are
// considered; absolute paths or paths in other directories are excluded.
func referencedImageNames(m *manifest.Manifest) map[string]bool {
	refs := make(map[string]bool)
	imageBasenameInDir(refs, m.Progress.CoverImagePath)
	for _, path := range m.AssetRegistry {
		imageBasenameInDir(refs, path)
	}
	for _, page := range m.Progress.Pages {
		imageBasenameInDir(refs, page.ImagePath)
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
