# plan: Task 5.30: Preflight Diagnostics Doctor Extensions

**Status:** Completed
**Date Completed:** 2026-06-20
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.3%

This task extends the `pithos doctor` preflight check subcommand. It registers check validation for the `pw-mcp-pdfcheck` plugin server, and elevates the image generation capability checks so that `pithos doctor` fails if the active backend does not support character references (`cref`) but a local book's manifest requests a character profile.

## User Review Required

> [!NOTE]
> **Check Severity:** The `pw-mcp-pdfcheck` plugin check will be registered as non-critical. If it is missing or handshake fails, `pithos doctor` will report a warning rather than failing the execution with a non-zero exit status, since it is only needed at the final assembly stage.

> [!IMPORTANT]
> **Cref Support Enforcement:** Without overrides (to be introduced in Task 5.31), `pithos doctor` will now return a `FAIL` status if character profiles are requested in `books/*/manifest.json` but the active backend does not support `cref`. This fails fast to prevent running expensive or incorrect pipelines.

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
- Elevate the unsupported `cref` status from a warning to a failure when a character profile is defined/requested in any local book manifest:
  ```go
  if seedingRequested {
      status = StatusFail
      msg += " - ERROR: active backend does not support cref, but local books request character profiles"
  }
  ```

---

## Verification Plan

### Automated Tests
- Update unit tests in `doctor_test.go` to mock calls to the `pw-mcp-pdfcheck` client handshake.
- Verify that a failed handshake for the PDF checker displays a warning instead of a failure block.
- Update `TestDoctor_ImageGenCapabilitiesWarning` (or add/rename tests) to verify that `RunDiagnostics` returns `hasFailure == true` and that the plugin status is `StatusFail` when the active backend lacks `cref` support but character profiles are defined in local book manifests.
- Verify total statement coverage meets or exceeds 91%.

### Manual Verification
- Run `pithos doctor` in a shell workspace:
  ```bash
  ./bin/pithos doctor
  ```
- Temporarily change `pdfcheck_path` in `.pithos.toml` to a non-existent binary path, re-run `pithos doctor`, and verify that the command displays a warning for the plugin.
- Configure `google` backend (which has `SupportsCref = false`) and ensure a book manifest has a `character_profile` defined. Run `./bin/pithos doctor` and verify it exits with a non-zero exit code due to the capability failure.
