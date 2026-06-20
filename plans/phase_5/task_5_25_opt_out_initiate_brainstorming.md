# plan: Task 5.25: Opt-out Brainstorming during Initiate

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-20
**Unit Test Coverage:** 91.5%

## Goal Description

Add an automated style and character brainstorming step by default to `pithos initiate` when a theme is provided, with a `--no-brainstorm` flag to opt out.
If the theme is provided, the command will initialize the LLM client (reusing the unified powerword LLM client adapter configuration) and query the LLM to generate the initial `style_seed` and `character_profile` immediately. The generated fields will be written directly into the newly created `manifest.json`, giving the user a ready-to-inspect draft immediately.
If no theme is provided, scaffolding completes normally without brainstorming.
If a theme is provided but API keys are missing/invalid, a helpful error is returned suggesting to configure API keys or run with `--no-brainstorm`.

## User Review Required

> [!NOTE]
> **API Key Requirements**: If the user provides a theme during initiate and does not pass `--no-brainstorm`, the command will fail if the required API keys (Gemini or OpenAI) are not configured.

## Proposed Changes

### Command Layer

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Added `initiateNoBrainstorm` boolean CLI flag bound to `--no-brainstorm`.
- Passed `NoBrainstorm` and `Context` (propagating `cmd.Context()`) in `pipeline.InitiateOptions`.

### Pipeline Layer

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Extracted client setup logic into `getLLMClient(llmOverride LLMClient, httpClient *http.Client) (LLMClient, error)`.
- Simplified `generateAndRecordVisualGuides` to remove the unused `BrewOptions` parameter.

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
- Updated `InitiateOptions` struct to include `NoBrainstorm`, `LLM`, `HTTPClient`, and `Context`.
- Implemented `brainstormVisualGuides` helper function called during `Initiate` to construct the LLM client, run visual guide generation, and record properties in the manifest before saving.
- Suppressed brainstorming in unit test environments where `opts.LLM == nil` and API keys are missing.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make check-coverage
  ```
- Added tests to `pipeline_test.go`:
  - `TestInitiate_Brainstorm_Enabled`
  - `TestInitiate_Brainstorm_Disabled`
  - `TestInitiate_Brainstorm_NoTheme`
  - `TestInitiate_Brainstorm_NoAPIKeys`
- Updated integration tests in `integration_test.go` to explicitly pass `NoBrainstorm: true`.

### Manual Verification
1. Run `./bin/pithos initiate --theme "A pessimist turtle" --output books/turtle_book` and verify that the manifest contains generated `style_seed` and `character_profile` fields.
2. Run `./bin/pithos initiate --theme "A pessimist turtle" --output books/turtle_book_no_brain --no-brainstorm` and verify that the manifest contains empty `style_seed` and `character_profile` fields.
