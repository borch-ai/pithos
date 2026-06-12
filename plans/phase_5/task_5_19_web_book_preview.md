# plan: Task 5.19: Interactive Web-Based Book Preview (HTML/CSS)

**Status:** Open
**Go Version:** 1.26.4

Implement an automated static HTML/CSS/JS web preview builder. When Pithos completes a brew run or review export, it will generate a self-contained web preview directory containing an interactive 3D-style book flip visualizer. This enables creators to open `preview.html` in any browser to review layout structures, stanza lengths, and image alignments interactively.

## User Review Required

> [!NOTE]
> **Zero Local Server Overhead**:
> The generated preview consists of entirely static assets (HTML, CSS, JS, and relative image references) and loads natively via the `file://` protocol in any standard web browser, requiring no local web server daemon.

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Add a pipeline step `generateWebPreview(outputDir string, m *manifest.Manifest) error` called upon completion of `Brew` or during review checkpoints.
- Under `generateWebPreview`:
  - Create the `web_preview/` directory inside the book's output folder.
  - Write `preview.html`, `preview.css`, and `preview.js` containing the player template.
  - Serialize the manifest's page list into a Javascript data script (`data.js` containing `const bookData = {...}`) containing stanza texts and relative image URLs.
  - Provide a CLI flag or log message with instructions: *"Web preview generated. Open file:///path/to/web_preview/preview.html to flip through your book!"*

### Web Templates

#### [NEW] [preview_templates.go](file://../../internal/placeholder/preview_templates.go) (or embedding in pipeline package)
- Create template content strings for:
  - `preview.html`: Basic page layout.
  - `preview.css`: Sleek dark mode styling with custom 3D page flip transition animations.
  - `preview.js`: Event listeners to handle arrow key navigation, button clicks, and page transitions.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Preview assets (`preview.html`, `preview.css`, `preview.js`, `data.js`) are correctly generated in the output directory.
  * `data.js` contains the correct page text and image paths matching the manifest.

### Manual Verification
1. Run the brew command:
   ```bash
   ./bin/pithos brew --output web-preview-test
   ```
2. Open `books/web-preview-test/web_preview/preview.html` in Safari, Chrome, or Firefox.
3. Confirm you can flip through the pages using arrow keys and that stanzas and generated illustrations are presented correctly.
