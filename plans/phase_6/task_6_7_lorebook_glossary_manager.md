# plan: Task 6.7: Automated Lorebook & Technical Glossary Manager

**Status:** Open
**Go Version:** 1.26.4

Implement a global Lorebook and Technical Glossary manager to prevent visual, narrative, or factual inconsistencies (such as lore-drift in fantasy novels or conflicting acronym definitions in technical books). Terms, names, and concepts will be cataloged in the manifest, injected into the LLM prompt context during text generation, and updated dynamically as new terms are introduced.

## User Review Required

> [!WARNING]
> **Cascading Chapter Redo on Glossary Edits**:
> If a user manually edits a glossary definition or lore book entry during review, Pithos will scan chapter contents, identify which chapters/sections reference that term, and reset **only those chapters** back to `pending`. This avoids full book regenerations while maintaining consistency.

## Proposed Changes

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add `Glossary` field to `BookProperties` struct:
  ```go
  type GlossaryEntry struct {
      Term        string   `json:"term"`
      Definition  string   `json:"definition"`
      Alternative []string `json:"alternative_terms,omitempty"`
  }

  type BookProperties struct {
      ...
      Glossary []GlossaryEntry `json:"glossary,omitempty"`
  }
  ```

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In `generateChapters`:
  - When prompting the LLM to write a chapter/section:
    - Pass the current `m.BookProperties.Glossary` block as a context constraint in the LLM system prompt: *"Adhere strictly to the following terminology and character constraints: [Glossary]"*.
    - Instruct the LLM to output both the written section text and any **new** glossary terms introduced in that section (acronyms, technical definitions, new character names).
  - Collect returned new terms and merge them into the manifest glossary.
- Implement `exportGlossaryToMarkdown(outputDir string, glossary []manifest.GlossaryEntry) error`:
  - Write a `glossary.md` file in the review workspace containing a structured table of terms.
- Implement `importGlossaryFromMarkdown(outputDir string, m *manifest.Manifest) (bool, error)`:
  - Read `glossary.md`, parse updated terms, and compare with manifest glossary.
  - If a term's definition is modified:
    - Identify all chapters/sections containing references to the term (by executing simple regex matches on section texts).
    - Reset the status of matching sections to `pending` and clear their image paths/draft text.
    - Update the glossary entries in the manifest and set `changed = true`.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Glossary extractor parses terms and definitions from LLM outputs.
  * Term modification successfully triggers cascading resets only on affected chapter sections.

### Manual Verification
1. Generate a technical book:
   ```bash
   ./bin/pithos brew --output glossary-test
   ```
2. Open `books/glossary-test/glossary.md` and edit a term definition (e.g. changing acronym *OKR* meaning).
3. Run `brew` and verify that only the chapters using *OKR* are flagged for regeneration.
