# plan: Task 5.13: Multi-Provider LLM Fallback & Retries

**Status:** Open
**Go Version:** 1.26.4

Implement dynamic retries with exponential backoff and automatic provider switching (e.g., fall back to OpenAI if Gemini fails due to rate limits or transient errors) to ensure Pithos pipeline runs reliably without halts in CI/CD or headless environments. This fallback mechanism applies to all pipeline LLM calls, including style guide generation and manuscript generation.

## User Review Required

> [!WARNING]
> **Provider Switching Billing Implications**:
> If the primary provider fails, switching to the secondary provider will result in billing charges mapping to that backup provider. Pithos will automatically track and record these backup costs in the manifest telemetry session, but users should verify they have appropriate backup API keys configured in `.pithos.toml` or environment variables.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add configurations for retry options:
  ```toml
  [api.retries]
  max_attempts = 3
  initial_backoff_ms = 500
  enable_fallback = true
  ```

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Implement a generic retry/fallback utility or refactor LLM client calls inside `generateManuscript`:
  - Wrap both `GenerateVisualGuides` and `GenerateStanzas` in retry loops:
    * If a generation attempt fails, execute retries with exponential backoff.
    * If retries fail and `enable_fallback` is active, check if another provider's key is configured (e.g. switch from Gemini to OpenAI, or vice versa).
    * Initialize the fallback client, retry the failed call, and record usage under the fallback provider's model name in telemetry metrics.

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Visual guides and stanzas generation retry logic executes up to `max_attempts` when errors are returned.
  * Backoff duration increases exponentially.
  * System switches to secondary provider when primary fails all attempts and fallback is enabled.
