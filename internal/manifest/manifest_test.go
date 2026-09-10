package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/borch-ai/powerword/pkg/telemetry"
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
		Theme:            "Adventure",
		Style:            "WaterColor",
		CharacterProfile: "A character profile",
		Format:           "6x9",
		TargetPageCount:  24,
	}
	m.Progress.ManuscriptGenerated = true
	m.Progress.CoverImageGenerated = true
	m.Progress.CoverImagePath = "/assets/cover.png"
	m.Progress.Pages = []PageState{
		{PageIndex: 0, Status: StatusCompleted, ImagePath: "/assets/page_0.png", Text: "Hello World", Layout: "facing-pages"},
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
	if m2.BookProperties.CharacterProfile != m.BookProperties.CharacterProfile {
		t.Errorf("expected CharacterProfile %s, got %s", m.BookProperties.CharacterProfile, m2.BookProperties.CharacterProfile)
	}
	if m2.Progress.ManuscriptGenerated != m.Progress.ManuscriptGenerated {
		t.Errorf("expected ManuscriptGenerated %v, got %v", m.Progress.ManuscriptGenerated, m2.Progress.ManuscriptGenerated)
	}
	if m2.AssetRegistry["cover"] != m.AssetRegistry["cover"] {
		t.Errorf("expected AssetRegistry['cover'] %s, got %s", m.AssetRegistry["cover"], m2.AssetRegistry["cover"])
	}
	if len(m2.Progress.Pages) != 1 || m2.Progress.Pages[0].Status != StatusCompleted || m2.Progress.Pages[0].Layout != "facing-pages" {
		t.Errorf("expected Pages[0].Status %v and Layout facing-pages, got status %v layout %q", StatusCompleted, m2.Progress.Pages[0].Status, m2.Progress.Pages[0].Layout)
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
	if err := m.UpdatePageStatus(1, StatusGeneratingImages, "", ""); err != nil {
		t.Fatalf("UpdatePageStatus failed: %v", err)
	}
	if len(m.Progress.Pages) != 1 || m.Progress.Pages[0].PageIndex != 1 || m.Progress.Pages[0].Status != StatusGeneratingImages {
		t.Error("expected page 1 to be added with StatusGeneratingImages")
	}

	// Update page status - page exists
	if err := m.UpdatePageStatus(1, StatusCompleted, "/page_1.png", "legacy-model"); err != nil {
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
	const workers = 20
	const iterations = 50
	errChan := make(chan error, workers*iterations*3)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pageIdx := workerID*iterations + j
				if opErr := m.UpdatePageStatus(pageIdx, StatusCompleted, "/path.png", ""); opErr != nil {
					errChan <- opErr
				}
				if opErr := m.RegisterAsset("key", "val"); opErr != nil {
					errChan <- opErr
				}
				if opErr := m.Save(); opErr != nil {
					errChan <- opErr
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for concurrentErr := range errChan {
		t.Errorf("concurrent operation failed: %v", concurrentErr)
	}

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

func TestUpdateTotalCost(t *testing.T) {
	m := NewManifest("")

	// Populate some usages
	m.Telemetry.ModelUsages["model-1"] = &telemetry.ModelUsage{InputTokens: 1000, OutputTokens: 2000, CachedTokens: 0}
	m.Telemetry.ImageGenerations = 2

	pricing := map[string]telemetry.ModelPricing{
		"model-1":  {Input: 1.0, Output: 2.0}, // $1 per 1M input, $2 per 1M output
		"imagegen": {Input: 40000.0},          // $40000 per 1M "input" -> $0.04 per image
	}

	m.UpdateTotalCost(pricing)

	// Cost of LLM: (1000*1.0 + 2000*2.0) / 1,000,000 = (1000 + 4000) / 1,000,000 = 0.005
	// Cost of ImageGen: 2 * 0.04 = 0.08
	// Total: 0.085

	expectedCost := 0.085
	if m.Telemetry.TotalCostUSD != expectedCost {
		t.Errorf("expected TotalCostUSD %f, got %f", expectedCost, m.Telemetry.TotalCostUSD)
	}
}

func TestKilnSyncSerialization(t *testing.T) {
	m := NewManifest("")
	m.Kiln = KilnSync{
		Version:         1,
		Milestones:      []string{"initiate_complete", "brew_complete"},
		TotalCostUSD:    0.125,
		InteriorPDFPath: "/some/interior.pdf",
		CoverPDFPath:    "/some/cover.pdf",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var m2 Manifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if m2.Kiln.Version != 1 {
		t.Errorf("expected Version 1, got %d", m2.Kiln.Version)
	}
	if len(m2.Kiln.Milestones) != 2 || m2.Kiln.Milestones[0] != "initiate_complete" || m2.Kiln.Milestones[1] != "brew_complete" {
		t.Errorf("expected milestones [initiate_complete, brew_complete], got %v", m2.Kiln.Milestones)
	}
	if m2.Kiln.TotalCostUSD != 0.125 {
		t.Errorf("expected TotalCostUSD 0.125, got %f", m2.Kiln.TotalCostUSD)
	}
	if m2.Kiln.InteriorPDFPath != "/some/interior.pdf" {
		t.Errorf("expected InteriorPDFPath /some/interior.pdf, got %s", m2.Kiln.InteriorPDFPath)
	}
	if m2.Kiln.CoverPDFPath != "/some/cover.pdf" {
		t.Errorf("expected CoverPDFPath /some/cover.pdf, got %s", m2.Kiln.CoverPDFPath)
	}
}

func TestKilnSyncHelpers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := NewManifest(manifestPath)

	if err := m.AddMilestone("initiate_complete"); err != nil {
		t.Fatalf("AddMilestone failed: %v", err)
	}
	if len(m.Kiln.Milestones) != 1 || m.Kiln.Milestones[0] != "initiate_complete" {
		t.Errorf("expected milestones [initiate_complete], got %v", m.Kiln.Milestones)
	}

	// Test duplicate prevention
	if err := m.AddMilestone("initiate_complete"); err != nil {
		t.Fatalf("AddMilestone failed: %v", err)
	}
	if len(m.Kiln.Milestones) != 1 {
		t.Errorf("expected milestone to not be duplicated, got len %d", len(m.Kiln.Milestones))
	}

	if err := m.AddMilestone("brew_complete"); err != nil {
		t.Fatalf("AddMilestone failed: %v", err)
	}
	if len(m.Kiln.Milestones) != 2 || m.Kiln.Milestones[1] != "brew_complete" {
		t.Errorf("expected milestones [initiate_complete, brew_complete], got %v", m.Kiln.Milestones)
	}

	if err := m.UpdatePDFPaths("/path/interior.pdf", "/path/cover.pdf"); err != nil {
		t.Fatalf("UpdatePDFPaths failed: %v", err)
	}
	if m.Kiln.InteriorPDFPath != "/path/interior.pdf" || m.Kiln.CoverPDFPath != "/path/cover.pdf" {
		t.Errorf("expected pdf paths to be saved, got interior=%s, cover=%s", m.Kiln.InteriorPDFPath, m.Kiln.CoverPDFPath)
	}
}

func TestUpdateTotalCostKilnSync(t *testing.T) {
	m := NewManifest("")
	m.Telemetry.ModelUsages["model-1"] = &telemetry.ModelUsage{InputTokens: 1000, OutputTokens: 2000, CachedTokens: 0}
	m.Telemetry.ImageGenerations = 2

	pricing := map[string]telemetry.ModelPricing{
		"model-1":  {Input: 1.0, Output: 2.0},
		"imagegen": {Input: 40000.0},
	}

	m.UpdateTotalCost(pricing)

	expectedCost := 0.085
	if m.Kiln.TotalCostUSD != expectedCost {
		t.Errorf("expected Kiln.TotalCostUSD to match Telemetry.TotalCostUSD %f, got %f", expectedCost, m.Kiln.TotalCostUSD)
	}
}

func TestLoadManifest_NilFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	rawJSON := `{
		"book_properties": {"theme": "Test theme"},
		"progress": {"manuscript_generated": false}
	}`
	if writeErr := os.WriteFile(manifestPath, []byte(rawJSON), 0600); writeErr != nil {
		t.Fatalf("failed to write manifest: %v", writeErr)
	}

	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}

	if loaded.AssetRegistry == nil {
		t.Error("expected AssetRegistry to be initialized")
	}
	if loaded.Progress.Pages == nil {
		t.Error("expected Progress.Pages to be initialized")
	}
	if loaded.Telemetry.ModelUsages == nil {
		t.Error("expected Telemetry.ModelUsages to be initialized")
	}
	if loaded.Kiln.Milestones == nil {
		t.Error("expected Kiln.Milestones to be initialized")
	}
	if loaded.Kiln.Version != 1 {
		t.Errorf("expected Kiln.Version to be initialized to 1, got %d", loaded.Kiln.Version)
	}
	if loaded.BookProperties.CharacterWeight != 100 {
		t.Errorf("expected default CharacterWeight 100, got %d", loaded.BookProperties.CharacterWeight)
	}
}

func TestManifestSchemaVersionAndMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// 1. NewManifest sets CurrentSchemaVersion
	mNew := NewManifest(filepath.Join(tmpDir, "new_manifest.json"))
	if mNew.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected NewManifest SchemaVersion to be %d, got %d", CurrentSchemaVersion, mNew.SchemaVersion)
	}

	// 2. Unversioned legacy manifest automatically migrates to CurrentSchemaVersion and saves to disk
	legacyPath := filepath.Join(tmpDir, "legacy_manifest.json")
	legacyJSON := `{
		"book_properties": {"theme": "Legacy Parody"},
		"progress": {"manuscript_generated": true}
	}`
	if writeErr := os.WriteFile(legacyPath, []byte(legacyJSON), 0600); writeErr != nil {
		t.Fatalf("failed to write legacy manifest: %v", writeErr)
	}

	loadedLegacy, err := LoadManifest(legacyPath)
	if err != nil {
		t.Fatalf("failed to load legacy manifest: %v", err)
	}

	if loadedLegacy.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected loaded legacy manifest SchemaVersion to be %d, got %d", CurrentSchemaVersion, loadedLegacy.SchemaVersion)
	}

	// Check file on disk to confirm automatic save after migration
	//nolint:gosec // ReadFile path is constructed in local CLI test environment
	diskData, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("failed to read migrated file from disk: %v", err)
	}
	var rawMap map[string]interface{}
	if parseErr := json.Unmarshal(diskData, &rawMap); parseErr != nil {
		t.Fatalf("failed to parse migrated json from disk: %v", parseErr)
	}
	if ver, ok := rawMap["schema_version"].(float64); !ok || int(ver) != CurrentSchemaVersion {
		t.Errorf("expected disk json schema_version to be %d, got %v", CurrentSchemaVersion, rawMap["schema_version"])
	}

	// 3. Manifest already at CurrentSchemaVersion loads cleanly
	loadedV2, err := LoadManifest(legacyPath)
	if err != nil {
		t.Fatalf("failed reloading v2 manifest: %v", err)
	}
	if loadedV2.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("expected SchemaVersion %d, got %d", CurrentSchemaVersion, loadedV2.SchemaVersion)
	}

	// 4. Manifest with newer schema version than supported fails to load to prevent data corruption
	futurePath := filepath.Join(tmpDir, "future_manifest.json")
	futureJSON := `{
		"schema_version": 99,
		"book_properties": {"theme": "Future Parody"}
	}`
	if writeErr := os.WriteFile(futurePath, []byte(futureJSON), 0600); writeErr != nil {
		t.Fatalf("failed to write future manifest: %v", writeErr)
	}
	if _, futureErr := LoadManifest(futurePath); futureErr == nil {
		t.Error("expected error loading manifest with future schema version, got nil")
	}

	// 5. Manifest with invalid negative schema version fails to load
	negativePath := filepath.Join(tmpDir, "negative_manifest.json")
	negativeJSON := `{
		"schema_version": -1,
		"book_properties": {"theme": "Invalid Version Parody"}
	}`
	if writeErr := os.WriteFile(negativePath, []byte(negativeJSON), 0600); writeErr != nil {
		t.Fatalf("failed to write negative manifest: %v", writeErr)
	}
	if _, negErr := LoadManifest(negativePath); negErr == nil {
		t.Error("expected error loading manifest with negative schema version, got nil")
	}
}
