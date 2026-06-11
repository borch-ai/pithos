package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewManifest(t *testing.T) {
	path := "/tmp/test_manifest.json"
	m := NewManifest(path)
	if m.filePath != path {
		t.Errorf("expected filePath to be %s, got %s", path, m.filePath)
	}
	if m.AssetRegistry == nil {
		t.Error("expected AssetRegistry to be initialized")
	}
	if m.Progress.Pages == nil {
		t.Error("expected Progress.Pages to be initialized")
	}
}

func TestJSONSerialization(t *testing.T) {
	m := NewManifest("")
	m.BookProperties = BookProperties{
		Theme:           "Adventure",
		Style:           "WaterColor",
		Format:          "6x9",
		TargetPageCount: 24,
	}
	m.Progress.ManuscriptGenerated = true
	m.Progress.CoverImageGenerated = true
	m.Progress.CoverImagePath = "/assets/cover.png"
	m.Progress.Pages = []PageState{
		{PageIndex: 0, Status: StatusCompleted, ImagePath: "/assets/page_0.png", Text: "Hello World"},
	}
	m.AssetRegistry["cover"] = "/assets/cover.png"
	m.KDPLayout = KDPLayout{
		SpineWidth: 0.15,
		MarginSize: 0.75,
		Bleed:      0.125,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	var m2 Manifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if m2.BookProperties.Theme != m.BookProperties.Theme {
		t.Errorf("expected Theme %s, got %s", m.BookProperties.Theme, m2.BookProperties.Theme)
	}
	if m2.Progress.ManuscriptGenerated != m.Progress.ManuscriptGenerated {
		t.Errorf("expected ManuscriptGenerated %v, got %v", m.Progress.ManuscriptGenerated, m2.Progress.ManuscriptGenerated)
	}
	if m2.AssetRegistry["cover"] != m.AssetRegistry["cover"] {
		t.Errorf("expected AssetRegistry['cover'] %s, got %s", m.AssetRegistry["cover"], m2.AssetRegistry["cover"])
	}
	if len(m2.Progress.Pages) != 1 || m2.Progress.Pages[0].Status != StatusCompleted {
		t.Errorf("expected Pages[0].Status %v, got %v", StatusCompleted, m2.Progress.Pages[0].Status)
	}
}

func TestAtomicSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := NewManifest(manifestPath)
	m.BookProperties.Theme = "Sci-Fi"

	err = m.Save()
	if err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	// Verify file exists
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		t.Fatalf("expected file to exist at %s, but it does not", manifestPath)
	}

	// Load manifest
	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if loaded.BookProperties.Theme != "Sci-Fi" {
		t.Errorf("expected Theme Sci-Fi, got %s", loaded.BookProperties.Theme)
	}

	if loaded.FilePath() != manifestPath {
		t.Errorf("expected FilePath %s, got %s", manifestPath, loaded.FilePath())
	}
}

