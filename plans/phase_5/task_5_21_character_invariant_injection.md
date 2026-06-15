# plan: Task 5.21: Character Invariant Injection

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Inject a consistent character description invariant into the prompts of all pages during image generation to ensure Red Riding Hood and the Wolf have high visual character consistency across all stanzas.

## User Review Required

> [!NOTE]
> Character descriptors are injected on the client-side (Pithos) and do not require image model changes.

## Proposed Changes

### Brew Engine
During `brew`, parse or retrieve character profiles, and append/prepend the description to each page's image generation prompt.

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Retrieve character description properties (e.g. from a shared prompt context block).
- Concatenate the visual descriptor with the page-specific illustration prompt before executing `imagegen_generate` tool.

---

## Verification Plan

### Automated Tests
- Add unit tests in `internal/pipeline/pipeline_test.go` verifying that visual invariants are correctly concatenated into the prompt arguments sent to the MCP image generation client.

### Manual Verification
- Run `pithos brew` on a new book niche, and verify generated prompts in `manifest.json` have consistent character description prefixes.
