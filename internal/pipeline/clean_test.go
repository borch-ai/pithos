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
		{PageIndex: 2, Status: manifest.StatusCompleted, ImagePath: "page2.png"},
		{PageIndex: 3, Status: manifest.StatusAwaitingApproval},
	}
	m.Progress.CoverImagePath = "cover.png"
	m.AssetRegistry["some_asset"] = "asset.png"

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

	if m.Progress.Pages[1].Status != manifest.StatusPending {
		t.Errorf("Expected page 1 status to be pending, got %v", m.Progress.Pages[1].Status)
	}
	if m.Progress.Pages[2].Status != manifest.StatusCompleted {
		t.Errorf("Expected page 2 status to be completed, got %v", m.Progress.Pages[2].Status)
	}
	if m.Progress.Pages[3].Status != manifest.StatusAwaitingApproval {
		t.Errorf("Expected page 3 status to remain awaiting_approval, got %v", m.Progress.Pages[3].Status)
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
	}
}

func TestCleanWorkspace_Orphans(t *testing.T) {
	tmpRoot, bookName := setupCleanTestWorkspace(t)
	imagesDir := filepath.Join(tmpRoot, bookName, "images")
	if err := os.Mkdir(imagesDir, 0700); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	filesToCreate := []string{
		"page2.png",   // Referenced by page 2
		"cover.png",   // Referenced by cover
		"asset.png",   // Referenced by asset registry
		"orphan1.png", // Not referenced
		"orphan2.jpg", // Not referenced
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

	expectedFiles := []string{"page2.png", "cover.png", "asset.png"}
	unexpectedFiles := []string{"orphan1.png", "orphan2.jpg"}

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
}
