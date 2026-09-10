# plan: Task 5.54: Print-Ready Artifact Packaging & Integrity Verification (`pithos pack`)

**Status:** Open
**Go Version:** 1.26+

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
- Add helper method `RecordReleaseBundle(archivePath string, checksums map[string]string) error`.

### Packaging Pipeline

#### [NEW] [pack.go](file://../../internal/pipeline/pack.go)
- Implement `PackWorkspace(workspaceDir string, opts PackOptions) (*ReleaseBundle, error)`:
  - Verify `interior.pdf` and `cover.pdf` exist in the workspace directory.
  - Verify preflight checks are clean (or `--force` flag is set).
  - Calculate SHA-256 hashes for interior, cover, and manifest.
  - Write `checksums.sha256` file.
  - Create zip archive in `<workspace>/dist/<slug>-print-ready.zip`.
  - Update `manifest.json` with `ReleaseBundle` metadata.

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
