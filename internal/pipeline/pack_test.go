package pipeline

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/borch-ai/pithos/internal/manifest"
)

func setupTestBookWorkspace(t *testing.T, preflightPassed bool, withMilestone string) (string, *manifest.Manifest) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "pithos-pack-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	m := manifest.NewManifest(manifestPath)
	m.BookProperties.Theme = "Existential Parody"
	m.BookProperties.Format = "6x9"

	// Write mock interior.pdf and cover.pdf
	interiorContent := []byte("%PDF-1.4 Mock Interior PDF Content")
	coverContent := []byte("%PDF-1.4 Mock Cover PDF Content")

	if err := os.WriteFile(filepath.Join(tmpDir, "interior.pdf"), interiorContent, 0600); err != nil {
		t.Fatalf("failed to write interior.pdf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "cover.pdf"), coverContent, 0600); err != nil {
		t.Fatalf("failed to write cover.pdf: %v", err)
	}

	m.AssetRegistry["interior_pdf"] = filepath.Join(tmpDir, "interior.pdf")
	m.AssetRegistry["cover_pdf"] = filepath.Join(tmpDir, "cover.pdf")

	if preflightPassed {
		m.Progress.PreflightPassed = true
		m.Progress.PreflightHashes = map[string]string{
			"interior.pdf": computeBytesSHA256(interiorContent),
			"cover.pdf":    computeBytesSHA256(coverContent),
		}
	}
	if withMilestone != "" {
		if err := m.AddMilestone(withMilestone); err != nil {
			t.Fatalf("failed to add milestone: %v", err)
		}
	}

	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	return tmpDir, m
}

func TestPackWorkspace_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("empty workspace dir", func(t *testing.T) {
		_, err := PackWorkspace(ctx, PackOptions{})
		if err == nil || !strings.Contains(err.Error(), "workspace directory path is required") {
			t.Fatalf("expected workspace required error, got: %v", err)
		}
	})

	t.Run("missing manifest.json", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "pithos-missing-manifest-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		_, err = PackWorkspace(ctx, PackOptions{WorkspaceDir: tmpDir})
		if err == nil || !strings.Contains(err.Error(), "manifest.json not found") {
			t.Fatalf("expected missing manifest error, got: %v", err)
		}
	})

	t.Run("preflight not passed and not forced", func(t *testing.T) {
		wsDir, _ := setupTestBookWorkspace(t, false, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "preflight validation has not passed") {
			t.Fatalf("expected preflight not passed error, got: %v", err)
		}
	})

	t.Run("missing interior.pdf", func(t *testing.T) {
		wsDir, m := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
		delete(m.AssetRegistry, "interior_pdf")
		_ = m.Save()

		_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "missing required interior PDF") {
			t.Fatalf("expected missing interior PDF error, got: %v", err)
		}
	})

	t.Run("missing cover.pdf without force", func(t *testing.T) {
		wsDir, m := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_ = os.Remove(filepath.Join(wsDir, "cover.pdf"))
		delete(m.AssetRegistry, "cover_pdf")
		_ = m.Save()

		_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "missing required cover PDF") {
			t.Fatalf("expected missing cover PDF error, got: %v", err)
		}
	})
}

