# plan: Task 6.8: Chapter Takeaways & Review Exercises Generator

**Status:** Open
**Go Version:** 1.26.4

Implement an automated sub-phase in the brew engine that digests finalized chapter drafts to draft a standard, professional "Summary & Exercises" section. The generator will extract key takeaways, frame learning objectives, and compile self-assessment questions tailored to the chapter's content.

## User Review Required

> [!NOTE]
> **Author Style Integration**:
> The questions and summary layout will automatically adapt to the loaded `AuthorProfile` properties (Task 6.1), matching the tone and formatting style of the active writing persona.

## Proposed Changes

### LLM Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Add `GenerateChapterExercises` to `LLMClient` interface:
  ```go
  type LLMClient interface {
      ...
      // GenerateChapterExercises creates structured summary points and review questions
      GenerateChapterExercises(ctx context.Context, authorName, genre, chapterTitle, chapterContent string) (summary []string, exercises []string, usage telemetry.TokenUsage, error)
  }
  ```
- Implement `GenerateChapterExercises` in `GeminiClient` and `OpenAIClient` using structured JSON outputs matching the schema keys `summary` and `exercises`.

---

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Extend the `Brew` pipeline stages to include a takeaways generation loop:
  - Add `generateTakeaways(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error`:
    - Iterate over completed chapters in the manifest.
    - Check if the takeaways/exercises section has already been compiled (to support incremental resumability).
    - If not, invoke `llmClient.GenerateChapterExercises` passing the loaded author details, chapter title, and the full text content of the chapter's sections.
    - Parse the returned JSON response.
    - format and append the generated summary bullets and exercises to the end of the chapter's draft (e.g. under `### Key Takeaways` and `### Self-Assessment Exercises` headers).
    - Record token usage in manifest telemetry.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  - `GenerateChapterExercises` returns correctly structured summary bullets and review questions from mock LLM responses.
  - The takeaways generation step executes successfully and formats the markdown headers correctly when appending to draft files.

### Manual Verification
1. Run a test book build:
   ```bash
   ./bin/pithos brew --output serious-test
   ```
2. Verify that the end of each generated chapter file contains structured headings like `### Key Takeaways` and `### Self-Assessment Exercises` populated with relevant items.
