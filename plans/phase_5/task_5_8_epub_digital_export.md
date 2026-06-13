# plan: Task 5.8: EPUB / Digital Publication Export

**Status:** Open (Issue #[TBD])

Integrate with a standalone `pw-mcp-epub` Powerword MCP plugin to export the parodic manuscript and generated illustration assets into a valid reflowable or fixed-layout EPUB file.

## User Review Required

> [!NOTE]
> None.

---

## Proposed Changes

### Dependencies & Configuration

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- [ ] Add `EPUBPath` to `MCPConfig` structure to manage the location of the `pw-mcp-epub` binary.
- [ ] Bind default path in Viper config initialization.

### Pipeline Integration

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- [ ] Update `Assemble` to launch the `pw-mcp-epub` MCP client.
- [ ] Call the `epub_export` tool on the MCP client to package the stanzas and page illustrations into a valid EPUB document.
- [ ] Register the EPUB path under the key `epub` in the manifest asset registry.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Unit tests verifying EPUB MCP server communication using a mock client.

### Manual Verification
- [ ] Run `pithos assemble` on a completed book directory, and verify that the output workspace contains both the print PDF and the EPUB digital release.