//nolint:funlen,gocognit // Test comprehensively verifies zip entries, checksums, and manifest
func TestPackWorkspace_Success(t *testing.T) {
	ctx := context.Background()
	wsDir, _ := setupTestBookWorkspace(t, true, "assemble_complete")
	defer func() { _ = os.RemoveAll(wsDir) }()

	bundle, err := PackWorkspace(ctx, PackOptions{
		WorkspaceDir: wsDir,
	})
	if err != nil {
		t.Fatalf("PackWorkspace failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("expected non-nil release bundle")
	}

	expectedArchivePath := filepath.Join(wsDir, "dist", filepath.Base(wsDir)+"-print-ready.zip")
	relExpected, _ := filepath.Rel(wsDir, expectedArchivePath)
	if bundle.ArchivePath != relExpected && bundle.ArchivePath != expectedArchivePath {
		t.Errorf("expected archive path %q, got %q", relExpected, bundle.ArchivePath)
	}

	// Verify checksums in bundle
	if bundle.Checksums["interior.pdf"] == "" {
		t.Error("expected checksum for interior.pdf")
	}
	if bundle.Checksums["cover.pdf"] == "" {
		t.Error("expected checksum for cover.pdf")
	}
	if bundle.Checksums["manifest.json"] == "" {
		t.Error("expected checksum for manifest.json")
	}

	// Verify checksums.sha256 file on disk
	csPath := filepath.Join(wsDir, "dist", "checksums.sha256")
	//nolint:gosec // csPath is validated within temporary test directory
	csData, err := os.ReadFile(csPath)
	if err != nil {
		t.Fatalf("failed to read checksums.sha256: %v", err)
	}
	csContent := string(csData)
	if !strings.Contains(csContent, "interior.pdf") || !strings.Contains(csContent, "cover.pdf") || !strings.Contains(csContent, "manifest.json") {
		t.Errorf("unexpected checksums content: %s", csContent)
	}

	// Verify zip contents
	zr, err := zip.OpenReader(expectedArchivePath)
	if err != nil {
		t.Fatalf("failed to open zip archive: %v", err)
	}
	defer func() { _ = zr.Close() }()

	foundFiles := make(map[string]bool)
	for _, f := range zr.File {
		foundFiles[f.Name] = true

		rc, openErr := f.Open()
		if openErr != nil {
			t.Fatalf("failed to open zip entry %q: %v", f.Name, openErr)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			t.Fatalf("failed to read zip entry %q: %v", f.Name, readErr)
		}

		// Verify SHA-256 matches bundle checksum
		if expectedHash, ok := bundle.Checksums[f.Name]; ok {
			h := sha256.Sum256(data)
			actualHash := hex.EncodeToString(h[:])
			if actualHash != expectedHash {
				t.Errorf("checksum mismatch for %q: expected %s, got %s", f.Name, expectedHash, actualHash)
			}
		}
	}

	for _, expected := range []string{"interior.pdf", "cover.pdf", "manifest.json", "checksums.sha256"} {
		if !foundFiles[expected] {
			t.Errorf("expected entry %q in zip archive, but was missing", expected)
		}
	}

	// Verify manifest on disk recorded release bundle
	loadedM, err := manifest.LoadManifest(filepath.Join(wsDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if loadedM.ReleaseBundle == nil {
		t.Fatal("expected manifest.ReleaseBundle to be recorded")
	}
	if loadedM.ReleaseBundle.ArchivePath != bundle.ArchivePath {
		t.Errorf("expected manifest release bundle path %q, got %q", bundle.ArchivePath, loadedM.ReleaseBundle.ArchivePath)
	}
}

func TestPackWorkspace_PreflightMilestoneVariants(t *testing.T) {
	ctx := context.Background()

	t.Run("preflight_passed via manifest field", func(t *testing.T) {
		wsDir, _ := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		bundle, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err != nil {
			t.Fatalf("expected pack to succeed with preflight_passed true, got: %v", err)
		}
		if bundle == nil {
			t.Fatal("expected non-nil bundle")
		}
	})

	t.Run("assemble_complete milestone alone does not bypass preflight gate", func(t *testing.T) {
		wsDir, _ := setupTestBookWorkspace(t, false, "assemble_complete")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "preflight validation has not passed") {
			t.Fatalf("expected preflight not passed error, got: %v", err)
		}
	})
}

func TestPackWorkspace_ForceFlagPreflightAndDeliverables(t *testing.T) {
	ctx := context.Background()

	t.Run("force flag bypasses unverified preflight", func(t *testing.T) {
		wsDir, _ := setupTestBookWorkspace(t, false, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		bundle, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			Force:        true,
		})
		if err != nil {
			t.Fatalf("expected pack to succeed with --force, got: %v", err)
		}
		if bundle == nil {
			t.Fatal("expected non-nil bundle")
		}
	})

	t.Run("force flag does not allow missing cover", func(t *testing.T) {
		wsDir, m := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_ = os.Remove(filepath.Join(wsDir, "cover.pdf"))
		delete(m.AssetRegistry, "cover_pdf")
		_ = m.Save()

		_, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			Force:        true,
		})
		if err == nil || !strings.Contains(err.Error(), "missing required cover PDF deliverable") {
			t.Fatalf("expected missing cover error even with force, got: %v", err)
		}
	})
}

