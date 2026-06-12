# plan: Task 5.17: LLM-Driven Stanza Refinement & Feedback Loop

**Status:** Open
**Go Version:** 1.26.4

Implement an interactive feedback and refinement mechanism for parodic stanzas. During the review loop (either in the markdown file or the terminal TUI), creators can provide free-text feedback instructions (e.g. *"make this rhyme better"* or *"make the cat sound more mischievous"*) for a specific page. Pithos will query the LLM to rewrite only that page's stanza and illustration prompt based on the feedback while preserving the global book style, character consistency profile, and overall narrative.

## User Review Required

> [!NOTE]
> This is a localized refinement step. The feedback loop only alters the target page's text and illustration prompt, keeping all other pages unchanged. Telemetry usage and costs are recorded normally.

## Proposed Changes

### LLM Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Add `RefineStanza` to `LLMClient` interface:
  ```go
  type LLMClient interface {
      GenerateVisualGuides(ctx context.Context, theme string) (style string, characterProfile string, usage telemetry.TokenUsage, error)
      GenerateStanzas(ctx context.Context, theme string, count int, style string, characterProfile string) ([]string, []string, telemetry.TokenUsage, error)
      
      // RefineStanza regenerates a single stanza and prompt based on user feedback
      RefineStanza(ctx context.Context, theme, style, characterProfile, currentStanza, feedback string) (newStanza, newPrompt string, usage telemetry.TokenUsage, error)
  }
  ```
- Implement `RefineStanza` on `GeminiClient` and `OpenAIClient`:
  * Build a prompt enclosing:
    - The overall book theme, visual style, and character consistency profile.
    - The current stanza text.
    - The user's specific feedback instruction.
  * Require the output to follow a JSON schema structure:
    ```json
    {
        "stanza": "new refined stanza text",
        "illustration_prompt": "new refined illustration prompt"
    }
    ```
  * Sanitize (via `cleanJSONText`) and parse the response.

### Pipeline Core & TUI

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In the Bubbletea TUI loop:
  - Add hotkey `f` (Feedback / Refine) when a page is selected.
  - When pressed, open a text input prompt: *"Enter refinement feedback: "*.
  - On submit:
    - Set the page state to generating.
    - Run `llmClient.RefineStanza` in a background goroutine.
    - On completion, update `Text` and `IllustrationPrompt` on the page state, and automatically trigger selective image regeneration for that page.
    - Save manifest updates.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * `RefineStanza` constructs prompts correctly incorporating theme, guides, and user feedback.
  * TUI keystroke events trigger LLM calls and properly update page text/prompts.

### Manual Verification
1. Launch Pithos in review mode:
   ```bash
   ./bin/pithos brew --output refine-test --review --tui
   ```
2. Select Page 1, press `f`, enter refinement feedback (e.g., *"Make it rhyme with spoon"*).
3. Confirm that the stanza is rewritten, the prompt is updated, and the page's image begins regenerating.
