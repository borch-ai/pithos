# plan: Task 5.24: Graceful Image Backend Fallbacks

**Status:** Completed
**Go Version:** 1.26.5
**Date Completed:** 2026-07-21
**Unit Test Coverage:** 91.0%

Implement automatic, graceful fallbacks between image generation backends (e.g., falling back from OpenAI DALL-E to Google Imagen, or vice-versa) when the primary backend encounters credential, permission, or model access errors.

## User Review Required

> [!IMPORTANT]
> Fallback triggers only on authentication, credential, or model permission errors (e.g., 400 Bad Request model not found, 401 Unauthorized). It will not trigger on transient network timeouts or validation errors, ensuring fast fail behavior.

---

## Proposed Changes

### Pithos CLI

#### [MODIFY] [client.go](file://../../internal/mcp/client.go)
- Add custom environment variable support (`env` field, `SetEnv` method) to `PluginClient` to configure child process environment variables.
- Add `GetTransport` method to retrieve the active transport under unit testing.

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Wrap the `imagegen_generate` MCP tool call in a retry/fallback handler in `generateImageRaw`.
- If the call fails:
  - Inspect the error message for patterns indicating authorization, credential, or model eligibility failure (e.g., `"model does not exist"`, `"401"`, `"unauthorized"`, `"api key"`).
  - Query current backend capabilities using `imagegen_get_capabilities` to detect the active backend name.
  - Determine the alternative fallback backend:
    - If active is `openai` or `midjourney`, check if Gemini credentials are set. If so, select `"google"` as fallback.
    - If active is `google`, `imagen`, or `veo`, check if OpenAI credentials are set. If so, select `"openai"` as fallback.
  - If fallback is supported, instantiate a new local `PluginClient` with `POWERWORD_IMAGEGEN_BACKEND` set in its environment variables, start it, and retry generation.

---

## Verification Plan

### Automated Tests
- In `pithos`: Added `TestBrew_ImagegenFallback` in `pipeline_test.go` mocking `imagegen_generate` calls to return a credential error on the first invocation, verifying that Pithos correctly performs the fallback request to the alternate provider using `multiTransport` mock server connections.

### Manual Verification
- Configure a project with a dummy/invalid OpenAI key but a valid Google Gemini key.
- Run `pithos brew` with OpenAI backend selected.
- Verify that Pithos detects the OpenAI credential failure, falls back to Google, generates the images successfully, and writes the fallback warning to the logs.
