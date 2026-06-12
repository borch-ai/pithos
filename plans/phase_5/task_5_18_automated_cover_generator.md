# plan: Task 5.18: Automated Cover Art & Title Layout Generator

**Status:** Open
**Go Version:** 1.26.4

Automate parodic book cover generation by creating a unified workflow that queries the LLM for cover art ideas, generates a high-resolution cover illustration matching the style guide, compiles KDP geometry specifications (margins, bleed, spine width), and produces a print-ready PDF cover wrap.

## User Review Required

> [!WARNING]
> **Spine Text Minimum Page Constraints**:
> KDP guidelines require a minimum of 79 pages for spine text to be printable. Pithos cover layout math will validate this check and omit/warn if target page count is insufficient for spine text.

## Proposed Changes

### LLM Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Add `GenerateCoverDesign` to `LLMClient` interface:
  ```go
  type LLMClient interface {
      GenerateVisualGuides(ctx context.Context, theme string) (style string, characterProfile string, usage telemetry.TokenUsage, error)
      GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error)
      RefineStanza(ctx context.Context, theme, style, characterProfile, currentStanza, feedback string) (newStanza, newPrompt string, usage telemetry.TokenUsage, error)
      
      // GenerateCoverDesign generates parodic title, author, back cover blurb, and cover art illustration prompt
      GenerateCoverDesign(ctx context.Context, theme, style, characterProfile string) (title, author, blurb, coverPrompt string, usage telemetry.TokenUsage, error)
  }
  ```
- Implement `GenerateCoverDesign` in `GeminiClient` and `OpenAIClient`, requesting structured JSON matching the keys `title`, `author`, `back_cover_blurb`, and `cover_prompt`.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Extend the `Brew` pipeline execution stages:
  - Add a cover generation phase: `generateCover(ctx, m, opts)`.
  - Under `generateCover`:
    - Call `llmClient.GenerateCoverDesign(ctx, m.BookProperties.Theme, m.BookProperties.Style, m.BookProperties.CharacterProfile)`.
    - Record token usage in manifest telemetry.
    - Save title, author, and back cover blurb to the manifest properties.
    - Query `imagegen_generate` using the returned `cover_prompt` and registered book style reference ID.
    - Copy output file to `images/cover.png` and update `m.Progress.CoverImagePath` and `m.Progress.CoverImageGenerated = true`.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Update cover assembly logic:
  - Calculate exact page count and fetch KDP geometries (spine thickness) from `pw-mcp-kdp-math`.
  - Construct layout instructions combining front cover art (`images/cover.png`), spine geometry, title texts, and back cover text blurb.
  - Compile the layout instructions into a print-ready full cover PDF wrapper (e.g., using `pw-mcp-typst`).

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * `GenerateCoverDesign` correctly returns parsed cover attributes from LLM mock responses.
  * Cover generation step is executed, image is downloaded, and manifest properties are populated correctly.
  * Geometries are correctly integrated during layout calculations.

### Manual Verification
1. Run Pithos brew command and check outputs:
   ```bash
   ./bin/pithos brew --output cover-gen-test
   ```
2. Verify `books/cover-gen-test/images/cover.png` is generated.
3. Run `assemble` and verify the output contains a completed cover layout instruction map.
