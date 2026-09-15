# plan: Task 5.54: Print-Ready Artifact Packaging & Integrity Verification (`pithos pack`)

**Status:** Completed
**Date Completed:** 2026-09-13
**Unit Test Coverage:** 91.10%
**Go Version:** 1.26.6

## Overview

Implement an automated packaging and integrity verification command (`pithos pack [book]`) to compile production-ready distribution archives for publishing platforms (such as Amazon KDP or automated distributors).

Currently, Pithos generates interior PDF files (`interior.pdf`) and cover wrap PDFs (`cover.pdf`) directly inside the book workspace directory. However, production deployment requires:
1. **Preflight Certification**: Confirming that `pw-mcp-pdfcheck` preflight verification passed with zero critical errors before assets are packaged.
2. **Checksum Integrity**: Generating SHA-256 cryptographic hashes for all deliverable assets to prevent corrupt or incomplete uploads.
3. **Packaging Artifact**: Bundling `interior.pdf`, `cover.pdf`, a sanitized export of `manifest.json`, and `checksums.sha256` into a compressed distribution archive (`dist/<slug>-print-ready.zip`).

## User Review Required

> [!IMPORTANT]
> **Preflight Enforcement**:
> `pithos pack` will strictly refuse to package deliverables if the manifest does not record a successful preflight check from `assemble`, preventing invalid page geometries or low-resolution covers from reaching distribution channels. A `--force` flag is provided for manual override.

> [!NOTE]
> **Schema Evolution**:
> Manifest schema version will track the addition of the `ReleaseBundle` struct in `manifest.json`, recording archive path, timestamp, and SHA-256 hashes.

## Open Questions

None.

## Proposed Changes

### State & Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add `ReleaseBundle` struct to `BookManifest`:
  ```go
  type ReleaseBundle struct {
      ArchivePath string            `json:"archive_path"`
      PackagedAt  string            `json:"packaged_at"`
      Checksums   map[string]string `json:"checksums"`
  }
  ```
- Add `PreflightHashes map[string]string` to `Progress` to bind preflight certification to exact deliverable hashes.
- Add helper method `RecordReleaseBundle(archivePath string, checksums map[string]string) error`.
- Add helper method `RecordPreflightPassed(hashes map[string]string) error`.

### Packaging Pipeline

#### [NEW] [pack.go](file://../../internal/pipeline/pack.go)
- Implement `PackWorkspace(ctx context.Context, opts PackOptions) (*manifest.ReleaseBundle, error)`:
  - Verify `interior.pdf` and `cover.pdf` exist, are regular files (`!info.Mode().IsRegular()`), and strictly reside inside workspace root (canonical path validation preventing symlink/traversal escape).
  - Capture deliverable bytes in memory and verify SHA-256 matches preflight hashes recorded during assemble (strictly requiring `cover.pdf` unless `--force` flag is set).
  - Write verified captured bytes into zip archive (`dist/<slug>-print-ready.zip`), eliminating TOCTOU discrepancies, rejecting symlinked archive targets.
  - Post-verify archive entries bit-for-bit against advertised checksums.
  - Write `checksums.sha256` file safely, rejecting symlinks and verifying directory bounds within workspace root.
  - Update `manifest.json` with `ReleaseBundle` metadata.
  - Create git checkpoint for packaged workspace.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Preflight TOCTOU hardening: compute and verify deliverable SHA-256 hashes before and after `runPDFPreflightCheck` to ensure deliverables were not modified during verification before recording preflight pass.

### CLI Layer

#### [NEW] [pack.go](file://../../cmd/pithos/pack.go)
- Implement `pithos pack [book]` command:
  - Flags: `--dir` / `-d`, `--force`, `--output` / `-o`.
  - Styled terminal feedback via Lipgloss summarizing packaged contents and SHA-256 hashes.

## Verification Plan

### Automated Tests
- Run unit test suite:
  ```bash
  make test
  ```
- Add unit tests verifying:
  - Preflight verification failure blocks packaging unless `--force` is set.
  - SHA-256 calculation matches actual file bytes.
  - Zip bundle contains `interior.pdf`, `cover.pdf`, `checksums.sha256`, and sanitized manifest.
  - Manifest records release bundle metadata accurately.
- Verify coverage threshold:
  ```bash
  make check-coverage
  ```
