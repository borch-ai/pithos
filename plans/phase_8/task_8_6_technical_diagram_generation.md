# plan: Task 8.6: Technical Diagram & Schematic Generation

**Status:** Open
**Go Version:** 1.26.4

Integrate vector diagram and schematic generation to support technical and business books (e.g. engineering executive guides). The LLM will identify when a diagram is needed, write standard markup diagrams (Mermaid, Graphviz, or D2) in markdown text blocks, and Pithos will parse, compile, and embed these as high-resolution images in the final book.

## User Review Required

> [!NOTE]
> **Mermaid/Graphviz Rendering Tools**:
> Compiling diagrams requires a local rendering tool (such as `mermaid-cli` or Graphviz) or access to a diagram generator MCP server. If unavailable, Pithos will keep the raw text diagrams in the manuscript, which Typst can natively render or style directly.

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In the `brew` text generation flow:
  - Configure the LLM prompts for technical/serious genres to incorporate diagrams (like flowcharts, org charts, architecture diagrams) represented as standard code blocks:
    ```mermaid
    graph TD
       A[Engineering Executive] --> B[Director of Eng]
       A --> C[Product Leader]
    ```
- Implement `renderDiagrams(outputDir string, m *manifest.Manifest) error`:
  - Scan all chapter and section texts in the manifest/workspace for diagram blocks:
    * ` ```mermaid ... ``` `
    * ` ```graphviz ... ``` `
  - For each detected diagram block:
    - Compute a stable hash of the diagram code block.
    - Check if the rendered diagram image `images/diagram_<hash>.png` already exists.
    - If it does not exist:
      - Call the Powerword diagram rendering MCP tool or invoke the local CLI tool (e.g. `mmdc -i diagram.mmd -o diagram.png`).
      - Save the rendered file in `images/diagram_<hash>.png`.
    - Update the manuscript layout code to link to the compiled diagram image path.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Diagram parser correctly extracts Mermaid/Graphviz code blocks from section text.
  * Correct hash is calculated and diagram compilation checks skip rendering if file exists.

### Manual Verification
1. Generate a chapter containing a Mermaid flowchart:
   ```bash
   ./bin/pithos brew --output diagram-test
   ```
2. Confirm the `books/diagram-test/images/` directory contains rendered diagram PNG files.
3. Open the output Typst PDF to check that flowcharts are embedded.
