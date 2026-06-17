# plan: Task 5.19: Interactive Web-Based Book Preview (HTML/CSS)

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** June 17, 2026
**Unit Test Coverage:** 91.00%

Implement an automated static HTML/CSS/JS web preview builder. When Pithos completes a brew run or review export, it will generate a self-contained web preview directory containing an interactive 3D-style book flip visualizer. This enables creators to open `preview.html` in any browser to review layout structures, stanza lengths, and image alignments interactively.

## User Review Required

> [!NOTE]
> **Zero Local Server Overhead**:
> The generated preview consists of entirely static assets (HTML, CSS, JS, and relative image references) and loads natively via the `file://` protocol in any standard web browser, requiring no local web server daemon.

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Automatically trigger `GenerateWebPreview` at the end of the manuscript review checkpoint (`handleReviewCheckpoint`) and the final completion step of the `Brew` function.
- Print a clear console help message on successful generation with the path to the HTML preview.
- Dynamic image size helper `getBestImageSize` resolves target `TrimSize` to the best matching `imagegen` aspect ratio (`1024x1024`, `1024x1792`, `1792x1024`) when calling Powerword.

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go) (internal & cmd)
- Add a new `--trim-size` CLI flag (default `"8.5x8.5"`) to scaffold the book's trim size into the manifest.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Fallback and override mapping to resolve and save `TrimSize` to `BookProperties` in the manifest.

### Web Templates

#### [MODIFY] [preview.go](file://../../internal/pipeline/preview.go)
- **`previewData`**: Added `trimSize`, `format`, and `kdpLayout` serialization to `data.js`.
- **`preview.html`**: Added `Show Print Guides` toggle button and sidebar fields for trim size and format.
- **`preview.css`**: Removed fixed dimensions, added paper texture dot grid background, added alternating classes `.page.recto` (crease left, padding left) and `.page.verso` (crease right, padding right), and print guidelines overlay styling.
- **`preview.js`**: Dynamic page container scaling based on trim size aspect ratio, alternating page layout assignment, and print safety guidelines drawing (red cut line, blue safe zone).

### Future Roadmap Extensions

#### [COMPLETED] Dynamic Geometry Parameter Binding (Task 5.19.3)
- Refactor print safety guide drawing in `preview.js` to draw lines dynamically based on precise dimensions calculated by `pw-mcp-kdp-math` and stored inside `manifest.json`, rather than estimating safety margins purely client-side.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  go test -v ./internal/pipeline/...
  ```
- Created/updated unit tests:
  * `TestGenerateWebPreview` in `preview_test.go` verifies HTML, CSS, JS, and data.js assets are correctly generated and data matches the mock manifest including the new metadata.
  * `TestGetBestImageSize` in `pipeline_test.go` verifies all width/height aspect ratio mapping branches.
  * `TestAssemble_TrimSizeDefaulting` and `TestAssemble_TrimSizeOverride` in `assemble_test.go` verify fallback/override behaviors.
  * `TestInitiate` verifies defaulted and custom trim size scaffolding.

### Manual Verification
1. Run the brew command:
   ```bash
   ./bin/pithos initiate --theme "The Existential Robot" --pages 3
   ./bin/pithos brew --review
   ```
2. Open `books/book/web_preview/preview.html` in Safari, Chrome, or Firefox.
3. Confirm you can flip through the pages using arrow keys and that stanzas and generated illustrations are presented correctly.
