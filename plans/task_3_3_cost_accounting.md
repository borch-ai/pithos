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

#### [MODIFY] [go.mod](file:///Users/human/code/pithos/go.mod)
- Require `github.com/borch-ai/powerword v0.0.0` (or local replaced path).
- Add `replace github.com/borch-ai/powerword => ../powerword`.

### State Representation

#### [MODIFY] [manifest.go](file:///Users/human/code/pithos/internal/manifest/manifest.go)
- Add a new struct `TelemetryMetrics` referencing the imported telemetry types:
  ```go
  import "github.com/borch-ai/powerword/pkg/telemetry"

  type TelemetryMetrics struct {
      ModelUsages      map[string]*telemetry.ModelUsage `json:"model_usages"`
      ImageGenerations int64                            `json:"image_generations"`
      TotalCostUSD     float64                          `json:"total_cost_usd"`
  }
  ```
- Add a `Telemetry` field to the root `Manifest` struct.

### Pipeline Integration

#### [MODIFY] [llm.go](file:///Users/human/code/pithos/internal/pipeline/llm.go)
- Parse `usageMetadata` (Gemini API) and `usage` (OpenAI/GPT API) in HTTP response bodies into standard `telemetry.TokenUsage` mappings.
- Use `telemetry.UsageTracker` to record LLM token usage and execute cost calculations using the shared model pricing tables.

#### [MODIFY] [brew.go](file:///Users/human/code/pithos/internal/pipeline/brew.go)
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

### Manual Verification
- Execute `pithos initiate` and run a short `pithos brew` execution.
- Inspect the generated `manifest.json` file under the target folder to ensure token counts and estimated costs match pricing schedules defined in the shared `powerword` config.
