package pipeline

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
	"github.com/borch-ai/pithos/internal/registry"
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
	if err := manifestA.UpdatePageStatus(0, manifest.StatusCompleted, "img0.png", ""); err != nil {
		t.Fatalf("failed to update page status: %v", err)
	}
	if err := manifestA.UpdatePageStatus(1, manifest.StatusCompleted, "img1.png", ""); err != nil {
		t.Fatalf("failed to update page status: %v", err)
	}
	if err := manifestA.AddMilestone("initiate_complete"); err != nil {
		t.Fatalf("failed to add milestone: %v", err)
	}
	if err := manifestA.AddMilestone("brew_complete"); err != nil {
		t.Fatalf("failed to add milestone: %v", err)
	}
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
	if err := manifestB.AddMilestone("initiate_complete"); err != nil {
		t.Fatalf("failed to add milestone: %v", err)
	}
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

func TestListWorkspaces_PermissionDeniedDir(t *testing.T) {
	tmpRoot := t.TempDir()
	permDeniedDir := filepath.Join(tmpRoot, "perm-denied")
	if err := os.Mkdir(permDeniedDir, 0000); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(permDeniedDir, 0700) }() // restore permission for cleanup

	_, err := ListWorkspaces(permDeniedDir)
	if err == nil {
		// If the OS/filesystem doesn't enforce permission restrictions on read (e.g. running as root),
		// we skip the failure to avoid breaking CI.
		_, readErr := os.ReadDir(permDeniedDir)
		if readErr == nil {
			t.Skip("skipping test: filesystem permitted read access to 0000 directory")
		}
		t.Error("expected error when listing directory with permission denied, got nil")
	}
}

