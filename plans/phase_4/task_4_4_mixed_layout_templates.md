# plan: Task 4.4: Mixed Layout Templates

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Support mixed layout patterns per page (e.g. full-bleed, split text-only page on the left, full image page on the right) to avoid visual monotony in compiled books.

## User Review Required

> [!NOTE]
> This requires introducing new layout schemas and mapping layout choices in the manifest data structure.

## Proposed Changes

### Manifest & Parsing
Modify `manifest.json` schema to support layout types per page.

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add `Layout` string field to `PageState` struct. Mapped to `"layout"`. Default: `"full_bleed"`.

### Typst Template Updates
Update the template in `pw-mcp-typst` (or override it in Pithos) to process layout tags.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Pass layout configurations for each page to the Typst compiler client tools.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/manifest/...` and verify layout serialization.

### Manual Verification
- Compile Pithos, run assembly on a book with split layouts, and check that the compiled PDF displays mixed left-text, right-image spreads.
