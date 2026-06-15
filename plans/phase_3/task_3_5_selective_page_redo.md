# plan: Task 3.5: Selective Page Redo / Overrides

**Status:** Completed
**Go Version:** Go 1.26.4
**Date Completed:** 2026-06-15
**Unit Test Coverage:** 91.2%

Support selective regeneration of specific pages or stanzas in the `brew` command, allowing users to rerun the pipeline for only a subset of pages instead of manually editing `manifest.json`.

## User Review Required

> [!NOTE]
> None.

---

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- [x] Add support for target page filtering in `Brew` options.
- [x] Modify `generateIllustrations` and `generateManuscript` to accept a slice of page numbers to regenerate.
- [x] Reset the state of only the specified pages (marking them `pending` / empty image paths) and execute the generation loop specifically for those pages.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- [x] Add the `--pages` string flag (e.g. `--pages 4,7`) to the `brew` command to specify page overrides.

---

## Verification Plan

### Automated Tests
- [x] Run command: `go test -v ./internal/pipeline/...`
- [x] Test that only specified pages are reset and regenerated when the filter is active.

### Manual Verification
- [x] Run `pithos brew --pages 2` on an already completed workspace, and verify that only page 2's image is regenerated and overwritten.

