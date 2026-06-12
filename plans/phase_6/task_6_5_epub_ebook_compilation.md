# plan: Task 6.5: EPUB Ebook Compilation & Formatting

**Status:** Open
**Go Version:** 1.26.4

Implement EPUB 3 eBook export capability to enable digital distribution on Kindle, Apple Books, and other platforms. Pithos will parse modular manuscript drafts into clean XHTML chapters, compile the standard EPUB container structure, map stylesheets, embed cover art, and package everything into a valid EPUB zip container.

## User Review Required

> [!NOTE]
> **Self-Contained Compiler**:
> To avoid external runtime requirements (such as Pandoc or Calibre), we will implement a pure Go EPUB 3 compiler structure using Go's built-in `archive/zip` library.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add structs for EPUB styling configurations:
  ```go
  type EPUBConfig struct {
      Identifier  string `toml:"identifier"` // e.g. ISBN or UUID
      Publisher   string `toml:"publisher"`
      Stylesheet  string `toml:"stylesheet"` // Custom user css path
  }
  ```

---

### CLI Command Layer

#### [NEW] [epub.go](file://../../cmd/pithos/epub.go)
- Add `epub` command to compile the book as an EPUB file:
  ```bash
  pithos epub --output [book_dir]
  ```

---

### Pipeline Core

#### [NEW] [epub.go](file://../../internal/pipeline/epub.go)
- Implement `CompileEPUB(ctx context.Context, outputDir string, m *manifest.Manifest) error`:
  - Read active `BookProperties` and `AuthorProfile` from the manifest.
  - Create standard EPUB 3 structures dynamically:
    * `mimetype` file containing `application/epub+zip` uncompressed.
    * `META-INF/container.xml` mapping the rootfile.
    * `OEBPS/content.opf` containing metadata, manifest (item declarations for cover image, CSS, XHTML chapters), and spine (reading order).
    * `OEBPS/toc.xhtml` for EPUB 3 visual table of contents.
    * `OEBPS/toc.ncx` for legacy EPUB 2 device compatibility.
  - For each chapter in `manifest.json`:
    * Load and parse chapter markdown files.
    * Convert markdown headings, lists, paragraphs, code blocks, and images to strict XHTML formatting.
    * Embed chapter CSS stylesheet mappings.
    * Save as `OEBPS/text/chapter_<index>.xhtml`.
  - Copy and compress files using Go's `archive/zip` writer, keeping the `mimetype` file as the first uncompressed entry.
  - Output completed file to `[book_dir]/book.epub`.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  - XHTML converter correctly sanitizes and formats markdown blocks.
  - EPUB builder correctly generates well-formed XML metadata manifests (`content.opf`, `toc.ncx`).
  - Zip writer successfully packages file structure with uncompressed `mimetype` signature.

### Manual Verification
1. Run epub compilation command:
   ```bash
   ./bin/pithos epub --output serious-test
   ```
2. Verify that `books/serious-test/book.epub` is generated.
3. Validate the ebook file using `epubcheck` to ensure zero structure or XML errors:
   ```bash
   epubcheck books/serious-test/book.epub
   ```
4. Load the EPUB in Apple Books, Thorium Reader, or a Kindle device to confirm readability.
