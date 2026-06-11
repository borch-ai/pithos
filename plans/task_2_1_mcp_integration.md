# plan: Task 2.1: MCP Client Integration

**Status:** Open (Issue #[TBD])

Integrate the `modelcontextprotocol/go-sdk` into Pithos to allow the pipeline to securely launch and communicate with Powerword's MCP plugin ecosystem via standard I/O (stdio).

## Proposed Changes

### MCP Subsystem

#### [NEW] [client.go](file:///Users/human/code/pithos/internal/mcp/client.go)
- [ ] Implement a struct `PluginClient` that manages a child process (e.g., executing `pw-mcp-imagegen`).
- [ ] Implement lifecycle methods: `Start()`, `Stop()`.
- [ ] Implement a method `CallTool(toolName string, args map[string]interface{}) (string, error)` that wraps the SDK's internal JSON-RPC calls.

### Configuration Integration

#### [MODIFY] [client.go](file:///Users/human/code/pithos/internal/mcp/client.go)
- [ ] Connect the plugin process starter to read the binary paths configured in `viper` (e.g., `mcp.imagegen_path`, `mcp.kdp_math_path`).
- [ ] Fallback to searching the system PATH if no explicit configuration path is found in the `.pithos.toml` file.


---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/mcp/...`
- [ ] Unit test the `CallTool` wrapper using a mock MCP server.

### Manual Verification
- [ ] Run `make build` and `make test`.
