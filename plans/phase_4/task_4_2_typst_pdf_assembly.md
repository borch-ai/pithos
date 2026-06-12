# plan: Task 4.2: Typst PDF Layout Assembly

**Status:** Open (Issue #[TBD])

Integrate with a standalone `pw-mcp-typst` Powerword MCP plugin to compile the book manuscript stanzas and generated page illustrations into a print-ready PDF file based on the KDP dimensions stored in `manifest.json`.

## User Review Required

> [!IMPORTANT]
> **Typst Dependency**:
> Assembly will delegate PDF rendering to the external `pw-mcp-typst` server, ensuring Pithos does not carry bloated direct dependencies on heavy PDF compilation engines. The `pw-mcp-typst` binary path must be configurable in `.pithos.toml`.

---

## Proposed Changes

### Dependencies & Configuration

#### [MODIFY] [config.go](../../internal/config/config.go)
- [ ] Add `TypstPath` to `MCPConfig` structure to manage the location of the `pw-mcp-typst` binary.
- [ ] Bind default path in Viper config initialization.

### Pipeline Integration

#### [MODIFY] [assemble.go](../../internal/pipeline/assemble.go)
- [ ] Update `Assemble` to launch the `pw-mcp-typst` MCP client.
- [ ] Compile the page illustrations and stanzas into a custom Typst layout template dynamically.
- [ ] Call the `typst_compile` tool on the MCP client to generate the high-res print-ready PDF.
- [ ] Save the generated PDF file path to the manifest registry under the key `interior_pdf`.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Unit tests verifying Typst MCP server communication using a mock client.

### Manual Verification
- [ ] Run `pithos assemble` on a completed book directory, and verify that a print-ready PDF is correctly compiled and saved.