func TestListWorkspaces_PermissionDeniedWorkspace(t *testing.T) {
	tmpRoot := t.TempDir()

	createManifestA(t, tmpRoot)

	secretDir := filepath.Join(tmpRoot, "book-secret")
	if err := os.Mkdir(secretDir, 0000); err != nil {
		t.Fatalf("failed to create secretDir: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(secretDir, 0700) }()

	_, err := ListWorkspaces(tmpRoot)
	if err == nil {
		manifestPath := filepath.Join(secretDir, "manifest.json")
		_, statErr := os.Stat(manifestPath)
		if statErr != nil && errors.Is(statErr, os.ErrNotExist) {
			t.Skip("skipping test: filesystem returned ENOENT instead of EACCES for non-searchable directory contents")
		}
		t.Error("expected error when workspace folder read fails with permission denied, got nil")
	}
}

func TestListWorkspaces_PermissionDeniedWorkspaceStat(t *testing.T) {
	// Set registry file override
	registryTemp := t.TempDir()
	registry.SetRegistryPathOverride(filepath.Join(registryTemp, "registry.json"))
	defer registry.SetRegistryPathOverride("")

	tmpRoot := t.TempDir()

	// Create secret parent and nested workspace directory
	secretParent := filepath.Join(tmpRoot, "secret_parent")
	if err := os.Mkdir(secretParent, 0750); err != nil {
		t.Fatalf("failed to create secret parent dir: %v", err)
	}
	wsDir := filepath.Join(secretParent, "ws")
	if err := os.Mkdir(wsDir, 0750); err != nil {
		t.Fatalf("failed to create nested ws dir: %v", err)
	}
	// Add manifest.json so it's a valid workspace
	if err := os.WriteFile(filepath.Join(wsDir, "manifest.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Register it
	if err := registry.Add(wsDir); err != nil {
		t.Fatalf("failed to add to registry: %v", err)
	}

	// Block search permission on parent
	if err := os.Chmod(secretParent, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	// #nosec G302
	defer func() { _ = os.Chmod(secretParent, 0750) }()

	_, err := ListWorkspaces(tmpRoot)
	if err == nil {
		t.Error("expected error when workspace folder stat fails with permission denied, got nil")
	}
}

//nolint:gocognit,funlen
func TestListWorkspaces_WithRegistry(t *testing.T) {
	// Set registry file override
	registryTemp := t.TempDir()
	registry.SetRegistryPathOverride(filepath.Join(registryTemp, "registry.json"))
	defer registry.SetRegistryPathOverride("")

	// Create root directories
	workspaceRoot := t.TempDir()
	customRoot := t.TempDir()

	// 1. Create a workspace in default root (book-local)
	bookLocalDir := filepath.Join(workspaceRoot, "book-local")
	if err := os.Mkdir(bookLocalDir, 0700); err != nil {
		t.Fatalf("failed to create bookLocalDir: %v", err)
	}
	mLocal := manifest.NewManifest(filepath.Join(bookLocalDir, "manifest.json"))
	mLocal.BookProperties.Theme = "Local Theme"
	if err := mLocal.Save(); err != nil {
		t.Fatalf("failed to save local manifest: %v", err)
	}

	// 2. Create a workspace in custom root (book-custom)
	bookCustomDir := filepath.Join(customRoot, "book-custom")
	if err := os.Mkdir(bookCustomDir, 0700); err != nil {
		t.Fatalf("failed to create bookCustomDir: %v", err)
	}
	mCustom := manifest.NewManifest(filepath.Join(bookCustomDir, "manifest.json"))
	mCustom.BookProperties.Theme = "Custom Theme"
	if err := mCustom.Save(); err != nil {
		t.Fatalf("failed to save custom manifest: %v", err)
	}

	// Register the custom book path
	if err := registry.Add(bookCustomDir); err != nil {
		t.Fatalf("failed to register custom workspace: %v", err)
	}

	// 3. Run ListWorkspaces
	summaries, err := ListWorkspaces(workspaceRoot)
	if err != nil {
		t.Fatalf("failed to list workspaces: %v", err)
	}

	// Verify both local and custom are found
	if len(summaries) != 2 {
		t.Fatalf("expected exactly 2 book summaries, got %d (list: %+v)", len(summaries), summaries)
	}

	// Because of alphabetical sorting, book-custom comes first
	if summaries[0].DirName != "book-custom" || summaries[0].Theme != "Custom Theme" {
		t.Errorf("unexpected summary at index 0: %+v", summaries[0])
	}
	if summaries[1].DirName != "book-local" || summaries[1].Theme != "Local Theme" {
		t.Errorf("unexpected summary at index 1: %+v", summaries[1])
	}

	// 4. Delete custom book workspace and verify auto-prune
	if removeErr := os.RemoveAll(bookCustomDir); removeErr != nil {
		t.Fatalf("failed to delete bookCustomDir: %v", removeErr)
	}

	summaries, err = ListWorkspaces(workspaceRoot)
	if err != nil {
		t.Fatalf("failed to list workspaces after delete: %v", err)
	}

	// Only local remains
	if len(summaries) != 1 {
		t.Fatalf("expected 1 book summary after deleting custom workspace, got %d", len(summaries))
	}
	if summaries[0].DirName != "book-local" {
		t.Errorf("expected book-local to remain, got %s", summaries[0].DirName)
	}

	// Registry should have pruned the stale path
	workspaces, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	for _, ws := range workspaces {
		if ws == bookCustomDir {
			t.Errorf("expected book-custom path to be pruned from registry, but it was found")
		}
	}

	// 5. Test registering a regular file (should be treated as stale/pruned)
	regFilePath := filepath.Join(workspaceRoot, "regular-file.txt")
	if writeErr := os.WriteFile(regFilePath, []byte("not-a-directory"), 0600); writeErr != nil {
		t.Fatalf("failed to write regular file: %v", writeErr)
	}
	if addErr := registry.Add(regFilePath); addErr != nil {
		t.Fatalf("failed to add file path to registry: %v", addErr)
	}

	// ListWorkspaces should prune the file path
	summaries, err = ListWorkspaces(workspaceRoot)
	if err != nil {
		t.Fatalf("failed to list workspaces with file in registry: %v", err)
	}
	if len(summaries) != 1 || summaries[0].DirName != "book-local" {
		t.Errorf("unexpected summaries after file prune: %+v", summaries)
	}

	workspaces, err = registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	for _, ws := range workspaces {
		if ws == regFilePath {
			t.Errorf("expected regular file path to be pruned from registry, but it was found")
		}
	}
}
