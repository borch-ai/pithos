# plan: Task 5.30: Preflight Diagnostics Doctor Extensions

**Status:** Pending
**Go Version:** 1.26.4
**Unit Test Coverage Target:** ≥91.0%

This task extends the `pithos doctor` preflight check subcommand to inspect the presence, location, and connection handshake health of the `pw-mcp-pdfcheck` plugin server.

## User Review Required

> [!NOTE]
> **Check Severity:** The `pw-mcp-pdfcheck` plugin check will be registered as non-critical. If it is missing or handshake fails, `pithos doctor` will report a warning rather than failing the execution with a non-zero exit status, since it is only needed at the final assembly stage.

---

## Proposed Changes

### Preflight Diagnostics

#### [MODIFY] [doctor.go](file://../../internal/pipeline/doctor.go)
- Add `mcp.PluginPDFCheck` to the plugins array within the `DiagnoseMCPPlugins` function:
  ```go
  {mcp.PluginPDFCheck, "PDF Preflight Validation Plugin (pw-mcp-pdfcheck)", false},
  ```
- This will automatically check that:
  * The configuration points to a valid file.
  * The binary is executable.
  * Pithos can successfully launch the process and complete the MCP connection handshake.

---

## Verification Plan

### Automated Tests
- Update unit tests in `doctor_test.go` to mock calls to the `pw-mcp-pdfcheck` client handshake.
- Verify that a failed handshake for the PDF checker displays a warning instead of a failure block.
- Verify total statement coverage meets or exceeds 91%.

### Manual Verification
- Run `pithos doctor` in a shell workspace:
  ```bash
  ./bin/pithos doctor
  ```
- Temporarily change `pdfcheck_path` in `.pithos.toml` to a non-existent binary path, re-run `pithos doctor`, and verify that the command displays a warning for the plugin.
