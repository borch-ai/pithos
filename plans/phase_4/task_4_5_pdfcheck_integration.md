# plan: Task 4.5: Print-Ready PDF Preflight Validation (pw-mcp-pdfcheck)

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-19
**Unit Test Coverage:** 91.10%

This task integrates the print-ready PDF preflight inspector (`pw-mcp-pdfcheck` plugin) directly into Pithos's book assembly pipeline. After generating the interior PDF, Pithos will execute automated checks to verify page geometry, margins, safe zones, embedded fonts, and barcode details against KDP guidelines, failing the build on critical errors.

## User Review Required

> [!IMPORTANT]
> **Pipeline Failure Policy:** Any preflight checking failures (e.g. invalid bleed dimensions, un-embedded fonts, low DPI images) returned by the validation tool will cause `pithos assemble` to return a non-zero exit status and block progress.

---

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add `PDFCheckPath` (string) to `MCPConfig` mapped to `pdfcheck_path` under `mcp`.
- Register the default fallback parameter value as `"pw-mcp-pdfcheck"` during config initialization.

### MCP Client Layer

#### [MODIFY] [client.go](file://../../internal/mcp/client.go)
- Add the `PluginPDFCheck` constant:
  ```go
  PluginPDFCheck PluginType = "pw-mcp-pdfcheck"
  ```
- Update `ResolveBinaryPath` to return `config.Cfg.MCP.PDFCheckPath` for `PluginPDFCheck`.

### Assembly Pipeline

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Update `Assemble` to perform preflight checks:
  * Immediately after interior PDF compilation (`compileInteriorPDF` succeeds), initialize the `pw-mcp-pdfcheck` client.
  * Start the client process and perform the handshake.
  * Extract width and height dimensions from `m.BookProperties.TrimSize` (e.g., converting `"6x9"` to `6.0` and `9.0`).
  * Invoke the `validate_pdf` tool, passing:
    - `pdf_path`: absolute path to compiled interior PDF.
    - `expected_width_inches`: trim width.
    - `expected_height_inches`: trim height.
    - `bleed_inches`: `m.KDPLayout.Bleed`.
    - `min_gutter_inches`: `m.KDPLayout.MarginSize` (or safety zones).
  * Parse the validation response:
    - If the preflight check reports structural errors or returns `IsError == true`, print the checker report output and fail the assembly.
  * If a cover PDF asset is registered under `AssetRegistry["cover_pdf"]`:
    - Call `validate_cover_pdf` passing the cover PDF path, trim size, page count, and paper type.
    - Fail assembly if the cover verification fails (e.g., missing spine barcode or mismatched spine width).

---

## Verification Plan

### Automated Tests
- Mock the `validate_pdf` and `validate_cover_pdf` tool calls in `assemble_test.go`.
- Add test scenarios for successful validations and formatting violations.
- Run `TestAssemble_Integration_RealSubprocess` in `integration_test.go` which compiles the real `pw-mcp-pdfcheck` plugin as a subprocess binary and runs it end-to-end.
- Verify total statement coverage meets the strict 91% coverage criteria (`make check-coverage`).

### Manual Verification
- Compile `pw-mcp-pdfcheck` and define its executable path in `.pithos.toml`.
- Run the assembly subcommand:
  ```bash
  ./bin/pithos assemble --output books/cyber_diogenes
  ```
- Verify the terminal prints preflight verification steps and successfully completes or failures block output.
