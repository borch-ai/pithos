# plan: Task 5.23: Dynamic Image Model Discovery

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Implement dynamic model discovery and capability resolution for the MCP image generation plugin to prevent hard failures when upstream model names are deprecated or updated by providers (e.g., resolving the Google Imagen/Veo version mismatch).

## User Review Required

> [!NOTE]
> This plan involves cross-repository cooperation. Changes will be made to `pw-mcp-imagegen` (Powerword) to expose capabilities, and to `pithos` to consume and report them.

---

## Proposed Changes

### External Dependencies (Powerword MCP Plugin)
- Update the `pw-mcp-imagegen` server to dynamically query active models from Google AI Studio REST endpoints.
- Register a new MCP tool `imagegen_list_models` and update `imagegen_generate` to return the name of the resolved model.
- *Note: Code changes within the Powerword repository will be tracked under its own separate implementation plan.*

### Pithos CLI

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Parse the actual model returned by the MCP `imagegen_generate` result.
- Log the model name used for each page generation.
- Store the resolved image model name in the page's metadata / telemetry inside `manifest.json`.

## Verification Plan

### Automated Tests
- In `powerword`: Add unit tests in `imagegen_test.go` mocking the Google `/v1beta/models` endpoint. Verify that it correctly filters and chooses the latest available model.
- In `pithos`: Update `brew_test.go` / `pipeline_test.go` to mock the new MCP tool and check that the resolved model name is captured in the manifest.

### Manual Verification
- Run `pithos doctor` (or a test CLI call) and verify that it prints the list of discovered image models.
- Set `POWERWORD_IMAGEGEN_GOOGLE_MODEL=""` or to a dummy value, run `pithos brew`, and verify that it successfully discovers and uses `imagen-4.0-generate-001` automatically.
