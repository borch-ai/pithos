# plan: Task 5.24: Graceful Image Backend Fallbacks

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Implement automatic, graceful fallbacks between image generation backends (e.g., falling back from OpenAI DALL-E to Google Imagen, or vice-versa) when the primary backend encounters credential, permission, or model access errors.

## User Review Required

> [!IMPORTANT]
> Fallback triggers only on authentication, credential, or model permission errors (e.g., 400 Bad Request model not found, 401 Unauthorized). It will not trigger on transient network timeouts or validation errors, ensuring fast fail behavior.

---

## Proposed Changes

### 1. Powerword Repository (MCP Server changes)

#### [MODIFY] [main.go](file:///Users/human/code/powerword/cmd/pw-mcp-imagegen/main.go)
- Update `imagegen_generate` MCP tool input schema to accept an optional `backend` string parameter:
  ```json
  "backend": {
      "type": "string",
      "description": "Optional override for the image generation backend (e.g. 'openai', 'google', 'midjourney')"
  }
  ```
- Unmarshal and pass the `backend` argument to `service.GenerateImage`.

#### [MODIFY] [imagegen.go](file:///Users/human/code/powerword/internal/plugins/imagegen/imagegen.go)
- Update the signature of `GenerateImage` to accept the `backend` override:
  ```go
  func (s *ImageGenService) GenerateImage(ctx context.Context, prompt string, size string, styleID string, backendOverride string) (string, string, error)
  ```
- If `backendOverride` is specified, use it in place of the configured default `s.cfg.Plugins.ImageGen.Backend`.

---

### 2. Pithos Repository (CLI client changes)

#### [MODIFY] [brew.go](file:///Users/human/code/pithos/internal/pipeline/brew.go)
- Wrap the `imagegen_generate` MCP tool call in a retry/fallback handler.
- If the call fails:
  - Inspect the error message for patterns indicating authorization, credential, or model eligibility failure (e.g., `"model does not exist"`, `"401"`, `"unauthorized"`, `"api key"`).
  - Check if the alternative backend (e.g. Google Gemini if OpenAI failed, or OpenAI if Google failed) has credentials set in Pithos/Powerword config.
  - Log a console warning: `"Primary imagegen backend failed with credential/model error. Attempting fallback to [Backend]..."`
  - Retry the tool invocation with the `backend` argument set to the fallback provider.

---

## Verification Plan

### Automated Tests
- In `powerword`: Add unit tests verifying that passing the `backend` parameter correctly overrides the configured default backend.
- In `pithos`: Add unit tests mocking `imagegen_generate` calls to return a credential error on the first invocation, verifying that Pithos correctly performs the fallback request to the alternate provider.

### Manual Verification
- Configure a project with a dummy/invalid OpenAI key but a valid Google Gemini key.
- Run `pithos brew` with OpenAI backend selected.
- Verify that Pithos detects the OpenAI credential failure, falls back to Google, generates the images successfully, and writes the fallback warning to the logs.
