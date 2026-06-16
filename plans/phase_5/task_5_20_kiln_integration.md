# plan: Task 5.20: Kiln Foundry State Integration

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-16
**Unit Test Coverage:** 91.3%

Define and implement a stable state contract between the Pithos `manifest.json` and the Kiln Foundry. Pithos must write pipeline milestone markers that Kiln can read to update book status without directly coupling to Pithos internals. This is a prerequisite for Kiln Task 4.2 (Manifest Sync).

## User Review Required

> [!IMPORTANT]
> **This task defines the cross-tool API contract.** Changes to the fields added here are breaking changes for Kiln consumers. Use a `kiln_sync_version` integer field to allow future schema evolution. Version 1 is defined in this task.

> [!IMPORTANT]
> **Pithos owns this schema — Kiln does not.** If Kiln needs additional fields in a future task, the request must go through a Pithos task. Kiln must never directly write to `manifest.json`.

> [!NOTE]
> **What Kiln needs from Pithos:**
> 1. A machine-readable signal when `brew` is complete (all pages generated).
> 2. A machine-readable signal when `assemble` is complete (PDFs compiled).
> 3. The total LLM cost incurred by the run (for Foundry cost tracking).
> 4. The absolute paths to the output PDFs.

## Proposed Changes

### `manifest.json` Schema Extension

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
Add the following fields to the Pithos `Manifest` struct:

```go
// KilnSync contains fields written by Pithos for consumption by Kiln.
// Kiln reads these fields; it never writes to this struct.
type KilnSync struct {
    // Version is the schema version of this block. Current: 1.
    Version int `json:"kiln_sync_version"`
    // Milestones is an ordered list of completed pipeline stages.
    // Valid values: "initiate_complete", "brew_complete", "assemble_complete"
    Milestones []string `json:"kiln_milestones"`
    // TotalCostUSD is the sum of all LLM and image generation costs in this run.
    TotalCostUSD float64 `json:"total_cost_usd"`
    // InteriorPDFPath is the absolute path to the compiled interior PDF.
    // Populated after "assemble_complete".
    InteriorPDFPath string `json:"interior_pdf_path,omitempty"`
    // CoverPDFPath is the absolute path to the compiled cover PDF.
    // Populated after "assemble_complete".
    CoverPDFPath string `json:"cover_pdf_path,omitempty"`
}
```

Embed `KilnSync` in the top-level `Manifest` struct:
```go
type Manifest struct {
    // ... existing fields ...
    Kiln KilnSync `json:"kiln"`
}
```

### Milestone Writing

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
After brew completes successfully, append `"brew_complete"` to `manifest.Kiln.Milestones` and call `manifest.Save()`.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
After assemble completes:
- Set `manifest.Kiln.InteriorPDFPath` and `manifest.Kiln.CoverPDFPath`.
- Append `"assemble_complete"` to `manifest.Kiln.Milestones`.
- Call `manifest.Save()`.

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
On initiate:
- Set `manifest.Kiln.Version = 1`.
- Initialize `manifest.Kiln.Milestones = []`.
- Append `"initiate_complete"` after workspace setup.

### Tests

#### [MODIFY] [manifest_test.go](file://../../internal/manifest/manifest_test.go)
- Verify `KilnSync` serializes and deserializes correctly.
- Verify `Version` is set to `1` on initiate.
- Verify milestones are appended in order.
- Verify `TotalCostUSD` is preserved across Save/Load round-trips.

---

## Verification Plan

### Automated Tests
- `go test ./internal/manifest/...` — schema tests pass
- `go test ./internal/pipeline/...` — pipeline milestone writes verified
- `make check-coverage` — ≥91%

### Manual Verification
1. Run `pithos initiate` → inspect `manifest.json`; verify `kiln.kiln_sync_version: 1` and `kiln.kiln_milestones: ["initiate_complete"]`.
2. Run `pithos brew` → verify `"brew_complete"` added to milestones.
3. Run `pithos assemble` → verify `"assemble_complete"` + PDF paths populated.
4. Run `kiln forge <book-id>` in Kiln (requires Kiln Task 4.2) — verify Kiln reads the milestones and transitions the book to `StatusForged` correctly.
