# plan: Task 8.4: Typst Professional Book Compilation & Templates

**Status:** Open
**Go Version:** 1.26.4

Integrate professional Typst typesetting layouts and templates to compile technical manuals, self-help books, and fiction novels into publication-ready PDFs. Instead of children's picture book grids, this adds sophisticated layout templates including tables of contents, running headers/footers, section numbering, proper page breaks, indexes, and bibliography integration.

## User Review Required

> [!NOTE]
> **Typst Compiler Dependency**:
> Building PDFs requires the Typst CLI binary installed locally. Pithos will check if `typst` is available on the path or configuration, falling back to basic PDF compilation via Powerword's Typst MCP server if available.

## Proposed Changes

### Pipeline Core

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Refactor `Assemble` execution loop to check `m.BookProperties.Genre` (loaded via the Author Profile in Task 6.1).
- If a serious genre (e.g. `technical`, `non-fiction`, `novel`) is configured:
  - Generate a Typst layout markup file (`document.typ`) rather than simple images compilation.
  - Dynamically load a Typst template matched to the genre:
    * `templates/technical.typ`: Implements double-sided layouts, structured titles, section numbers, margins, and inline code formatting.
    * `templates/novel.typ`: Implements compact sizes, serif fonts, centered chapter titles, and drop caps.
  - Stitch the front-matter (TOC, title page, copyright), chapter sections, bibliography files, and diagrams.
  - Compile the Typst markup:
    - Invoke the local `typst compile document.typ book.pdf` shell command.
    - Save the completed PDF to the output directory.

### Templates Library

#### [NEW] [templates.go](file://../../internal/placeholder/templates.go) (or embed in pipeline assets)
- Define standard Typst base templates in Go string parameters:
  - `TechnicalTemplate`: Standard academic/technical formatting using sans-serif fonts, syntax-highlighted code blocks, and dynamic page numbering.
  - `NovelTemplate`: Standard trade paperback format using serif typography (e.g., Garamond/Linux Libertine), page margins, and classic header designs.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Typst markup builder correctly escapes text characters and structures headers.
  * Correct template is loaded according to the manifest genre.

### Manual Verification
1. Run compilation on a serious workspace:
   ```bash
   ./bin/pithos assemble --output serious-test
   ```
2. Verify `books/serious-test/book.pdf` is generated.
3. Open the PDF and confirm that it contains a Table of Contents, formatted chapters, and running footers.
