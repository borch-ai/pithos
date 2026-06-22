package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
)

func TestListWorkspaces_NonExistent(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	res, err := ListWorkspaces(nonExistent)
	if err != nil {
		t.Fatalf("expected no error for non-existent directory, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty list for non-existent directory, got %d items", len(res))
	}
}

func createManifestA(t *testing.T, tmpRoot string) {
	bookADir := filepath.Join(tmpRoot, "book-a")
	if err := os.Mkdir(bookADir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	manifestA := manifest.NewManifest(filepath.Join(bookADir, "manifest.json"))
	manifestA.BookProperties.Theme = "Existential Dread"
	manifestA.BookProperties.Format = "hardcover"
	manifestA.BookProperties.TargetPageCount = 20
	_ = manifestA.UpdatePageStatus(0, manifest.StatusCompleted, "img0.png")
	_ = manifestA.UpdatePageStatus(1, manifest.StatusCompleted, "img1.png")
	_ = manifestA.AddMilestone("initiate_complete")
	_ = manifestA.AddMilestone("brew_complete")
	manifestA.Telemetry.TotalCostUSD = 1.25
	if saveErr := manifestA.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest A: %v", saveErr)
	}
}

func createManifestB(t *testing.T, tmpRoot string) {
	bookBDir := filepath.Join(tmpRoot, "book-b")
	if err := os.Mkdir(bookBDir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	manifestB := manifest.NewManifest(filepath.Join(bookBDir, "manifest.json"))
	manifestB.BookProperties.Theme = "Philosophical Sadness"
	manifestB.BookProperties.Format = "paperback"
	manifestB.BookProperties.TargetPageCount = 15
	_ = manifestB.AddMilestone("initiate_complete")
	manifestB.Telemetry.TotalCostUSD = 0.40
	if saveErr := manifestB.Save(); saveErr != nil {
		t.Fatalf("failed to save manifest B: %v", saveErr)
	}
}

func TestListWorkspaces_Valid(t *testing.T) {
	tmpRoot := t.TempDir()

	createManifestA(t, tmpRoot)
	createManifestB(t, tmpRoot)

	// Run ListWorkspaces
	summaries, err := ListWorkspaces(tmpRoot)
	if err != nil {
		t.Fatalf("failed to list workspaces: %v", err)
	}

	// Assertions
	if len(summaries) != 2 {
		t.Fatalf("expected exactly 2 valid book summaries, got %d", len(summaries))
	}

	// Verify sorting and fields
	// book-a
	sA := summaries[0]
	if sA.DirName != "book-a" {
		t.Errorf("expected first book to be book-a, got %s", sA.DirName)
	}
	if sA.Theme != "Existential Dread" {
		t.Errorf("expected theme 'Existential Dread', got %s", sA.Theme)
	}
	if sA.Format != "hardcover" {
		t.Errorf("expected format 'hardcover', got %s", sA.Format)
	}
	if sA.PageCount != 2 {
		t.Errorf("expected page count 2, got %d", sA.PageCount)
	}
	if sA.TargetPageCount != 20 {
		t.Errorf("expected target page count 20, got %d", sA.TargetPageCount)
	}
	if sA.TotalCost != 1.25 {
		t.Errorf("expected total cost 1.25, got %.2f", sA.TotalCost)
	}
	if len(sA.Milestones) != 2 || sA.Milestones[0] != "initiate_complete" || sA.Milestones[1] != "brew_complete" {
		t.Errorf("unexpected milestones for book-a: %v", sA.Milestones)
	}

	// book-b
	sB := summaries[1]
	if sB.DirName != "book-b" {
		t.Errorf("expected second book to be book-b, got %s", sB.DirName)
	}
	if sB.Theme != "Philosophical Sadness" {
		t.Errorf("expected theme 'Philosophical Sadness', got %s", sB.Theme)
	}
	if sB.Format != "paperback" {
		t.Errorf("expected format 'paperback', got %s", sB.Format)
	}
	if sB.PageCount != 0 {
		t.Errorf("expected page count 0, got %d", sB.PageCount)
	}
	if sB.TargetPageCount != 15 {
		t.Errorf("expected target page count 15, got %d", sB.TargetPageCount)
	}
	if sB.TotalCost != 0.40 {
		t.Errorf("expected total cost 0.40, got %.2f", sB.TotalCost)
	}
	if len(sB.Milestones) != 1 || sB.Milestones[0] != "initiate_complete" {
		t.Errorf("unexpected milestones for book-b: %v", sB.Milestones)
	}
}

func TestListWorkspaces_InvalidAndIgnored(t *testing.T) {
	tmpRoot := t.TempDir()

	// C: Directory with invalid manifest
	badDir := filepath.Join(tmpRoot, "book-bad")
	if err := os.Mkdir(badDir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "manifest.json"), []byte("invalid json{"), 0600); err != nil {
		t.Fatalf("failed to write bad manifest: %v", err)
	}

	// D: Directory with missing manifest
	emptyDir := filepath.Join(tmpRoot, "book-empty")
	if err := os.Mkdir(emptyDir, 0700); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// E: Plain file (not a directory)
	plainFile := filepath.Join(tmpRoot, "not-a-directory.txt")
	if err := os.WriteFile(plainFile, []byte("hello"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Run ListWorkspaces
	summaries, err := ListWorkspaces(tmpRoot)
	if err != nil {
		t.Fatalf("failed to list workspaces: %v", err)
	}

	// Assertions: none of C, D, or E should be loaded as valid books
	if len(summaries) != 0 {
		t.Fatalf("expected 0 book summaries, got %d", len(summaries))
	}
}
