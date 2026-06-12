# plan: Task 5.10: Visual Prompt Expansion for Character Consistency in Pithos

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-12
**Unit Test Coverage:** 92.10% (exceeds strict 91% coverage requirement)

## Goal

Implement Visual Prompt Expansion on the Pithos orchestrator to achieve character and style consistency across generated images without requiring modifications to the underlying Powerword plugin. We modify the LLM phase to generate both parodic stanzas and detailed, consistent visual illustration prompts referencing a consistent character description. These prompts are stored in the manifest and exposed in the local review loop.

## User Review Required

> [!NOTE]
> None. This is a clean backend implementation of character consistency prompts and review loops.

## Proposed Changes

### Pithos Orchestrator

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Updated `PageState` struct to include a new `IllustrationPrompt` field:
  ```go
  type PageState struct {
      PageIndex          int        `json:"page_index"`
      Status             PageStatus `json:"status"`
      ImagePath          string     `json:"image_path,omitempty"`
      Text               string     `json:"text,omitempty"`
      IllustrationPrompt string     `json:"illustration_prompt,omitempty"`
  }
  ```

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Updated `stanzasResponse` JSON schema struct to capture both stanzas and illustration prompts:
  ```go
  type stanzasResponse struct {
      Stanzas             []string `json:"stanzas"`
      IllustrationPrompts []string `json:"illustration_prompts"`
  }
  ```
- Updated system prompts in `GeminiClient.GenerateStanzas` and `OpenAIClient.GenerateStanzas` to generate:
  1. Parodic children's book stanzas in strict meter.
  2. A consistent character description.
  3. Corresponding arrays of detailed illustration prompts describing action, setting, and character details.
- Validated that `len(IllustrationPrompts) == count` and `len(Stanzas) == count`.

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Updated `generateManuscript` to map returned illustration prompts to the manifest pages.
- Updated `generateIllustrations` loop to pass `IllustrationPrompt` (falling back to `Text` if empty) to the image generator.
- Updated `exportManuscriptToMarkdown` to output both text and prompt subheaders under `# Page N`:
  ```markdown
  # Page 1
  ## Text
  [Stanza text]

  ## Prompt
  [Illustration prompt]
  ```
- Updated `parseManuscriptLines` and `importManuscriptFromMarkdown` to parse the new subheaders. If the manuscript uses the old legacy format (no `## Text` or `## Prompt` subheaders), it falls back gracefully to updating only `Text` and leaves `IllustrationPrompt` unchanged.
- Defined the `ErrReviewPause` sentinel error to signal that the pipeline has paused for manual review.

### Pithos CLI Command Layer

#### [MODIFY] [root.go](file://../../cmd/pithos/root.go)
- Configured `SilenceUsage: true` on `rootCmd` to suppress printing of verbose help text during runtime/pipeline pauses.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Intercepted `ErrReviewPause` in the command's execution loop, printed the review pause instruction message, and returned `nil` so that the program exits cleanly with exit code 0.

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Run linter:
  ```bash
  make lint
  ```
- Check coverage:
  ```bash
  make check-coverage
  ```

### Manual Verification
1. Scaffolded a new test book:
   ```bash
   ./bin/pithos initiate --output consistency-test --theme "A parody of Humpty Dumpty" --pages 3
   ```
2. Generated stanzas and checked the review file:
   ```bash
   ./bin/pithos brew --output consistency-test --review
   ```
3. Verified that `books/consistency-test/manuscript.md` contains the new format with `## Text` and `## Prompt` sections.
4. Modified the prompt in `manuscript.md` and ran `brew` to continue:
   ```bash
   ./bin/pithos brew --output consistency-test
   ```
5. Confirmed that the final manifest.json contains the updated illustration prompts.