func TestSaveErrors(t *testing.T) {
	// 1. Path not set
	m := NewManifest("")
	if err := m.Save(); err == nil {
		t.Error("expected error when saving manifest with empty filePath, got nil")
	}

	// 2. Directory write permission or path is invalid (e.g. directory doesn't exist and can't be created)
	// We pass an invalid path where a file acts as a directory component
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a dummy file
	dummyFile := filepath.Join(tmpDir, "dummy")
	if err := os.WriteFile(dummyFile, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// Try to use it as a directory component
	mInvalid := NewManifest(filepath.Join(dummyFile, "manifest.json"))
	if err := mInvalid.Save(); err == nil {
		t.Error("expected error when saving to invalid path, got nil")
	}
}

func TestLoadManifestErrors(t *testing.T) {
	// 1. Non-existent file
	_, err := LoadManifest("/nonexistent/manifest.json")
	if err == nil {
		t.Error("expected error loading nonexistent manifest, got nil")
	}

	// 2. Invalid JSON content
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if writeErr := os.WriteFile(invalidFile, []byte("{invalid-json}"), 0600); writeErr != nil {
		t.Fatalf("failed to write invalid file: %v", writeErr)
	}

	_, err = LoadManifest(invalidFile)
	if err == nil {
		t.Error("expected error loading invalid json manifest, got nil")
	}
}

func TestUpdateHelpers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := NewManifest(manifestPath)

	// Update manuscript status
	if err := m.UpdateManuscriptStatus(true); err != nil {
		t.Fatalf("UpdateManuscriptStatus failed: %v", err)
	}
	if !m.Progress.ManuscriptGenerated {
		t.Error("expected Progress.ManuscriptGenerated to be true")
	}

	// Update cover image
	if err := m.UpdateCoverImage(true, "/cover.png"); err != nil {
		t.Fatalf("UpdateCoverImage failed: %v", err)
	}
	if !m.Progress.CoverImageGenerated || m.Progress.CoverImagePath != "/cover.png" {
		t.Error("expected Progress.CoverImageGenerated to be true and path to be /cover.png")
	}

	// Register Asset
	if err := m.RegisterAsset("page_1", "/page_1.png"); err != nil {
		t.Fatalf("RegisterAsset failed: %v", err)
	}
	if m.AssetRegistry["page_1"] != "/page_1.png" {
		t.Errorf("expected AssetRegistry['page_1'] to be /page_1.png, got %s", m.AssetRegistry["page_1"])
	}
	// Test nil asset registry check
	mNilAsset := &Manifest{filePath: manifestPath}
	if err := mNilAsset.RegisterAsset("key", "value"); err != nil {
		t.Fatalf("RegisterAsset failed with nil registry: %v", err)
	}
	if mNilAsset.AssetRegistry["key"] != "value" {
		t.Errorf("expected registry value 'value', got '%s'", mNilAsset.AssetRegistry["key"])
	}
	// Update page status - page not exist
	if err := m.UpdatePageStatus(1, StatusGeneratingImages, ""); err != nil {
		t.Fatalf("UpdatePageStatus failed: %v", err)
	}
	if len(m.Progress.Pages) != 1 || m.Progress.Pages[0].PageIndex != 1 || m.Progress.Pages[0].Status != StatusGeneratingImages {
		t.Error("expected page 1 to be added with StatusGeneratingImages")
	}

	// Update page status - page exists
	if err := m.UpdatePageStatus(1, StatusCompleted, "/page_1.png"); err != nil {
		t.Fatalf("UpdatePageStatus failed: %v", err)
	}
	if len(m.Progress.Pages) != 1 || m.Progress.Pages[0].Status != StatusCompleted || m.Progress.Pages[0].ImagePath != "/page_1.png" {
		t.Error("expected page 1 to be updated to StatusCompleted and image path to /page_1.png")
	}
}

func TestConcurrentSaveAndUpdates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := NewManifest(manifestPath)
	m.SetFilePath(manifestPath)

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pageIdx := workerID*iterations + j
				_ = m.UpdatePageStatus(pageIdx, StatusCompleted, "/path.png")
				_ = m.RegisterAsset("key", "val")
				_ = m.Save()
			}
		}(i)
	}

	wg.Wait()

	// Verify loaded
	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest after concurrent writes: %v", err)
	}

	expectedLen := workers * iterations
	if len(loaded.Progress.Pages) != expectedLen {
		t.Errorf("expected %d pages, got %d", expectedLen, len(loaded.Progress.Pages))
	}
}

func TestKDPLayoutHelpers(t *testing.T) {
	const delta = 1e-9
	isNear := func(a, b float64) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff < delta
	}

	// Test Spine calculations
	wWhite := CalculateSpineWidth(100, PaperWhite)
	expectedWhite := 100 * 0.002252
	if !isNear(wWhite, expectedWhite) {
		t.Errorf("expected white spine %f, got %f", expectedWhite, wWhite)
	}

	wCream := CalculateSpineWidth(100, PaperCream)
	expectedCream := 100 * 0.0025
	if !isNear(wCream, expectedCream) {
		t.Errorf("expected cream spine %f, got %f", expectedCream, wCream)
	}

	wColor := CalculateSpineWidth(100, PaperColor)
	expectedColor := 100 * 0.002347
	if !isNear(wColor, expectedColor) {
		t.Errorf("expected color spine %f, got %f", expectedColor, wColor)
	}

	wDefault := CalculateSpineWidth(100, PaperType("unknown"))
	if !isNear(wDefault, expectedWhite) {
		t.Errorf("expected default spine (white) %f, got %f", expectedWhite, wDefault)
	}

	// Test Bleed calculations
	wb, hb := CalculateTrimWithBleed(6.0, 9.0)
	if !isNear(wb, 6.125) || !isNear(hb, 9.25) {
		t.Errorf("expected 6.125 x 9.25, got %f x %f", wb, hb)
	}
}
