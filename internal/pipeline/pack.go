package pipeline

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/borch-ai/pithos/internal/logger"
	"github.com/borch-ai/pithos/internal/manifest"
)

// PackOptions defines execution settings for compiling a distribution bundle.
type PackOptions struct {
	WorkspaceDir string
	OutputPath   string
	Force        bool
	DryRun       bool
}

// ComputeFileSHA256 computes the SHA-256 hexadecimal checksum of a file on disk.
func ComputeFileSHA256(filePath string) (string, error) {
	//nolint:gosec // filePath is validated within the workspace directory
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file %q: %w", filePath, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func computeFileSHA256(filePath string) (string, error) {
	return ComputeFileSHA256(filePath)
}

// ComputeBytesSHA256 computes the SHA-256 hexadecimal checksum of an in-memory byte slice.
func ComputeBytesSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func computeBytesSHA256(data []byte) string {
	return ComputeBytesSHA256(data)
}

// addBytesToZip writes in-memory bytes into the zip archive with deflate compression.
func addBytesToZip(zw *zip.Writer, nameInZip string, data []byte) error {
	header := &zip.FileHeader{
		Name:     nameInZip,
		Method:   zip.Deflate,
		Modified: time.Now().UTC(),
	}

	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create entry %q in zip: %w", nameInZip, err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write %q to zip: %w", nameInZip, err)
	}
	return nil
}

func validatePreflight(m *manifest.Manifest, interiorHash, coverHash string, force bool) error {
	preflightPassed := m.Progress.PreflightPassed
	if force {
		if !preflightPassed || len(m.Progress.PreflightHashes) == 0 {
			logger.Warn("Packaging forced: preflight validation checks bypassed with --force")
		}
		return nil
	}
	if !preflightPassed {
		return errors.New("cannot package book: preflight validation has not passed (run 'pithos assemble' or pass --force)")
	}
	if len(m.Progress.PreflightHashes) == 0 {
		return errors.New("cannot package book: preflight verification hashes missing (run 'pithos assemble' or pass --force)")
	}
	if interiorHash != "" {
		expected, ok := m.Progress.PreflightHashes["interior.pdf"]
		if !ok {
			return errors.New("cannot package book: deliverable interior.pdf has not passed preflight validation (run 'pithos assemble' or pass --force)")
		}
		if expected != interiorHash {
			return fmt.Errorf("cannot package book: deliverable interior.pdf has changed since preflight validation (expected %s, got %s; run 'pithos assemble' or pass --force)", expected, interiorHash)
		}
	}
	if coverHash != "" {
		expected, ok := m.Progress.PreflightHashes["cover.pdf"]
		if !ok {
			return errors.New("cannot package book: deliverable cover.pdf has not passed preflight validation (run 'pithos assemble' or pass --force)")
		}
		if expected != coverHash {
			return fmt.Errorf("cannot package book: deliverable cover.pdf has changed since preflight validation (expected %s, got %s; run 'pithos assemble' or pass --force)", expected, coverHash)
		}
	}
	return nil
}

// validateDeliverablePath verifies that candidatePath exists, is a regular file, and resolves strictly inside wsDir.
func validateDeliverablePath(wsDir, candidatePath string) (string, error) {
	if candidatePath == "" {
		return "", errors.New("empty path candidate")
	}

	canonicalWs, err := filepath.Abs(wsDir)
	if err != nil {
		return "", fmt.Errorf("failed to determine absolute path for workspace: %w", err)
	}
	if evaluatedWs, symErr := filepath.EvalSymlinks(canonicalWs); symErr == nil {
		canonicalWs = evaluatedWs
	}

	target := candidatePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(wsDir, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %q is not a regular deliverable file", candidatePath)
	}

	canonicalTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed to determine absolute path for %q: %w", candidatePath, err)
	}
	if evaluatedTarget, symErr := filepath.EvalSymlinks(canonicalTarget); symErr == nil {
		canonicalTarget = evaluatedTarget
	}

	rel, err := filepath.Rel(canonicalWs, canonicalTarget)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("deliverable path %q resolves outside workspace directory", candidatePath)
	}

	return canonicalTarget, nil
}

