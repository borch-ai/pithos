# plan: Task 7.2: EPUB / Digital Publication Export

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit/Integration Test Coverage Target:** ≥91.0%

Integrate with the standalone `pw-mcp-epub` Powerword MCP plugin to export the parodic manuscript and generated illustration assets into a valid reflowable EPUB file.

## User Review Required

> [!NOTE]
> None.

---

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add `EPUBPath` to `MCPConfig` structure to manage the location of the `pw-mcp-epub` executable.
- Bind the environment variable `POWERWORD_EPUB_PATH` (or default binary name) in Viper configuration initialization.

### Pipeline Integration

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Update the `Assemble` orchestrator function to:
  - Initialize the MCP client subprocess using the configured `EPUBPath`.
  - Perform the JSON-RPC handshake to confirm communication.
  - Verify that the `compile_epub` tool is registered on the server.
  - Invoke `compile_epub` passing the manuscript path, stanzas illustration directory, desired output path, title, and author retrieved from `manifest.json`.
  - Register the resulting `.epub` output path under the key `epub` in the manifest output asset registry.
  - Provide graceful error propagation: if the plugin fails, log the error clearly and return it without crashing the CLI.

#### [NEW] [epub.go](file://../../cmd/pithos/epub.go)
- Define a new Cobra command `epubCmd` for `pithos epub`.
- Set up parsing for `--output` (the book directory).
- Execute the EPUB compilation client directly from the command handler by loading the workspace manifest and invoking the `pw-mcp-epub` tool, enabling standalone digital exports.

---

## End-to-End Test Suite Design

The E2E test suite in Pithos will be implemented in [integration_test.go](file://../../internal/pipeline/integration_test.go) as `TestAssemble_EPUBExport_Integration`. It will verify all elements of the integration pipeline:

### 1. Subprocess Compilation and Lifecycle
- The integration test will locate the sibling `powerword` repository at `../../../powerword/cmd/pw-mcp-epub` (or search locally if run via CLI).
- It will compile the `pw-mcp-epub` binary on-the-fly into a temporary folder and clean it up on test completion.
- Config `EPUBPath` will be dynamically redirected to this temporary compiled binary.

### 2. MCP Handshake and Tool Discovery
- The test will verify that Pithos can successfully launch the `pw-mcp-epub` subprocess via standard I/O.
- The test will assert that the `compile_epub` tool is exposed by the server during client initialization.

### 3. Pipeline Assembly Trigger
- The test will initialize a mock book workspace with:
  - A mock `manifest.json` containing metadata (Title, Author, UUID).
  - Multi-stanza markdown chapters.
  - Dummy images in the `images/` directory.
- It will run `Assemble()` and verify the compile-epub tool request executes successfully.

### 4. EPUB Package Standards Validation
Upon successful compilation, the test will programmatically unzip the resulting `.epub` file and assert the following strict EPUB 3 standards:
- **First-Entry Ordering**: The first entry in the ZIP archive must be named exactly `mimetype`.
- **MIME Compression Constraint**: The `mimetype` entry must be stored with **zero compression** (`zip.Store` / method 0).
- **MIME Value**: The content of the `mimetype` file must be exactly `application/epub+zip` with no trailing spaces or newlines.
- **Root Container Mapping**: The file `META-INF/container.xml` must exist, parse as valid XML, and contain a `<rootfile>` tag mapping `full-path="OEBPS/content.opf"`.
- **OPF Metadata and Spine**: The file `OEBPS/content.opf` must parse as valid XML and contain:
  - Correct metadata headers: `<dc:title>`, `<dc:creator>`, `<dc:language>`.
  - A unique `<dc:identifier>` matching the manifest UUID.
  - A complete manifest listing every generated chapter XHTML file and every image asset.
  - A spine mapping the correct logical reading order of the chapters.
- **EPUB 3 Table of Contents**: The file `OEBPS/toc.xhtml` must exist and contain valid XHTML5 navigation list elements (`<nav epub:type="toc">`).
- **Assets Presence**: Verify that all XHTML files and image files declared in the OPF manifest actually exist inside the unpacked ZIP container.

### 5. Error Boundaries
- **Missing Binary**: Test that a missing or invalid `EPUBPath` returns a descriptive error rather than causing a panic.
- **Tool Failure**: Mock a tool execution error (e.g. invalid target output directory or write permission denial) and verify that the pipeline bubble-up error captures the underlying message.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  go test -v -tags=integration ./internal/pipeline/...
  ```
- Verify total statement coverage meets or exceeds the project threshold of 91.0%.

### Manual Verification
1. Build both `powerword` and `pithos` binaries.
2. Run `pithos assemble` on a completed workspace.
3. Verify `book.epub` is generated alongside the print PDF.
4. Run standard EPUB validation utility:
   ```bash
   epubcheck book.epub
   ```
