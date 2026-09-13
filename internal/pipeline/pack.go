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

// computeFileSHA256 computes the SHA-256 hexadecimal checksum of a file on disk.
func computeFileSHA256(filePath string) (string, error) {
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

// computeBytesSHA256 computes the SHA-256 hexadecimal checksum of an in-memory byte slice.
func computeBytesSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// addFileToZip copies an existing file from disk into the zip archive with deflate compression.
func addFileToZip(zw *zip.Writer, nameInZip, srcPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat file %q: %w", srcPath, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to create zip header for %q: %w", srcPath, err)
	}
	header.Name = nameInZip
	header.Method = zip.Deflate

	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create entry %q in zip: %w", nameInZip, err)
	}

	//nolint:gosec // srcPath is a local deliverable file inside workspace
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", srcPath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(w, f); err != nil {
		return fmt.Errorf("failed to copy file %q to zip: %w", srcPath, err)
	}
	return nil
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

func validatePreflight(m *manifest.Manifest, force bool) error {
	preflightPassed := m.Progress.PreflightPassed
	if !preflightPassed && !force {
		return errors.New("cannot package book: preflight validation has not passed (run 'pithos assemble' or pass --force)")
	}
	if !preflightPassed && force {
		logger.Warn("Packaging forced: preflight validation checks bypassed with --force")
	}
	return nil
}

func resolveAssetPath(wsDir, filename, registeredPath string) (string, error) {
	p := filepath.Join(wsDir, filename)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if registeredPath != "" {
		if _, err := os.Stat(registeredPath); err == nil {
			return registeredPath, nil
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

func prepareSanitizedManifestAndChecksums(archivePath, interiorPath, coverPath string, m *manifest.Manifest) (map[string]string, []byte, []byte, error) {
	checksums := make(map[string]string)

	interiorHash, err := computeFileSHA256(interiorPath)
	if err != nil {
		return nil, nil, nil, err
	}
	checksums["interior.pdf"] = interiorHash

	coverHash, cErr := computeFileSHA256(coverPath)
	if cErr != nil {
		return nil, nil, nil, cErr
	}
	checksums["cover.pdf"] = coverHash

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

func createZipArchive(archivePath, interiorPath, coverPath string, manifestBytes, checksumsBytes []byte) error {
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

	if err := addFileToZip(zw, "interior.pdf", interiorPath); err != nil {
		return err
	}
	if err := addFileToZip(zw, "cover.pdf", coverPath); err != nil {
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

	if valErr := validatePreflight(m, opts.Force); valErr != nil {
		return nil, valErr
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
		return executeDryRun(wsDir, archivePath, m)
	}

	checksums, manifestBytes, checksumsBytes, prepErr := prepareSanitizedManifestAndChecksums(archivePath, interiorPath, coverPath, m)
	if prepErr != nil {
		return nil, prepErr
	}

	checksumsFilePath := filepath.Join(filepath.Dir(archivePath), "checksums.sha256")
	if writeErr := os.WriteFile(checksumsFilePath, checksumsBytes, 0600); writeErr != nil {
		return nil, fmt.Errorf("failed to write checksums file %q: %w", checksumsFilePath, writeErr)
	}

	if zipErr := createZipArchive(archivePath, interiorPath, coverPath, manifestBytes, checksumsBytes); zipErr != nil {
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
