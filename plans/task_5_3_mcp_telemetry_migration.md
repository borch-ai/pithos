# plan: Task 5.3: MCP Telemetry Migration

**Status:** Open (Issue #[TBD])

Migrate Pithos's in-process token telemetry and cost accounting from the imported Go library (`github.com/borch-ai/powerword/pkg/telemetry`) to a standalone, decoupled MCP server (`pw-mcp-telemetry`) communicating over stdio.

## User Review Required

> [!NOTE]
> **Decoupling Dependency**:
> This task completely removes the compile-time dependency of Pithos on Powerword's source code, moving the integration entirely to runtime RPC over the Model Context Protocol.

---

## Proposed Changes

### Dependencies & Configuration

#### [MODIFY] [go.mod](file:///Users/human/code/pithos/go.mod)
- Remove `github.com/borch-ai/powerword` from required modules.
- Remove the local `replace` directive mapping to `../powerword`.

#### [MODIFY] [config.go](file:///Users/human/code/pithos/internal/config/config.go)
- Add `TelemetryPath` to `MCPConfig` structure to manage the location of the `pw-mcp-telemetry` binary.
- Bind default paths in Viper config initialization.

### State Representation

#### [MODIFY] [manifest.go](file:///Users/human/code/pithos/internal/manifest/manifest.go)
- Refactor the `TelemetryMetrics` struct to define native Pithos JSON structures instead of referencing `telemetry.ModelUsage` package types:
  ```go
  type TelemetryMetrics struct {
      ModelUsages      map[string]ModelUsage `json:"model_usages"`
      ImageGenerations int64                 `json:"image_generations"`
      TotalCostUSD     float64               `json:"total_cost_usd"`
  }

  type ModelUsage struct {
      InputTokens  int64 `json:"input_tokens"`
      OutputTokens int64 `json:"output_tokens"`
      CachedTokens int64 `json:"cached_tokens"`
  }
  ```

### Pipeline Integration

#### [MODIFY] [llm.go](file:///Users/human/code/pithos/internal/pipeline/llm.go) / [brew.go](file:///Users/human/code/pithos/internal/pipeline/brew.go)
- Initialize an MCP client connection to the `pw-mcp-telemetry` server process.
- Call the `calculate_tokens_cost` tool with the model token parameters on every iteration response.
- Update running totals and save them to `manifest.json`.

---

## Verification Plan

### Automated Tests
- Run command: `go test ./internal/pipeline/... ./internal/manifest/...`
- Unit tests verifying:
  * Mock MCP server response handling.
  * Correct persistence of parsed telemetry data in the state manifest.
