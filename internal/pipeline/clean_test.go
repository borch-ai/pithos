package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
)

func setupCleanTestWorkspace(t *testing.T) (string, string) {
	tmpRoot := t.TempDir()
	bookName := "test-book"
	bookDir := filepath.Join(tmpRoot, bookName)
	if err := os.Mkdir(bookDir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	m := manifest.NewManifest(filepath.Join(bookDir, "manifest.json"))
	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 0, Status: manifest.StatusPending},
		{PageIndex: 1, Status: manifest.StatusGeneratingImages},
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "images/page2.png"},
		{PageIndex: 3, Status: manifest.StatusAwaitingApproval},
		{PageIndex: 4, Status: manifest.PageStatus("failed")},
		{PageIndex: 5, Status: manifest.StatusPending, ImagePath: "images/stale_pending.png"},
		{PageIndex: 6, Status: manifest.StatusCompleted, ImagePath: ""},
	}
	m.Progress.CoverImagePath = "images/cover.png"
	m.AssetRegistry["some_asset"] = "images/asset.png"

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return tmpRoot, bookName
}

func TestCleanWorkspace_ResetFailed(t *testing.T) {
	tmpRoot, bookName := setupCleanTestWorkspace(t)

	err := CleanWorkspace(tmpRoot, bookName, false, true, false)
	if err != nil {
		t.Fatalf("CleanWorkspace failed: %v", err)
	}

	m, err := manifest.LoadManifest(filepath.Join(tmpRoot, bookName, "manifest.json"))
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	if m.Progress.Pages[1].Status != manifest.StatusGeneratingImages {
		t.Errorf("Expected page 1 status to remain generating_images, got %v", m.Progress.Pages[1].Status)
	}
	if m.Progress.Pages[2].Status != manifest.StatusCompleted {
		t.Errorf("Expected page 2 status to be completed, got %v", m.Progress.Pages[2].Status)
	}
	if m.Progress.Pages[2].ImagePath != "images/page2.png" {
		t.Errorf("Expected page 2 ImagePath to be preserved, got %q", m.Progress.Pages[2].ImagePath)
	}
	if m.Progress.Pages[3].Status != manifest.StatusAwaitingApproval {
		t.Errorf("Expected page 3 status to remain awaiting_approval, got %v", m.Progress.Pages[3].Status)
	}
	if m.Progress.Pages[4].Status != manifest.StatusPending {
		t.Errorf("Expected page 4 status to be pending, got %v", m.Progress.Pages[4].Status)
	}
	if m.Progress.Pages[4].ImagePath != "" {
		t.Errorf("Expected page 4 ImagePath to be cleared on reset, got %q", m.Progress.Pages[4].ImagePath)
	}
	if m.Progress.Pages[5].Status != manifest.StatusPending {
		t.Errorf("Expected page 5 status to remain pending under reset-failed, got %v", m.Progress.Pages[5].Status)
	}
	if m.Progress.Pages[5].ImagePath != "images/stale_pending.png" {
		t.Errorf("Expected page 5 ImagePath to remain intact under reset-failed, got %q", m.Progress.Pages[5].ImagePath)
	}
	if m.Progress.Pages[6].Status != manifest.StatusPending {
		t.Errorf("Expected page 6 status to be reset to pending under reset-failed, got %v", m.Progress.Pages[6].Status)
	}
	if m.Progress.Pages[6].ImagePath != "" {
		t.Errorf("Expected page 6 ImagePath to remain empty, got %q", m.Progress.Pages[6].ImagePath)
	}
}

func TestCleanWorkspace_All(t *testing.T) {
	tmpRoot, bookName := setupCleanTestWorkspace(t)

	err := CleanWorkspace(tmpRoot, bookName, false, false, true)
	if err != nil {
		t.Fatalf("CleanWorkspace failed: %v", err)
	}

	m, err := manifest.LoadManifest(filepath.Join(tmpRoot, bookName, "manifest.json"))
	if err != nil {
		t.Fatalf("Failed to load manifest: %v", err)
	}

	for _, page := range m.Progress.Pages {
		if page.Status != manifest.StatusPending {
			t.Errorf("Expected page %d status to be pending, got %v", page.PageIndex, page.Status)
		}
		if page.ImagePath != "" {
			t.Errorf("Expected page %d ImagePath to be cleared on --all reset, got %q", page.PageIndex, page.ImagePath)
		}
	}
}

