# plan: Task 5.25: Opt-in Brainstorming during Initiate

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

## Goal Description

Add an optional `--brainstorm` boolean flag to the `pithos initiate` command. By default, `pithos initiate` simply creates a directory structure and an empty manifest without network connections or LLM requests. However, when the `--brainstorm` flag is specified, the command will initialize the LLM client (using the credentials configured in `.pithos.toml` or environment variables) and query the LLM to generate the initial `style_seed` and `character_profile` immediately based on the provided `--theme`. The generated fields will be written directly into the newly created `manifest.json`, giving the user a ready-to-inspect draft immediately.

## User Review Required

> [!IMPORTANT]
> **API Key Requirements**: If the user provides `--brainstorm` during `initiate`, the command will fail if the required API keys (Gemini or OpenAI) are not configured. The CLI should output a helpful error message guiding the user to configure them.

## Proposed Changes

### Command Layer

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Add a new boolean flag variable `initiateBrainstorm` bound to `--brainstorm`.
- Pass this flag value inside the `pipeline.InitiateOptions` struct to `pipeline.Initiate`.

### Pipeline Layer

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
- Add `Brainstorm bool` to the `InitiateOptions` struct.
- In `Initiate(opts InitiateOptions)`:
  * If `opts.Brainstorm` is `true`:
    1. Validate that `opts.Theme` is not empty. If it is, return an error.
    2. Set up the LLM client by calling `setupLLMClient(BrewOptions{...})`.
    3. Generate the visual style and character profile by calling `GenerateVisualGuides(ctx, theme)`.
    4. Save the generated `style` and `character_profile` in the new manifest.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline/...`
- Add `TestInitiate_Brainstorm` to `pipeline_test.go` to verify:
  * With `Brainstorm: true`, the LLM client is initialized, `GenerateVisualGuides` is invoked, and the generated values are successfully saved to `manifest.json`.
  - Ensure total coverage remains $\ge 91\%$.

### Manual Verification
1. Run `./bin/pithos initiate --theme "A lonely robot on Mars" --output books/robot_book --brainstorm`
2. Open `books/robot_book/manifest.json` and verify that the `style_seed` and `character_profile` fields are populated with descriptive text.
