# plan: Task 2.1: MCP Client Integration

**Status:** Completed
**Go Version:** Go 1.26.4
**Date Completed:** 2026-06-11
**Unit Test Coverage:** 92.20% total coverage (meets the 91% threshold)

Integrate the `modelcontextprotocol/go-sdk` into Pithos to allow the pipeline to securely launch and communicate with Powerword's MCP plugin ecosystem via standard I/O (stdio).

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### MCP Subsystem

#### [NEW] [client.go](file:///Users/human/code/pithos/internal/mcp/client.go)
- [x] Implement a struct `PluginClient` that manages a child process (e.g., executing `pw-mcp-imagegen`).
- [x] Implement lifecycle methods: `Start()`, `Stop()`.
- [x] Implement a method `CallTool(toolName string, args map[string]interface{}) (string, error)` that wraps the SDK's internal JSON-RPC calls.

### Configuration Integration

#### [MODIFY] [client.go](file:///Users/human/code/pithos/internal/mcp/client.go)
- [x] Connect the plugin process starter to read the binary paths configured in `viper` (e.g., `mcp.imagegen_path`, `mcp.kdp_math_path`).
- [x] Fallback to searching the system PATH if no explicit configuration path is found in the `.pithos.toml` file.


---

## Verification Plan

### Automated Tests
- [x] Run command: `go test ./internal/mcp/...`
- [x] Unit test the `CallTool` wrapper using a mock MCP server.

### Manual Verification
- [x] Run `make build` and `make test`.
