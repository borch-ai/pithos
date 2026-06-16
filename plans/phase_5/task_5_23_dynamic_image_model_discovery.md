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

### 1. Powerword Repository (MCP Server changes)

#### [MODIFY] [imagegen.go](file://../../../powerword/internal/plugins/imagegen/imagegen.go)
- Implement a method `(b *GoogleBackend) DiscoverModels(ctx context.Context) ([]string, error)` that queries Google's model list API endpoint (`GET /v1beta/models`).
- Filter models to find those supporting image/video generation (e.g., containing `imagen` or `veo`).
- Update `GenerateImage` in `GoogleBackend` and `VeoBackend` to:
  - Dynamically query available models if the configured model is empty, invalid, or returns a `404 Not Found`.
  - Fall back to the latest verified/active version (e.g., resolving `imagen-4.0-generate-001` if `imagen-3.0-generate-002` fails).
- Update the return type of `ImageGenService.GenerateImage` to return both the saved local file path and the **actual model name** used.

#### [MODIFY] [main.go](file://../../../powerword/cmd/pw-mcp-imagegen/main.go)
- Register a new MCP tool `imagegen_list_models` which returns the list of dynamically discovered image/video generation models.
- Update the response of `imagegen_generate` tool to include the model used in the output string or structured metadata.

---

### 2. Pithos Repository (CLI client changes)

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Parse the actual model returned by the MCP `imagegen_generate` result.
- Log the model name used for each page generation.
- Store the resolved image model name in the page's metadata / telemetry inside `manifest.json`.

---

## Verification Plan

### Automated Tests
- In `powerword`: Add unit tests in `imagegen_test.go` mocking the Google `/v1beta/models` endpoint. Verify that it correctly filters and chooses the latest available model.
- In `pithos`: Update `brew_test.go` / `pipeline_test.go` to mock the new MCP tool and check that the resolved model name is captured in the manifest.

### Manual Verification
- Run `pithos doctor` (or a test CLI call) and verify that it prints the list of discovered image models.
- Set `POWERWORD_IMAGEGEN_GOOGLE_MODEL=""` or to a dummy value, run `pithos brew`, and verify that it successfully discovers and uses `imagen-4.0-generate-001` automatically.
