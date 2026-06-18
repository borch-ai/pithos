# plan: Task 4.4: Mixed Layout Templates
 
**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-18
**Unit Test Coverage:** 91.40%

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

1. **Build the CLI**:
   ```bash
   make build
   ```
2. **Initiate a Test Book**:
   Scaffold a test workspace directory for a new book:
   ```bash
   ./bin/pithos initiate --theme "The Cybernetic Diogenes" --output books/cyber_diogenes --pages 4
   ```
3. **Generate Manuscript & Pause for Review**:
   Run the initial brew phase to generate the manuscript stanzas and prompt seed, pausing for review:
   ```bash
   ./bin/pithos brew --output books/cyber_diogenes --review
   ```
4. **Edit the Manuscript for Mixed Layouts**:
   Open the exported `books/cyber_diogenes/manuscript.md` file. Add the layout comment tag right underneath the page headers to test different layout styles:
   - For Page 1, leave/set it to:
     ```markdown
     # Page 1
     <!-- Layout: full-bleed -->
     ```
   - For Page 2, set it to:
     ```markdown
     # Page 2
     <!-- Layout: facing-pages -->
     ```
   - For Page 3, set it to:
     ```markdown
     # Page 3
     <!-- Layout: facing-pages-flipped -->
     ```
5. **Import Edits back into Manifest**:
   Run the brew command without the `--review` flag to import the updated stanzas and layouts into the manifest (this will also generate page illustrations):
   ```bash
   ./bin/pithos brew --output books/cyber_diogenes
   ```
6. **Verify Manifest State**:
   Open `books/cyber_diogenes/manifest.json` and verify that the `layout` field for each page has been parsed and stored correctly:
   - Page 1: `"layout": "full-bleed"`
   - Page 2: `"layout": "facing-pages"`
   - Page 3: `"layout": "facing-pages-flipped"`
7. **Assemble the Print Book**:
   Run the assembly pipeline to calculate exact layout dimensions and compile the interior PDF via the Typst plugin:
   ```bash
   ./bin/pithos assemble --input books/cyber_diogenes
   ```
8. **Verify Web Book Previewer**:
   Open `books/cyber_diogenes/web_preview/preview.html` in your browser. Open the browser developer console or view the generated `web_preview/data.js` and verify that each page object inside the `pages` array carries the correct `layout` property.
9. **Verify Compiled PDF Assembly (Once Powerword Task 3.21 is Shipped)**:
   Open the compiled `books/cyber_diogenes/interior.pdf` file in a PDF reader and verify:
   - Page 1 renders as a full-bleed illustration with text overlayed at the bottom.
   - Page 2 renders a text-only page (white background, black text) followed by a full-page illustration.
   - Page 3 renders a full-page illustration followed by a text-only page.