func TestCleanWorkspace_Orphans(t *testing.T) {
	tmpRoot, bookName := setupCleanTestWorkspace(t)
	imagesDir := filepath.Join(tmpRoot, bookName, "images")
	if err := os.Mkdir(imagesDir, 0700); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	filesToCreate := []string{
		"page2.png",    // Referenced by page 2
		"cover.png",    // Referenced by cover
		"asset.png",    // Referenced by asset registry
		"orphan1.png",  // Not referenced — should be deleted
		"orphan2.jpg",  // Not referenced — should be deleted
		".gitkeep",     // Dotfile — must be preserved
		"notes.txt",    // Non-image file — must be preserved
		"orphan3.webp", // Unreferenced webp — should be deleted
	}

	for _, f := range filesToCreate {
		if err := os.WriteFile(filepath.Join(imagesDir, f), []byte("dummy data"), 0600); err != nil {
			t.Fatalf("failed to create dummy image file %s: %v", f, err)
		}
	}

	err := CleanWorkspace(tmpRoot, bookName, true, false, false)
	if err != nil {
		t.Fatalf("CleanWorkspace failed: %v", err)
	}

	// Check which files remain
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		t.Fatalf("failed to read images dir: %v", err)
	}

	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		foundFiles[entry.Name()] = true
	}

	expectedFiles := []string{"page2.png", "cover.png", "asset.png", ".gitkeep", "notes.txt"}
	unexpectedFiles := []string{"orphan1.png", "orphan2.jpg", "orphan3.webp"}

	for _, f := range expectedFiles {
		if !foundFiles[f] {
			t.Errorf("Expected file %s to be kept, but it was deleted", f)
		}
	}

	for _, f := range unexpectedFiles {
		if foundFiles[f] {
			t.Errorf("Expected file %s to be deleted, but it was kept", f)
		}
	}
}

func TestCleanWorkspace_InvalidBookName(t *testing.T) {
	err := CleanWorkspace("/tmp", "invalid/book/name", false, false, false)
	if err == nil {
		t.Errorf("Expected error for bookName containing '/', got nil")
	}

	err = CleanWorkspace("/tmp", "invalid\\book\\name", false, false, false)
	if err == nil {
		t.Errorf("Expected error for bookName containing '\\', got nil")
	}

	err = CleanWorkspace("/tmp", "", false, false, false)
	if err == nil {
		t.Errorf("Expected error for empty bookName, got nil")
	}

	err = CleanWorkspace("/tmp", ".", false, false, false)
	if err == nil {
		t.Errorf("Expected error for bookName '.', got nil")
	}

	err = CleanWorkspace("/tmp", "..", false, false, false)
	if err == nil {
		t.Errorf("Expected error for bookName '..', got nil")
	}

	err = CleanWorkspace("/tmp", "C:", false, false, false)
	if err == nil {
		t.Errorf("Expected error for bookName 'C:', got nil")
	}
}

func TestReferencedImageNames(t *testing.T) {
	m := &manifest.Manifest{
		Progress: manifest.Progress{
			CoverImagePath: "images/cover.png",
			Pages: []manifest.PageState{
				{ImagePath: "images/page1.png"},    // valid: in images/
				{ImagePath: "page_bare.png"},       // valid: bare basename
				{ImagePath: "/abs/path/other.png"}, // excluded: absolute path
				{ImagePath: "other/dir/extra.png"}, // excluded: wrong directory
				{ImagePath: ""},                    // excluded: empty
			},
		},
		AssetRegistry: map[string]string{
			"a": "images/asset.png",  // valid
			"b": "/absolute/bad.png", // excluded
		},
	}

	refs := referencedImageNames(m)

	if !refs["cover.png"] {
		t.Error("Expected cover.png to be in refs")
	}
	if !refs["page1.png"] {
		t.Error("Expected page1.png to be in refs")
	}
	if !refs["page_bare.png"] {
		t.Error("Expected page_bare.png to be in refs")
	}
	if !refs["asset.png"] {
		t.Error("Expected asset.png to be in refs")
	}
	if refs["other.png"] {
		t.Error("Expected other.png (absolute path) to be excluded from refs")
	}
	if refs["extra.png"] {
		t.Error("Expected extra.png (wrong directory) to be excluded from refs")
	}
	if refs["bad.png"] {
		t.Error("Expected bad.png (absolute asset path) to be excluded from refs")
	}
}
