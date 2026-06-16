# plan: Task 5.11: LLM-Driven Style & Character Seeds

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-16
**Unit Test Coverage:** 91.5%

Implement a two-level automated visual and character-consistency seeding pipeline:
1. **Level 1: Visual Book Style (Global)**: Defines the overall art medium, rendering style, lighting, and palette (e.g., claymation, 3D, sketch). This is registered with the image generator as a style reference.
2. **Level 2: Character Consistency Profile (Global)**: Defines the persistent visual properties of the main character(s) (e.g., clothing colors, physical features, accessories). This is fed into the LLM during stanza/prompt generation to ensure the character's details are woven into all page illustration prompts.

Before generating stanzas, the LLM will generate both the Level 1 Book Style and Level 2 Character Profile based on the parodic theme. Pithos will store these in the manifest, pass them to the stanza generation step, expose them for editing at the top of the review markdown file, and apply page reset rules on modifications.

## User Review Required

> [!WARNING]
> **Page Reset on Modification**:
> If the user edits either the global book style comment (`<!-- Style: ... -->`) or the character profile comment (`<!-- CharacterProfile: ... -->`) in `manuscript.md` during review, Pithos will import the edits, reset the status of **all** pages back to `pending`, and clear all image paths. This ensures the entire book is regenerated consistently.

## Proposed Changes

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Add `CharacterProfile` field to `BookProperties`:
  ```go
  type BookProperties struct {
      Theme            string `json:"theme"`
      Style            string `json:"style"` // Level 1 Style Guide
      CharacterProfile string `json:"character_profile"` // Level 2 Character Guide
      Format           string `json:"format"`
      TargetPageCount  int    `json:"target_page_count"`
  }
  ```

### LLM Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Update `LLMClient` interface:
  ```go
  type LLMClient interface {
      GenerateVisualGuides(ctx context.Context, theme string) (style string, characterProfile string, usage telemetry.TokenUsage, error)
      GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error)
  }
  ```
- Modify `GenerateStanzas` in `GeminiClient` and `OpenAIClient`:
  * Accept `style` and `characterProfile` string inputs.
  * Update prompt structure to inject these guides as strict constraints for stanza and prompt drafting.
- Implement `GenerateVisualGuides`:
  * Prompt the LLM to return a JSON containing two keys:
    * `style_seed`: General art style medium/lighting guide.
    * `character_profile`: Description of main characters/apparel.
  * Use the robust parsing helper from Task 5.12 to sanitize and unmarshal the JSON response.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In `generateManuscript`:
  - If `m.BookProperties.Style` or `m.BookProperties.CharacterProfile` is empty (and no CLI flag `--style` was provided):
    - Call `llmClient.GenerateVisualGuides(ctx, theme)` to generate visual styles (Level 1) and character profiles (Level 2).
    - Save these to the manifest.
  - Call `llmClient.GenerateStanzas(ctx, theme, pageCount, m.BookProperties.Style, m.BookProperties.CharacterProfile)` to generate the manuscript stanzas and page-level illustration prompts.
- In `generateIllustrations`:
  - Pass the Level 1 `Style` to the MCP plugin's `imagegen_register_style` tool.
  - Page-specific illustration prompts (incorporating Level 2 character details) are generated and sent to `imagegen_generate` using the registered style ID.
- Update `exportManuscriptToMarkdown` to output both guides in comments at the top of the file:
  ```markdown
  <!-- PITHOS MANUSCRIPT REVIEW -->
  <!-- Style: claymation style, vibrant cinematic lighting, shallow depth of field -->
  <!-- CharacterProfile: A chubby orange cat wearing green square-rimmed reading glasses and a small blue collar -->
  <!-- Edit the stanzas, prompts, and global style/character guides above. -->
  ```
- Update `parseManuscriptLines` to parse both `<!-- Style: ... -->` and `<!-- CharacterProfile: ... -->` comment headers.
- Update `importManuscriptFromMarkdown`:
  - If the parsed style or character profile differs from the manifest's current values, update them in the manifest, reset all pages to pending, and clear all image paths.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * `GenerateVisualGuides` returns correctly parsed Level 1 and Level 2 guides from mocked LLM JSON responses.
  * `GenerateStanzas` includes both visual guides in the LLM instruction prompts.
  * Modifying either the Style or CharacterProfile comment resets all page progress and clears image paths on import.

### Manual Verification
1. **Initialize a New Project**:
   ```bash
   go run ./cmd/pithos initiate --output book_test --theme "existential dread cat" --pages 3
   ```
   Check that `book_test/manifest.json` is created, and the `character_profile` property under `book_properties` is empty.

2. **Generate the Manuscript and Visual Guides**:
   ```bash
   go run ./cmd/pithos brew --output book_test --review
   ```
   Verify that:
   - The console logs indicate manuscript generation.
   - `book_test/manifest.json` contains newly generated values for both `style` and `character_profile`.
   - `book_test/manuscript.md` is exported, and starts with the comments:
     ```markdown
     <!-- PITHOS MANUSCRIPT REVIEW -->
     <!-- Style: <generated style> -->
     <!-- CharacterProfile: <generated profile> -->
     ```
   - Each page illustration prompt under `## Prompt` incorporates the character and style details.

3. **Verify Guide Modification Page Resets**:
   - Manually edit the `<!-- Style: ... -->` or `<!-- CharacterProfile: ... -->` comment in `book_test/manuscript.md`.
   - Run the import/brew loop again:
     ```bash
     go run ./cmd/pithos brew --output book_test --review
     ```
   - Verify that:
     - The changes are successfully imported.
     - `book_test/manifest.json` is updated with the modified style/character profile.
     - The status of all pages is reset back to `pending`, and all page image paths are cleared.
