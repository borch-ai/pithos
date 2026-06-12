# plan: Task 6.11: Bibliography, Citations & References Manager

**Status:** Open
**Go Version:** 1.26.4

Implement support for standard BibTeX bibliography files (`references.bib`) to enable citation management for technical writing, non-fiction, and academic guides. The system will parse available reference keys, feed them into the LLM context during drafting to enforce precise in-text citations, and compile a formatted bibliography section at the end of the typeset book.

## User Review Required

> [!NOTE]
> **Typst Native Citation System**:
> Typst has robust native support for BibTeX (`#bibliography("references.bib")`). Pithos's task is only to manage ingestion, supply citation targets to the LLM, and pass reference paths to the Typst compiler.

## Proposed Changes

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add references configuration to `BookProperties`:
  ```go
  type BookProperties struct {
      ...
      ReferencesPath string `json:"references_path,omitempty"` // Path to references.bib
  }
  ```

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Implement `loadBibTeXKeys(bibPath string) ([]string, error)`:
  - If `references.bib` exists in the workspace, scan the file for entry keys (e.g. `@book{larson2019staff, ...}` or `@article{topologies, ...}`).
  - Extract and return a slice of available citation keys.
- In `generateChapters`:
  - Load the keys using `loadBibTeXKeys`.
  - If keys are found, append them to the LLM generation prompt:
    * Prompt: *"When referencing external research or frameworks, use these BibTeX keys as citations using standard Typst notation (e.g., '@larson2019staff' or '@topologies'): [keys list]."*
  - The LLM will include these citation marks in the written section markdown text.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- In Typst PDF compilation:
  - Check if `references_path` is configured in `BookProperties`.
  - If yes, append the bibliography declaration to the end of the generated `document.typ` file:
    ```typst
    #bibliography("references.bib", style: "association-for-computing-machinery")
    ```

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * BibTeX parser successfully extracts citation keys from mock files.
  * Bibliography directives are correctly appended to Typst assembly templates.

### Manual Verification
1. Place a sample `references.bib` in your workspace.
2. Run `brew` to write a chapter and verify the LLM inserts the `@key` citations correctly in the generated text.
3. Run `assemble` and verify the output PDF includes a styled bibliography index at the end of the book.
