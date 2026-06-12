# plan: Task 3.3: Token Telemetry & Cost Accounting

**Status:** Open (Issue #[TBD])

Implement tracking and logging of token counts and API consumption costs across the Pithos generation pipeline. Metrics should be stored in the book manifest so developers and monitors can track usage and budget.

## User Review Required

> [!IMPORTANT]
> **Shared Dependency on Powerword**:
> Pithos will declare a dependency on `github.com/borch-ai/powerword/pkg/telemetry` to consume the pricing tables and cost calculation logic. During local development, a `replace` directive mapping `github.com/borch-ai/powerword` to `../powerword` will be utilized.

---

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](../../go.mod)
- Require `github.com/borch-ai/powerword v0.0.0` (or local replaced path).
- Add `replace github.com/borch-ai/powerword => ../powerword`.

### Configuration

#### [MODIFY] [config.go](../../internal/config/config.go)
- Import `github.com/borch-ai/powerword/pkg/telemetry`.
- Add `Pricing map[string]telemetry.ModelPricing` to `Config` struct.
- In `finalizeLoad`, set standard defaults for `"gemini-1.5-flash"`, `"gpt-4o"`, and `"imagegen"`.

### State Representation

#### [MODIFY] [manifest.go](../../internal/manifest/manifest.go)
- Import `github.com/borch-ai/powerword/pkg/telemetry`.
- Add a new struct `TelemetryMetrics` referencing the imported telemetry types:
  ```go
  type TelemetryMetrics struct {
      ModelUsages      map[string]*telemetry.ModelUsage `json:"model_usages"`
      ImageGenerations int64                            `json:"image_generations"`
      TotalCostUSD     float64                          `json:"total_cost_usd"`
  }
  ```
- Add a `Telemetry` field of type `TelemetryMetrics` to the root `Manifest` struct.
- Add helper method `UpdateTotalCost(pricing map[string]telemetry.ModelPricing)` to Manifest.

### Pipeline Integration

#### [MODIFY] [llm.go](../../internal/pipeline/llm.go)
- Update `LLMClient` interface `GenerateStanzas` signature to return `telemetry.TokenUsage`:
  ```go
  GenerateStanzas(ctx context.Context, theme string, count int) ([]string, telemetry.TokenUsage, error)
  ```
- Parse `usageMetadata` (Gemini API) and `usage` (OpenAI/GPT API) in HTTP response bodies into standard `telemetry.TokenUsage` mappings.

#### [MODIFY] [brew.go](../../internal/pipeline/brew.go)
- Capture token metrics from the LLM client responses.
- Increment the image generation count when the MCP client completes illustrations.
- Accumulate metrics and save the updated manifest state at each checkpoint.

---

## Verification Plan

### Automated Tests
- Run command: `go test ./internal/pipeline/... ./internal/manifest/...`
- Unit tests verifying:
  * Parsing of token usage schemas from Gemini and OpenAI REST responses.
  * Correct updates of telemetry metrics in the manifest structure using the imported library.
  * Correct calculation of total costs (LLM + images).

### Manual Verification
- Execute `pithos initiate` and run a short `pithos brew` execution.
- Inspect the generated `manifest.json` file under the target folder to ensure token counts and estimated costs match pricing schedules defined in the shared `powerword` config.