func TestPackWorkspace_NormalizedManifestExport(t *testing.T) {
	ctx := context.Background()
	wsDir, m := setupTestBookWorkspace(t, true, "")
	defer func() { _ = os.RemoveAll(wsDir) }()

	altInterior := filepath.Join(wsDir, "alt_interior.pdf")
	altInteriorBytes := []byte("%PDF-1.4 Alt Interior Content")
	if err := os.WriteFile(altInterior, altInteriorBytes, 0600); err != nil {
		t.Fatalf("failed to write alt_interior.pdf: %v", err)
	}
	m.AssetRegistry["interior_pdf"] = altInterior
	_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
	m.Progress.PreflightHashes["interior.pdf"] = computeBytesSHA256(altInteriorBytes)
	if err := m.Save(); err != nil {
		t.Fatalf("failed to save manifest: %v", err)
	}

	bundle, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if err != nil {
		t.Fatalf("PackWorkspace failed with custom interior asset: %v", err)
	}

	zipPath := filepath.Join(wsDir, bundle.ArchivePath)
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer func() { _ = r.Close() }()

	var manifestFound bool
	for _, f := range r.File {
		if f.Name == "manifest.json" {
			manifestFound = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to read manifest from zip: %v", err)
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()

			contentStr := string(data)
			if !strings.Contains(contentStr, `"interior_pdf_path": "interior.pdf"`) {
				t.Errorf("expected interior_pdf_path to be normalized to interior.pdf in zip manifest, got: %s", contentStr)
			}
			if !strings.Contains(contentStr, `"interior_pdf": "interior.pdf"`) {
				t.Errorf("expected interior_pdf asset to be normalized to interior.pdf in zip manifest, got: %s", contentStr)
			}
		}
	}
	if !manifestFound {
		t.Fatal("manifest.json not found in archive")
	}
}

func TestPackWorkspace_CustomOutputPaths(t *testing.T) {
	ctx := context.Background()
	wsDir, _ := setupTestBookWorkspace(t, true, "")
	defer func() { _ = os.RemoveAll(wsDir) }()

	customOutDir, err := os.MkdirTemp("", "pithos-custom-out-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(customOutDir) }()

	t.Run("relative custom output path resolved relative to workspace", func(t *testing.T) {
		bundle, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			OutputPath:   "releases",
		})
		if err != nil {
			t.Fatalf("PackWorkspace failed with relative OutputPath: %v", err)
		}

		expectedZip := filepath.Join(wsDir, "releases", filepath.Base(wsDir)+"-print-ready.zip")
		if _, err := os.Stat(expectedZip); err != nil {
			t.Fatalf("expected zip to exist at %q: %v", expectedZip, err)
		}
		relExpected := filepath.Join("releases", filepath.Base(wsDir)+"-print-ready.zip")
		if bundle.ArchivePath != relExpected {
			t.Errorf("expected archive path %q, got %q", relExpected, bundle.ArchivePath)
		}
	})

	t.Run("output path as zip file", func(t *testing.T) {
		targetZip := filepath.Join(customOutDir, "my-custom-package.zip")
		bundle, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			OutputPath:   targetZip,
		})
		if err != nil {
			t.Fatalf("PackWorkspace failed: %v", err)
		}
		if bundle.ArchivePath != targetZip {
			t.Errorf("expected archive path %q, got %q", targetZip, bundle.ArchivePath)
		}
		if _, statErr := os.Stat(targetZip); statErr != nil {
			t.Errorf("expected target zip file %q to exist: %v", targetZip, statErr)
		}
	})

	t.Run("output path as directory", func(t *testing.T) {
		bundle, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			OutputPath:   customOutDir,
		})
		if err != nil {
			t.Fatalf("PackWorkspace failed: %v", err)
		}
		expectedPath := filepath.Join(customOutDir, filepath.Base(wsDir)+"-print-ready.zip")
		if bundle.ArchivePath != expectedPath {
			t.Errorf("expected archive path %q, got %q", expectedPath, bundle.ArchivePath)
		}
		if _, statErr := os.Stat(expectedPath); statErr != nil {
			t.Errorf("expected target zip file %q to exist: %v", expectedPath, statErr)
		}
	})
}

