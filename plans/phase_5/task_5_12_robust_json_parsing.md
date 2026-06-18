# plan: Task 5.12: Robust JSON Output Parsing

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** June 17, 2026
**Unit Test Coverage:** 91.00%

Implement an LLM response sanitization helper to strip markdown code blocks (e.g. ````json ... ````) from output text, protecting Pithos against JSON parsing errors when providers wrap structured JSON responses in formatting blocks. This helper will protect all structured JSON queries, including visual style guide generation and manuscript generation.

## User Review Required

> [!NOTE]
> This is a clean internal robustness optimization. No breaking changes or configuration additions.

## Proposed Changes

### LLM Integration Layer

#### [MODIFY] [llm.go](file://../../internal/pipeline/llm.go)
- Implement `cleanJSONText(text string) string` helper function:
  * Trim leading/trailing whitespace.
  * Check if text has a leading ``` block. If yes, locate the first newline and strip the opening fence.
  * Check if text has a trailing ``` block, and strip it.
  * Trim resulting whitespace and return clean JSON string.
- Update structured JSON API methods to run their response text through `cleanJSONText` prior to calling `json.Unmarshal`:
  * `GeminiClient.GenerateVisualGuides` / `OpenAIClient.GenerateVisualGuides`
  * `GeminiClient.GenerateStanzas` / `OpenAIClient.GenerateStanzas`

## Verification Plan

### Automated Tests
- Run tests in the `pipeline` package:
  ```bash
  go test -v ./internal/pipeline/...
  ```
- Add unit tests in `llm_test.go` verifying that `cleanJSONText` handles:
  * Normal JSON output (no markdown blocks).
  * Markdown JSON blocks with ` ```json ` header and ` ``` ` footer.
  * Markdown block with trailing whitespace/newlines.
  * Empty input strings.
