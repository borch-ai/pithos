# plan: Task 7.1: Trend-Based Brainstorming

**Status:** Open (Issue #[TBD])

Implement the `brainstorm` command and connect to the external `pw-mcp-trends` Powerword MCP plugin to generate parodic themes, titles, and illustration styles based on real-time news or search trends.

## User Review Required

> [!NOTE]
> None.

---

## Proposed Changes

### Dependencies & Configuration

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- [ ] Add `TrendsPath` to `MCPConfig` structure to manage the location of the `pw-mcp-trends` binary.
- [ ] Bind default path in Viper config initialization.

### Pipeline Integration

#### [NEW] [brainstorm.go](file://../../internal/pipeline/brainstorm.go)
- [ ] Implement `Brainstorm` function that connects to the `pw-mcp-trends` MCP server.
- [ ] Query the server for trending news parodies, returning a list of ideas (title, theme, style suggestion).
- [ ] Render the output cleanly in the console.

#### [NEW] [brainstorm.go](file://../../cmd/pithos/brainstorm.go)
- [ ] Add the `brainstorm` CLI subcommand to Cobra.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Unit tests checking CLI output structure using a mock MCP trends client.

### Manual Verification
- [ ] Run `pithos brainstorm` and verify that the command displays parodic ideas with details.