func resolveAssetPath(wsDir, filename, registeredPath string) (string, error) {
	defaultPath := filepath.Join(wsDir, filename)
	if p, err := validateDeliverablePath(wsDir, defaultPath); err == nil {
		return p, nil
	} else if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("invalid deliverable %s: %w", filename, err)
	}

	if registeredPath != "" {
		if p, err := validateDeliverablePath(wsDir, registeredPath); err == nil {
			return p, nil
		} else if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("invalid deliverable %s from asset registry: %w", filename, err)
		}
	}

	return "", fmt.Errorf("%s not found on disk", filename)
}

func locateDeliverables(wsDir string, m *manifest.Manifest) (string, string, error) {
	interiorPath, err := resolveAssetPath(wsDir, "interior.pdf", m.AssetRegistry["interior_pdf"])
	if err != nil {
		return "", "", fmt.Errorf("missing required interior PDF deliverable (interior.pdf): %w", err)
	}

	coverPath, err := resolveAssetPath(wsDir, "cover.pdf", m.AssetRegistry["cover_pdf"])
	if err != nil {
		return "", "", fmt.Errorf("missing required cover PDF deliverable (cover.pdf): %w", err)
	}

	return interiorPath, coverPath, nil
}

func resolveArchivePath(wsDir, outputPath, defaultZip string) string {
	if outputPath == "" {
		return filepath.Join(wsDir, "dist", defaultZip)
	}
	target := outputPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(wsDir, target)
	}
	cleanOut := filepath.Clean(target)
	if strings.HasSuffix(strings.ToLower(cleanOut), ".zip") {
		return cleanOut
	}
	return filepath.Join(cleanOut, defaultZip)
}

func determineArchivePath(wsDir, outputPath string) (string, error) {
	slug := filepath.Base(wsDir)
	if slug == "." || slug == "/" {
		slug = "book"
	}
	defaultZip := fmt.Sprintf("%s-print-ready.zip", slug)
	archivePath := resolveArchivePath(wsDir, outputPath, defaultZip)

	outDir := filepath.Dir(archivePath)
	if mkdirErr := os.MkdirAll(outDir, 0750); mkdirErr != nil {
		return "", fmt.Errorf("failed to create output archive directory %q: %w", outDir, mkdirErr)
	}
	return archivePath, nil
}

func prepareSanitizedManifestAndChecksums(archivePath, interiorHash, coverHash string, m *manifest.Manifest) (map[string]string, []byte, []byte, error) {
	checksums := map[string]string{
		"interior.pdf": interiorHash,
		"cover.pdf":    coverHash,
	}

	sanitizedM, err := m.SanitizedExport()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create sanitized manifest export: %w", err)
	}

	// Normalize PDF paths and asset registry entries to match the exact archive entry names
	sanitizedM.Kiln.InteriorPDFPath = "interior.pdf"
	if sanitizedM.AssetRegistry != nil {
		sanitizedM.AssetRegistry["interior_pdf"] = "interior.pdf"
	}
	sanitizedM.Kiln.CoverPDFPath = "cover.pdf"
	if sanitizedM.AssetRegistry != nil {
		sanitizedM.AssetRegistry["cover_pdf"] = "cover.pdf"
	}

	sanitizedM.ReleaseBundle = &manifest.ReleaseBundle{
		ArchivePath: filepath.Base(archivePath),
		PackagedAt:  time.Now().UTC().Format(time.RFC3339),
		Checksums:   checksums,
	}

	manifestBytes, err := json.MarshalIndent(sanitizedM, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal export manifest: %w", err)
	}
	checksums["manifest.json"] = computeBytesSHA256(manifestBytes)

	keys := make([]string, 0, len(checksums))
	for k := range checksums {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s  %s\n", checksums[k], k)
	}

	return checksums, manifestBytes, []byte(sb.String()), nil
}

