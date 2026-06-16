# plan: Task 3.5: Selective Page Redo / Overrides

**Status:** Completed
**Go Version:** Go 1.26.4
**Date Completed:** 2026-06-16
**Unit Test Coverage:** 91.3%

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
- [x] Reset the status of target pages to `pending` (keeping their original `ImagePath` to prevent data loss on intermediate failures) and execute the generation loop specifically for those pages.
- [x] Precompute a membership set/map of allowed page indices to reduce membership lookup complexity in the illustration generation loop.
- [x] Implement a fast-fail validation check in manuscript import that returns an error prior to mutating any state if there are edits on pages not included in the selective overrides filter.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- [x] Add the `--pages` string flag (e.g. `--pages 4,7`) to the `brew` command to specify page overrides.
- [x] Validate flag inputs, failing fast if the flag is set but parses to empty/zero page numbers (e.g. `--pages " , "`).

---

## Verification Plan

### Automated Tests
- [x] Run command: `go test -v ./internal/pipeline/...`
- [x] Test that only specified pages are reset and regenerated when the filter is active.

### Manual Verification
- [x] Run `pithos brew --pages 2` on an already completed workspace, and verify that only page 2's image is regenerated and overwritten.

