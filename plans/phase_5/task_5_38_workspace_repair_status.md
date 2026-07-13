# plan: Task 5.38: Workspace Status Diagnostics & State Repair Utility

**Status:** Completed
**Go Version:** 1.26.5
**Date Completed:** 2026-07-13

This task adds subcommands (`pithos status` and `pithos clean`) to easily inspect workspace health and repair corrupted or failed page generation states.
- **Unit Test Coverage:** 91.10% total coverage (meets the 91% threshold constraint)

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [status.go](file://../../cmd/pithos/status.go)
- Create a new Cobra command `statusCmd` for `pithos status <book>`.
- The command runner should:
  - Load `manifest.json` for the given book.
  - Print a detailed status dashboard styled with `lipgloss`:
    - Title, theme, target page count, and format.
    - Status grids (Completed, Pending, Failed, Generating) for each page.
    - Accumulated token telemetry and USD cost metrics.

#### [NEW] [clean.go](file://../../cmd/pithos/clean.go)
- Create a new Cobra command `cleanCmd` for `pithos clean <book>`.
- Supports flags:
  - `--orphans`: Delete images in the images folder that are not referenced in the manifest.
  - `--reset-failed`: Revert any failed or stuck pages back to `pending` status so they can be re-brewed.
  - `--all`: Revert all pages to pending status.

### Pipeline Engine

#### [NEW] [status.go](file://../../internal/pipeline/status.go)
- Implement state inspection functions that load the manifest and compile structural diagnostics.

#### [NEW] [clean.go](file://../../internal/pipeline/clean.go)
- Implement the cleanup and state reset logic, modifying files and saving updates back to `manifest.json`.

---

## Final Implementation Details

### Input Validation
- bookName validation rejects Windows volume-qualified paths (e.g. `C:`) using `filepath.VolumeName` and checks path separators/colons using `strings.ContainsAny(bookName, "/\\:")` for consistent cross-platform validation.

### Workspace Status
- Completed pages without an image path are classified as `Unknown` to surface manifest corruption.

### Clean & Orphan Pruning
- Bare basenames are supported in orphan checks to maintain compatibility with older manifests.
- Manifest updates (e.g. page status resets) are saved back to `manifest.json` before attempting orphaned image file deletions to avoid partially-applied/corrupted workspace state.

---

## Verification Plan

### Automated Tests
- Test that `pithos clean` resets the manifest statuses correctly without deleting referenced images.
- Verify orphan file deletion removes only unreferenced files.
- Run `go test ./...` and `make check-coverage` to confirm unit tests pass and code coverage is at least **91.10%**.

### Manual Verification
- Simulate a failed image generation (status: `"generating"` or failed in manifest).
- Run `pithos status book_name` to view the diagnostic dashboard.
- Run `pithos clean book_name --reset-failed` and confirm the page status reverts back to `"pending"`.