func verifyZipArchive(zipPath string, expectedChecksums map[string]string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive for verification: %w", err)
	}
	defer func() { _ = zr.Close() }()

	found := make(map[string]bool)
	for _, f := range zr.File {
		expectedHash, ok := expectedChecksums[f.Name]
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to read entry %q from zip archive: %w", f.Name, err)
		}
		h := sha256.New()
		// Guard against decompression bombs by limiting read to 512MB
		if _, err := io.Copy(h, io.LimitReader(rc, 512*1024*1024)); err != nil {
			_ = rc.Close()
			return fmt.Errorf("failed to hash entry %q from zip archive: %w", f.Name, err)
		}
		_ = rc.Close()

		actualHash := hex.EncodeToString(h.Sum(nil))
		if actualHash != expectedHash {
			return fmt.Errorf("archive entry %q checksum mismatch: expected %s, got %s", f.Name, expectedHash, actualHash)
		}
		found[f.Name] = true
	}

	for name := range expectedChecksums {
		if !found[name] {
			return fmt.Errorf("missing expected entry %q in zip archive", name)
		}
	}
	return nil
}

func createZipArchive(archivePath string, interiorBytes, coverBytes, manifestBytes, checksumsBytes []byte, checksums map[string]string) error {
	if info, err := os.Lstat(archivePath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive path %q is a symlink", archivePath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive path %q is not a regular file", archivePath)
		}
	} else if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect archive path %q: %w", archivePath, err)
	}

	outDir := filepath.Dir(archivePath)
	tmpZip, err := os.CreateTemp(outDir, "pithos-pack-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary archive: %w", err)
	}
	tmpName := tmpZip.Name()
	defer func() {
		if tmpZip != nil {
			_ = tmpZip.Close()
		}
		_ = os.Remove(tmpName)
	}()

	zw := zip.NewWriter(tmpZip)

	if err := addBytesToZip(zw, "interior.pdf", interiorBytes); err != nil {
		return err
	}
	if err := addBytesToZip(zw, "cover.pdf", coverBytes); err != nil {
		return err
	}
	if err := addBytesToZip(zw, "manifest.json", manifestBytes); err != nil {
		return err
	}
	if err := addBytesToZip(zw, "checksums.sha256", checksumsBytes); err != nil {
		return err
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to finalize zip archive: %w", err)
	}
	if err := tmpZip.Sync(); err != nil {
		return fmt.Errorf("failed to sync archive to disk: %w", err)
	}
	if err := tmpZip.Close(); err != nil {
		return fmt.Errorf("failed to close temporary archive: %w", err)
	}
	tmpZip = nil

	if err := verifyZipArchive(tmpName, checksums); err != nil {
		return fmt.Errorf("archive verification failed: %w", err)
	}

	if err := os.Rename(tmpName, archivePath); err != nil {
		return fmt.Errorf("failed to place distribution archive at %q: %w", archivePath, err)
	}
	return nil
}

func executeDryRun(wsDir, archivePath string, m *manifest.Manifest) (*manifest.ReleaseBundle, error) {
	simulatedChecksums := map[string]string{
		"interior.pdf":  "dry-run-interior-hash-00000000000000000000000000000000",
		"cover.pdf":     "dry-run-cover-hash-00000000000000000000000000000000",
		"manifest.json": "dry-run-manifest-hash-00000000000000000000000000000000",
	}
	recordedPath := archivePath
	if rel, relErr := filepath.Rel(wsDir, archivePath); relErr == nil && !strings.HasPrefix(rel, "..") {
		recordedPath = rel
	}
	if err := m.RecordReleaseBundle(recordedPath, simulatedChecksums); err != nil {
		return nil, fmt.Errorf("failed to record simulated release bundle in manifest: %w", err)
	}
	return m.ReleaseBundle, nil
}

