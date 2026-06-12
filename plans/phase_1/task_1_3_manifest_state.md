# plan: Task 1.3: Project State & Resumability Manifest

**Status:** Completed
**Go Version:** Go 1.26.4
**Date Completed:** 2026-06-11

Define the structured schema of `manifest.json` (or `state.json`) which serves as the source-of-truth and checkpoint logger for long-running book generation runs. Create helper routines for loading, writing, validating, and resuming states.
- **Unit Test Coverage:** 91.70% total coverage (meets the 91% threshold constraint)

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### State Management Subsystem

#### [NEW] [manifest.go](file:///Users/human/code/pithos/internal/manifest/manifest.go)
- [x] Define Go structures mirroring the `manifest.json` file structure:
  - [x] **Book Properties:** theme, style, format, target page count.
  - [x] **Generation Progress Checkpoints:** manuscript text generated, cover image generated, page-by-page progress status (`pending`, `generating_images`, `awaiting_approval`, `completed`).
  - [x] **Asset Registry:** maps page numbers/stanzas to local file paths (e.g. `/images/page_1.png`).
  - [x] **KDP Layout Details:** spine width, margin sizes, bleed calculations.
- [x] Implement `LoadManifest(path string) (*Manifest, error)`
- [x] Implement `Save()` and `SaveTo(path string) error` (with atomic file saving mechanism)
- [x] Implement checkpoint helper functions (`UpdatePageStatus`, `UpdateManuscriptStatus`, `UpdateCoverImage`, and `RegisterAsset`) to write state immediately.

---

## Verification Plan

### Automated Tests
- [x] Run command: `go test ./internal/manifest/...`
- [x] Test JSON marshaling/unmarshaling logic.
- [x] Test the file loading and safety of atomic saves (saving to a temp file first, then renaming to prevent corruption).

### Manual Verification
- [x] Verify that a mock pipeline run successfully updates progress in the manifest after each simulated page generation task.