func TestPackWorkspace_DryRun(t *testing.T) {
	ctx := context.Background()
	wsDir, _ := setupTestBookWorkspace(t, true, "")
	defer func() { _ = os.RemoveAll(wsDir) }()

	bundle, err := PackWorkspace(ctx, PackOptions{
		WorkspaceDir: wsDir,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("PackWorkspace dry-run failed: %v", err)
	}

	if bundle == nil {
		t.Fatal("expected non-nil bundle in dry-run")
	}
	if !strings.HasPrefix(bundle.Checksums["interior.pdf"], "dry-run-") {
		t.Errorf("expected dry-run checksum prefix, got %q", bundle.Checksums["interior.pdf"])
	}

	// Verify manifest records release bundle even in dry run
	loadedM, err := manifest.LoadManifest(filepath.Join(wsDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if loadedM.ReleaseBundle == nil {
		t.Error("expected release bundle recorded in manifest during dry-run")
	}
}

//nolint:funlen,gocognit // Test covers multiple helper and error pathways
func TestPackWorkspace_HelpersAndErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("computeFileSHA256 error on missing file", func(t *testing.T) {
		_, err := computeFileSHA256("/nonexistent/file/path.pdf")
		if err == nil {
			t.Fatal("expected error on missing file, got nil")
		}
	})

	t.Run("addBytesToZip error on compressor creation failure", func(t *testing.T) {
		var buf strings.Builder
		zw := zip.NewWriter(&buf)
		zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
			return nil, errors.New("failed to initialize compressor")
		})
		err := addBytesToZip(zw, "test.pdf", []byte("hello"))
		if err == nil || !strings.Contains(err.Error(), "failed to create entry") {
			t.Fatalf("expected create entry error, got: %v", err)
		}
	})

	t.Run("corrupted manifest json", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "pithos-corrupted-manifest-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		if writeErr := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), []byte("invalid json {{"), 0600); writeErr != nil {
			t.Fatalf("failed to write corrupted manifest: %v", writeErr)
		}

		_, err = PackWorkspace(ctx, PackOptions{WorkspaceDir: tmpDir})
		if err == nil || !strings.Contains(err.Error(), "failed to load manifest") {
			t.Fatalf("expected load manifest error, got: %v", err)
		}
	})

	t.Run("invalid output directory", func(t *testing.T) {
		wsDir, _ := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		// Create a file where directory is expected
		blockerFile := filepath.Join(wsDir, "blocker-file")
		if writeErr := os.WriteFile(blockerFile, []byte("i am a file"), 0600); writeErr != nil {
			t.Fatalf("failed to write blocker file: %v", writeErr)
		}

		_, err := PackWorkspace(ctx, PackOptions{
			WorkspaceDir: wsDir,
			OutputPath:   filepath.Join(blockerFile, "dist", "out.zip"),
		})
		if err == nil || !strings.Contains(err.Error(), "failed to create output archive directory") {
			t.Fatalf("expected create output directory error, got: %v", err)
		}
	})

	t.Run("registered asset fallback paths", func(t *testing.T) {
		wsDir, m := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		// Place custom deliverables inside wsDir under custom names and point via AssetRegistry
		altDir := filepath.Join(wsDir, "custom_build")
		if err := os.MkdirAll(altDir, 0750); err != nil {
			t.Fatalf("failed to create custom dir: %v", err)
		}

		altInterior := filepath.Join(altDir, "alt_interior.pdf")
		altCover := filepath.Join(altDir, "alt_cover.pdf")

		altInteriorBytes := []byte("%PDF-1.4 alt interior")
		altCoverBytes := []byte("%PDF-1.4 alt cover")

		if writeErr := os.WriteFile(altInterior, altInteriorBytes, 0600); writeErr != nil {
			t.Fatalf("failed to write alt interior: %v", writeErr)
		}
		if writeErr := os.WriteFile(altCover, altCoverBytes, 0600); writeErr != nil {
			t.Fatalf("failed to write alt cover: %v", writeErr)
		}

		_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
		_ = os.Remove(filepath.Join(wsDir, "cover.pdf"))

		m.AssetRegistry["interior_pdf"] = altInterior
		m.AssetRegistry["cover_pdf"] = altCover
		m.Progress.PreflightHashes["interior.pdf"] = computeBytesSHA256(altInteriorBytes)
		m.Progress.PreflightHashes["cover.pdf"] = computeBytesSHA256(altCoverBytes)
		if saveErr := m.Save(); saveErr != nil {
			t.Fatalf("failed to save manifest: %v", saveErr)
		}

		bundle, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err != nil {
			t.Fatalf("PackWorkspace with registered asset paths failed: %v", err)
		}
		if bundle.Checksums["interior.pdf"] == "" || bundle.Checksums["cover.pdf"] == "" {
			t.Error("expected checksums for both interior and cover with registered asset paths")
		}
	})

	t.Run("registered asset path nonexistent errors", func(t *testing.T) {
		wsDir, m := setupTestBookWorkspace(t, true, "")
		defer func() { _ = os.RemoveAll(wsDir) }()

		_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
		m.AssetRegistry["interior_pdf"] = "/nonexistent/interior.pdf"
		_ = m.Save()

		_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "missing required interior PDF deliverable") {
			t.Fatalf("expected missing interior deliverable error, got: %v", err)
		}

		// Restore interior and test missing cover with nonexistent registered path
		if writeErr := os.WriteFile(filepath.Join(wsDir, "interior.pdf"), []byte("%PDF-1.4 interior"), 0600); writeErr != nil {
			t.Fatalf("failed to write interior.pdf: %v", writeErr)
		}
		m.AssetRegistry["interior_pdf"] = filepath.Join(wsDir, "interior.pdf")
		_ = os.Remove(filepath.Join(wsDir, "cover.pdf"))
		m.AssetRegistry["cover_pdf"] = "/nonexistent/cover.pdf"
		_ = m.Save()

		_, err = PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
		if err == nil || !strings.Contains(err.Error(), "missing required cover PDF deliverable") {
			t.Fatalf("expected missing cover deliverable error, got: %v", err)
		}
	})
}

