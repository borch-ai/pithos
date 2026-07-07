package pipeline

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/manifest"
)

//nolint:funlen // E2E pipeline simulation tests are naturally long and sequential
func TestDryRun_E2EPipeline(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test Initiate with DryRun
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Existential Dread of a Software Engineer",
		TargetPageCount: 3,
		DryRun:          true,
	}

	m, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("Initiate dry-run failed: %v", err)
	}

	if m.BookProperties.Style != "Simulated style description" {
		t.Errorf("expected style 'Simulated style description', got %q", m.BookProperties.Style)
	}
	if m.BookProperties.CharacterProfile != "Simulated character profile" {
		t.Errorf("expected character profile 'Simulated character profile', got %q", m.BookProperties.CharacterProfile)
	}

	// 2. Test Brew with DryRun
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	optsBrew := BrewOptions{
		OutputDir:   tempDir,
		Concurrency: 2,
		Silent:      true,
		DryRun:      true,
	}

	err = Brew(ctx, optsBrew)
	if err != nil {
		t.Fatalf("Brew dry-run failed: %v", err)
	}

	// Verify manifest milestones and pages
	m, err = manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}

	expectedMilestones := map[string]bool{
		"initiate_complete": true,
		"brew_complete":     true,
	}
	for _, mil := range m.Kiln.Milestones {
		delete(expectedMilestones, mil)
	}
	if len(expectedMilestones) > 0 {
		t.Errorf("missing expected milestones in manifest: %v", expectedMilestones)
	}

	if len(m.Progress.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(m.Progress.Pages))
	}

	// Verify character seed image is valid PNG
	charSeedPath := filepath.Join(tempDir, "images", "character_seed.png")
	verifyValidPNG(t, charSeedPath)

	if m.BookProperties.CharacterReferenceURL != "http://storage.googleapis.com/simulated-bucket/character_seed.png" {
		t.Errorf("expected character reference URL 'http://storage.googleapis.com/simulated-bucket/character_seed.png', got %q", m.BookProperties.CharacterReferenceURL)
	}

	// Verify page images are valid PNGs
	for i := 1; i <= 3; i++ {
		pageImgPath := filepath.Join(tempDir, "images", "page_"+uintToString(i)+".png")
		verifyValidPNG(t, pageImgPath)
	}

	// 3. Test Assemble with DryRun
	optsAssemble := AssembleOptions{
		InputDir:  tempDir,
		Format:    "paperback",
		Bleed:     true,
		TrimSize:  "6x9",
		PaperType: "white",
		Silent:    true,
		DryRun:    true,
	}

	m, err = Assemble(ctx, optsAssemble)
	if err != nil {
		t.Fatalf("Assemble dry-run failed: %v", err)
	}

	if m.KDPLayout.SpineWidth != 0.15 {
		t.Errorf("expected simulated SpineWidth 0.15, got %f", m.KDPLayout.SpineWidth)
	}
	if m.KDPLayout.CoverWidthInches != 12.5 {
		t.Errorf("expected simulated CoverWidthInches 12.5, got %f", m.KDPLayout.CoverWidthInches)
	}

	// Verify manuscript.md exists and contains simulated text
	//nolint:gosec // tempDir is generated inside test workspace
	manuscriptData, err := os.ReadFile(filepath.Join(tempDir, "manuscript.md"))
	if err != nil {
		t.Fatalf("failed to read manuscript.md: %v", err)
	}
	manuscriptStr := string(manuscriptData)
	if !strings.Contains(manuscriptStr, "This is page 1 simulated stanza.") {
		t.Errorf("manuscript.md missing simulated stanza text: %s", manuscriptStr)
	}

	// Verify interior.pdf
	//nolint:gosec // tempDir is generated inside test workspace
	pdfData, err := os.ReadFile(filepath.Join(tempDir, "interior.pdf"))
	if err != nil {
		t.Fatalf("failed to read interior.pdf: %v", err)
	}
	if string(pdfData) != "SIMULATED PDF CONTENT" {
		t.Errorf("expected interior.pdf to contain 'SIMULATED PDF CONTENT', got %q", string(pdfData))
	}
}

func TestGenerateCharacterSeed_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize manifest
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Dry Run Character Theme",
		TargetPageCount: 1,
		DryRun:          true,
	}
	_, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	optsChar := CharacterOptions{
		OutputDir: tempDir,
		DryRun:    true,
	}

	err = GenerateCharacterSeed(ctx, optsChar)
	if err != nil {
		t.Fatalf("GenerateCharacterSeed dry-run failed: %v", err)
	}

	charSeedPath := filepath.Join(tempDir, "images", "character_seed.png")
	verifyValidPNG(t, charSeedPath)

	m, err := manifest.LoadManifest(filepath.Join(tempDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if m.BookProperties.CharacterReferenceURL != "http://storage.googleapis.com/simulated-bucket/character_seed.png" {
		t.Errorf("expected character reference URL 'http://storage.googleapis.com/simulated-bucket/character_seed.png', got %q", m.BookProperties.CharacterReferenceURL)
	}
}

func uintToString(val int) string {
	switch val {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	default:
		return "unknown"
	}
}

func verifyValidPNG(t *testing.T, path string) {
	t.Helper()
	//nolint:gosec // path is generated inside test workspace
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open PNG file %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	_, err = png.Decode(f)
	if err != nil {
		t.Errorf("file %s is not a valid PNG: %v", path, err)
	}
}

func TestWriteDummyPNG_Errors(t *testing.T) {
	// 1. MkdirAll fails
	tempDir := t.TempDir()
	blockedPath := filepath.Join(tempDir, "blocked_file")
	if err := os.WriteFile(blockedPath, []byte("data"), 0600); err != nil {
		t.Fatalf("failed to setup blocked file: %v", err)
	}
	// Try creating a directory under a regular file
	err := writeDummyPNG(filepath.Join(blockedPath, "dir", "test.png"))
	if err == nil {
		t.Error("expected MkdirAll to fail, got nil")
	}

	// 2. os.Create fails (read-only directory or invalid name)
	// Try creating a file with an invalid/empty filename in an existing directory
	err = writeDummyPNG(tempDir) // tempDir is a directory, not a file path we can create
	if err == nil {
		t.Error("expected os.Create to fail when target is a directory, got nil")
	}
}

func TestDryRun_Assemble_WriteFileError(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize manifest
	optsInit := InitiateOptions{
		OutputDir:       tempDir,
		Theme:           "Existential Dread",
		TargetPageCount: 3,
		DryRun:          true,
	}
	_, err := Initiate(optsInit)
	if err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// Block the output path with a directory
	pdfPath := filepath.Join(tempDir, "interior.pdf")
	if mkdirErr := os.Mkdir(pdfPath, 0750); mkdirErr != nil {
		t.Fatalf("failed to create directory: %v", mkdirErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	optsAssemble := AssembleOptions{
		InputDir: tempDir,
		Silent:   true,
		DryRun:   true,
	}
	_, err = Assemble(ctx, optsAssemble)
	if err == nil {
		t.Error("expected Assemble to fail when interior.pdf output path is blocked by a directory, got nil")
	}
}
