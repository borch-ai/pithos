package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
)

func setupStatusTestWorkspace(t *testing.T) (string, string) {
	tmpRoot := t.TempDir()
	bookName := "status-test-book"
	bookDir := filepath.Join(tmpRoot, bookName)
	if err := os.Mkdir(bookDir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	m := manifest.NewManifest(filepath.Join(bookDir, "manifest.json"))
	m.BookProperties.Theme = "Test Theme"
	m.BookProperties.Format = "hardcover"
	m.BookProperties.TargetPageCount = 10
	m.Telemetry.TotalCostUSD = 12.34

	m.Progress.Pages = []manifest.PageState{
		{PageIndex: 0, Status: manifest.StatusPending},
		{PageIndex: 1, Status: manifest.StatusGeneratingImages},
		{PageIndex: 2, Status: manifest.StatusCompleted},
		{PageIndex: 3, Status: manifest.StatusAwaitingApproval},
		{PageIndex: 4, Status: "failed"}, // unknown/stuck
	}

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return tmpRoot, bookName
}

func TestGetWorkspaceStatus(t *testing.T) {
	tmpRoot, bookName := setupStatusTestWorkspace(t)

	ws, err := GetWorkspaceStatus(tmpRoot, bookName)
	if err != nil {
		t.Fatalf("GetWorkspaceStatus failed: %v", err)
	}

	if ws.BookName != bookName {
		t.Errorf("Expected BookName %q, got %q", bookName, ws.BookName)
	}
	if ws.Theme != "Test Theme" {
		t.Errorf("Expected Theme 'Test Theme', got %q", ws.Theme)
	}
	if ws.Format != "hardcover" {
		t.Errorf("Expected Format 'hardcover', got %q", ws.Format)
	}
	if ws.TargetPageCount != 10 {
		t.Errorf("Expected TargetPageCount 10, got %d", ws.TargetPageCount)
	}
	if ws.TotalPages != 5 {
		t.Errorf("Expected TotalPages 5, got %d", ws.TotalPages)
	}
	if ws.TotalCostUSD != 12.34 {
		t.Errorf("Expected TotalCostUSD 12.34, got %v", ws.TotalCostUSD)
	}

	if ws.Pending != 2 { // 1 pending + 1 unknown ("failed")
		t.Errorf("Expected Pending 2, got %d", ws.Pending)
	}
	if ws.Generating != 1 {
		t.Errorf("Expected Generating 1, got %d", ws.Generating)
	}
	if ws.Completed != 1 {
		t.Errorf("Expected Completed 1, got %d", ws.Completed)
	}
	if ws.Awaiting != 1 {
		t.Errorf("Expected Awaiting 1, got %d", ws.Awaiting)
	}
}

func TestGetWorkspaceStatus_ManifestLoadError(t *testing.T) {
	tmpRoot := t.TempDir()
	bookName := "missing-book"

	_, err := GetWorkspaceStatus(tmpRoot, bookName)
	if err == nil {
		t.Errorf("Expected error when manifest does not exist, got nil")
	}
}