func TestCreateZipArchive_Errors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos-zip-err-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	validInterior := filepath.Join(tmpDir, "interior.pdf")
	if writeErr := os.WriteFile(validInterior, []byte("%PDF-1.4 interior"), 0600); writeErr != nil {
		t.Fatalf("failed to write valid interior: %v", writeErr)
	}

	interiorBytes := []byte("%PDF-1.4 interior")
	coverBytes := []byte("%PDF-1.4 cover")
	manifestBytes := []byte("{}")
	checksumsBytes := []byte("hash  interior.pdf\n")
	checksums := map[string]string{
		"interior.pdf":  computeBytesSHA256(interiorBytes),
		"cover.pdf":     computeBytesSHA256(coverBytes),
		"manifest.json": computeBytesSHA256(manifestBytes),
	}

	// 1. Invalid outDir -> CreateTemp failure
	err = createZipArchive(filepath.Join(validInterior, "out.zip"), interiorBytes, coverBytes, manifestBytes, checksumsBytes, checksums)
	if err == nil {
		t.Error("expected createTemp error, got nil")
	}

	// 2. Checksum mismatch -> verifyZipArchive failure
	badChecksums := map[string]string{
		"interior.pdf":  "mismatched_hash",
		"cover.pdf":     computeBytesSHA256(coverBytes),
		"manifest.json": computeBytesSHA256(manifestBytes),
	}
	err = createZipArchive(filepath.Join(tmpDir, "out1.zip"), interiorBytes, coverBytes, manifestBytes, checksumsBytes, badChecksums)
	if err == nil || !strings.Contains(err.Error(), "archive verification failed") {
		t.Errorf("expected archive verification error for bad checksum, got: %v", err)
	}

	// 3. Missing expected entry in zip -> verifyZipArchive failure
	missingEntryChecksums := map[string]string{
		"nonexistent.pdf": "some_hash",
	}
	err = createZipArchive(filepath.Join(tmpDir, "out2.zip"), interiorBytes, coverBytes, manifestBytes, checksumsBytes, missingEntryChecksums)
	if err == nil || !strings.Contains(err.Error(), "missing expected entry") {
		t.Errorf("expected missing expected entry error, got: %v", err)
	}

	// 4. Destination path rename failure when target is directory
	err = createZipArchive(tmpDir, interiorBytes, coverBytes, manifestBytes, checksumsBytes, checksums)
	if err == nil || !strings.Contains(err.Error(), "failed to place distribution archive") {
		t.Errorf("expected rename error when archive path is directory, got: %v", err)
	}

	// 5. verifyZipArchive with corrupt zip file
	corruptZip := filepath.Join(tmpDir, "corrupt.zip")
	_ = os.WriteFile(corruptZip, []byte("not a zip file"), 0600)
	if vErr := verifyZipArchive(corruptZip, checksums); vErr == nil || !strings.Contains(vErr.Error(), "failed to open zip archive") {
		t.Errorf("expected error verifying corrupt zip, got: %v", vErr)
	}
}

