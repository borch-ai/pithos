# plan: Task 5.22: Preflight Diagnostic Subcommand (`pithos doctor`)

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-16
**Unit Test Coverage:** 91.2%

This task implements a preflight connection diagnostic subcommand (`pithos doctor`). It checks the setup and accessibility of all downstream services (MCP servers, API keys) before running the main book generation pipelines, reporting a structured checklist in the terminal.

## User Review Required

> [!NOTE]
> **Strictly Read-Only/Diagnostic:** This command is purely diagnostic, meaning it will never alter the manifest, checkpoint files, or compile actual book assets. It runs a lightweight check and exits.

---

## Proposed Changes

### CLI Layer

#### [NEW] [doctor.go](file://../../cmd/pithos/doctor.go)
- Defined `doctorCmd` using Cobra.
- Parsed standard configuration and called the diagnostic runner.
- Output a styled terminal checklist (with green checkmarks `[✔]` for success and red crosses `[✘]` for failure).

### Core Diagnostics Engine

#### [NEW] [doctor.go](file://../../internal/pipeline/doctor.go)
- Implemented `RunDiagnostics(ctx context.Context) error`.
- Performed the following checks:
  1. **Configuration Integrity:** Check if the config file was loaded, displaying loaded values.
  2. **Credential Audits:** Verify `PITHOS_API_GEMINI_KEY` (or `api.gemini_key`) and `PITHOS_API_OPENAI_KEY` (or `api.openai_key`) configuration.
  3. **MCP Binary Check:** Iterate through configured binary paths (`pw-mcp-imagegen`, `pw-mcp-kdp-math`, `pw-mcp-seo`, `pw-mcp-video`, `pw-mcp-typst`, `pw-mcp-cloud`) and verify if they are executable via `exec.LookPath`.
  4. **MCP Connection Handshake:** For each found binary, initialize a temporary `PluginClient`, call `Start` to verify stdio communication, and call `Stop`.
  5. **LLM Connection Handshake:** If LLM keys are configured, instantiate a lightweight adapter client and execute a basic ping prompt (e.g. `say 'pong'`) to verify remote network access.
- Non-critical plugins (`pw-mcp-seo`, `pw-mcp-video`, and `pw-mcp-cloud`) are reported as warnings instead of hard failures.

### Testing

#### [NEW] [doctor_test.go](file://../../internal/pipeline/doctor_test.go)
- Tested `RunDiagnostics` using mock MCP servers and mock LLM clients.
- Verified that correct diagnostics are reported when a binary is missing or LLM call fails.
- Maintained target unit test coverage >= 91.2%.

### Build & Installation

#### [MODIFY] [Makefile](file://../../Makefile)
- Added `install` target to compile and install Pithos globally using `go install ./cmd/pithos`.
- Added `install` to `.PHONY`.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline -run="TestDoctor.*"`
- Verify test coverage remains above the 91.2% threshold.

### Manual Verification
1. Run `pithos doctor` with valid credentials and observe a healthy report.
2. Corrupt/remove the Gemini API key and rename `pw-mcp-imagegen` in the config to a non-existent path. Run `pithos doctor` and verify that the command displays failures for both checks and returns a non-zero exit code.