// PackWorkspace verifies deliverables and preflight gates, computes cryptographic checksums,
// and packages print-ready assets into a distribution archive.
func PackWorkspace(ctx context.Context, opts PackOptions) (*manifest.ReleaseBundle, error) {
	if opts.WorkspaceDir == "" {
		return nil, errors.New("workspace directory path is required")
	}

	wsDir := filepath.Clean(opts.WorkspaceDir)
	manifestPath := filepath.Join(wsDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("manifest.json not found in workspace %q: %w", wsDir, err)
	}

	m, err := manifest.LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	interiorPath, coverPath, delivErr := locateDeliverables(wsDir, m)
	if delivErr != nil {
		return nil, delivErr
	}

	archivePath, archErr := determineArchivePath(wsDir, opts.OutputPath)
	if archErr != nil {
		return nil, archErr
	}

	if opts.DryRun {
		if valErr := validatePreflight(m, "", "", opts.Force); valErr != nil {
			return nil, valErr
		}
		return executeDryRun(wsDir, archivePath, m)
	}

	// Capture deliverable bytes into memory once to eliminate TOCTOU discrepancies
	//nolint:gosec // interiorPath is validated to reside strictly within wsDir
	interiorBytes, err := os.ReadFile(interiorPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read interior deliverable %q: %w", interiorPath, err)
	}
	//nolint:gosec // coverPath is validated to reside strictly within wsDir
	coverBytes, err := os.ReadFile(coverPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cover deliverable %q: %w", coverPath, err)
	}

	interiorHash := computeBytesSHA256(interiorBytes)
	coverHash := computeBytesSHA256(coverBytes)

	if valErr := validatePreflight(m, interiorHash, coverHash, opts.Force); valErr != nil {
		return nil, valErr
	}

	checksums, manifestBytes, checksumsBytes, prepErr := prepareSanitizedManifestAndChecksums(archivePath, interiorHash, coverHash, m)
	if prepErr != nil {
		return nil, prepErr
	}

	checksumsFilePath := filepath.Join(filepath.Dir(archivePath), "checksums.sha256")
	customOutput := opts.OutputPath != ""
	if writeErr := writeChecksumsFile(wsDir, checksumsFilePath, checksumsBytes, customOutput); writeErr != nil {
		return nil, writeErr
	}

	if zipErr := createZipArchive(archivePath, interiorBytes, coverBytes, manifestBytes, checksumsBytes, checksums); zipErr != nil {
		return nil, zipErr
	}

	recordedPath := archivePath
	if rel, relErr := filepath.Rel(wsDir, archivePath); relErr == nil && !strings.HasPrefix(rel, "..") {
		recordedPath = rel
	}
	if recErr := m.RecordReleaseBundle(recordedPath, checksums); recErr != nil {
		return nil, fmt.Errorf("failed to record release bundle in manifest: %w", recErr)
	}

	if err := Checkpoint(ctx, wsDir, "Packaged print-ready distribution bundle"); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint after packaging: %w", err)
	}
	return m.ReleaseBundle, nil
}

func writeChecksumsFile(wsDir, filePath string, data []byte, customOutput bool) error {
	canonicalWs, err := filepath.Abs(wsDir)
	if err != nil {
		return fmt.Errorf("failed to determine absolute path for workspace: %w", err)
	}
	if evalWs, symErr := filepath.EvalSymlinks(canonicalWs); symErr == nil {
		canonicalWs = evalWs
	}

	outDir := filepath.Dir(filePath)
	canonicalOutDir, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("failed to determine absolute path for output dir %q: %w", outDir, err)
	}
	if evalOutDir, symErr := filepath.EvalSymlinks(canonicalOutDir); symErr == nil {
		canonicalOutDir = evalOutDir
	}

	if !customOutput {
		rel, err := filepath.Rel(canonicalWs, canonicalOutDir)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return fmt.Errorf("output directory %q resolves outside workspace directory", outDir)
		}
	}

	if info, err := os.Lstat(filePath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("checksums file %q is a symlink", filePath)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("checksums file %q is not a regular file", filePath)
		}
	} else if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect checksums file %q: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write checksums file %q: %w", filePath, err)
	}
	return nil
}