func TestDetermineArchivePath_EdgeCases(t *testing.T) {
	p, err := determineArchivePath(".", "")
	if err != nil {
		t.Fatalf("unexpected error for dot slug: %v", err)
	}
	if !strings.HasSuffix(p, "book-print-ready.zip") {
		t.Errorf("expected default book-print-ready.zip for dot slug, got: %s", p)
	}
}

func TestPackWorkspace_PreflightHashMismatch(t *testing.T) {
	ctx := context.Background()
	wsDir, m := setupTestBookWorkspace(t, true, "assemble_complete")
	defer func() { _ = os.RemoveAll(wsDir) }()

	// Overwrite interior.pdf with different bytes without re-running preflight
	alteredInterior := []byte("%PDF-1.4 TAMPERED INTERIOR CONTENT")
	if err := os.WriteFile(filepath.Join(wsDir, "interior.pdf"), alteredInterior, 0600); err != nil {
		t.Fatalf("failed to overwrite interior.pdf: %v", err)
	}

	// Packaging must fail because interior deliverable hash changed
	_, err := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if err == nil || !strings.Contains(err.Error(), "interior.pdf has changed since preflight validation") {
		t.Fatalf("expected preflight hash mismatch error for interior.pdf, got: %v", err)
	}

	// Packaging with --force should succeed despite hash mismatch
	bundle, fErr := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir, Force: true})
	if fErr != nil {
		t.Fatalf("expected --force to succeed despite hash mismatch, got: %v", fErr)
	}
	if bundle == nil {
		t.Fatal("expected non-nil release bundle with --force")
	}

	// Reset interior.pdf to match, but alter cover.pdf
	origInterior := []byte("%PDF-1.4 Mock Interior PDF Content")
	_ = os.WriteFile(filepath.Join(wsDir, "interior.pdf"), origInterior, 0600)
	alteredCover := []byte("%PDF-1.4 TAMPERED COVER CONTENT")
	_ = os.WriteFile(filepath.Join(wsDir, "cover.pdf"), alteredCover, 0600)

	_, cErr := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if cErr == nil || !strings.Contains(cErr.Error(), "cover.pdf has changed since preflight validation") {
		t.Fatalf("expected preflight hash mismatch error for cover.pdf, got: %v", cErr)
	}

	// Test missing preflight verification hashes
	m.Progress.PreflightHashes = nil
	_ = m.Save()
	_, mErr := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if mErr == nil || !strings.Contains(mErr.Error(), "preflight verification hashes missing") {
		t.Fatalf("expected missing verification hashes error, got: %v", mErr)
	}
}

