# plan: Task 1.3: Project State & Resumability Manifest

**Status:** Open (Issue #[TBD])

Define the structured schema of `manifest.json` (or `state.json`) which serves as the source-of-truth and checkpoint logger for long-running book generation runs. Create helper routines for loading, writing, validating, and resuming states.

## Proposed Changes

### State Management Subsystem

#### [NEW] [manifest.go](file:///Users/human/code/pithos/internal/manifest/manifest.go)
- [ ] Define Go structures mirroring the `manifest.json` file structure:
  - [ ] **Book Properties:** theme, style, format, target page count.
  - [ ] **Generation Progress Checkpoints:** manuscript text generated, cover image generated, page-by-page progress status (`pending`, `generating_images`, `awaiting_approval`, `completed`).
  - [ ] **Asset Registry:** maps page numbers/stanzas to local file paths (e.g. `/images/page_1.png`).
  - [ ] **KDP Layout Details:** spine width, margin sizes, bleed calculations.
- [ ] Implement `LoadManifest(path string) (*Manifest, error)`
- [ ] Implement `SaveManifest(path string, m *Manifest) error`
- [ ] Implement checkpoint helper functions (e.g., `UpdatePageStatus(pageIndex int, status string, imagePath string)`) to write state immediately.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/manifest/...`
- [ ] Test JSON marshaling/unmarshaling logic.
- [ ] Test the file loading and safety of atomic saves (saving to a temp file first, then renaming to prevent corruption).

### Manual Verification
- [ ] Verify that a mock pipeline run successfully updates progress in the manifest after each simulated page generation task.
