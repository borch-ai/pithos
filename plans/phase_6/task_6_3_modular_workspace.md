# plan: Task 6.3: Modular Multi-File Workspace

**Status:** Open
**Go Version:** 1.26.4

Evolve Pithos to manage a structured multi-file book workspace instead of a single monolithic `manuscript.md` file. This is essential for serious writing where files quickly become large and hard to navigate. Pithos will save chapters and sections in individual markdown files (e.g. `chapters/01_introduction.md`, `chapters/02_hiring.md`), stitch them together during compilation, and sync manual edits back to the manifest.

## User Review Required

> [!NOTE]
> **Independent Chapter Modification Rules**:
> Editing a single chapter file (e.g. `chapters/02_hiring.md`) will only trigger regeneration/processing on that specific chapter, leaving all other chapters in the manifest intact.

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Implement `exportChaptersToWorkspace(outputDir string, chapters []manifest.ChapterState) error`:
  - Create a `chapters/` directory in the book's output folder.
  - Write each chapter as a standalone file: `chapters/XX_<slugified_title>.md`.
  - Include metadata front-matter inside each file:
    ```markdown
    ---
    chapter_index: 1
    title: Introduction
    status: completed
    ---
    
    ## Section 1: Executive Transitions
    [Draft text...]
    ```
- Implement `importChaptersFromWorkspace(outputDir string, m *manifest.Manifest) (bool, error)`:
  - Recursively read all files inside `chapters/*.md`.
  - Parse the front-matter metadata and section content.
  - Compare section text with corresponding manifest values.
  - If edits are detected:
    - Update `Text` in `m.BookProperties.Chapters`.
    - Reset the section status to `pending` (triggering clean builds/diagram regenerations).
    - Set `changed = true`.
- Implement a compile step that stitches all markdown chapters into a unified manuscript for publication.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Modular exporter correctly writes slugified filenames.
  * Parser correctly extracts metadata front-matter and section bodies from modular markdown files.
  * Local modifications are synced back to the manifest while leaving other chapters unaffected.

### Manual Verification
1. Scaffold a book workspace:
   ```bash
   ./bin/pithos brew --output serious-test
   ```
2. Confirm the `books/serious-test/chapters/` directory contains files like `01_introduction.md`.
3. Open a chapter file, edit some text, and run `brew` again.
4. Verify the manifest.json registers the edits and logs the file import.