func TestPackWorkspace_PathTraversalOutsideWorkspace(t *testing.T) {
	ctx := context.Background()
	wsDir, m := setupTestBookWorkspace(t, true, "assemble_complete")
	defer func() { _ = os.RemoveAll(wsDir) }()

	extDir, err := os.MkdirTemp("", "pithos-external-*")
	if err != nil {
		t.Fatalf("failed to create ext dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(extDir) }()

	extFile := filepath.Join(extDir, "outside.pdf")
	if err := os.WriteFile(extFile, []byte("%PDF-1.4 secret external file"), 0600); err != nil {
		t.Fatalf("failed to write external file: %v", err)
	}

	_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
	m.AssetRegistry["interior_pdf"] = extFile
	_ = m.Save()

	_, pErr := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if pErr == nil || !strings.Contains(pErr.Error(), "resolves outside workspace directory") {
		t.Fatalf("expected outside workspace error, got: %v", pErr)
	}
}

func TestPackWorkspace_SymlinkOutsideWorkspace(t *testing.T) {
	ctx := context.Background()
	wsDir, _ := setupTestBookWorkspace(t, true, "assemble_complete")
	defer func() { _ = os.RemoveAll(wsDir) }()

	extDir, err := os.MkdirTemp("", "pithos-symlink-ext-*")
	if err != nil {
		t.Fatalf("failed to create ext dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(extDir) }()

	extFile := filepath.Join(extDir, "outside_target.pdf")
	if err := os.WriteFile(extFile, []byte("%PDF-1.4 external target"), 0600); err != nil {
		t.Fatalf("failed to write ext file: %v", err)
	}

	_ = os.Remove(filepath.Join(wsDir, "interior.pdf"))
	if err := os.Symlink(extFile, filepath.Join(wsDir, "interior.pdf")); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	_, pErr := PackWorkspace(ctx, PackOptions{WorkspaceDir: wsDir})
	if pErr == nil || !strings.Contains(pErr.Error(), "resolves outside workspace directory") {
		t.Fatalf("expected outside workspace error for symlink, got: %v", pErr)
	}
}

func TestPackWorkspace_ValidateDeliverablePathEdgeCases(t *testing.T) {
	wsDir, err := os.MkdirTemp("", "pithos-val-path-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(wsDir) }()

	if _, err := validateDeliverablePath(wsDir, ""); err == nil {
		t.Error("expected error for empty candidate path")
	}

	dirPath := filepath.Join(wsDir, "subdir")
	if err := os.Mkdir(dirPath, 0750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if _, err := validateDeliverablePath(wsDir, dirPath); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("expected directory error, got: %v", err)
	}
}
