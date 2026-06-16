# plan: Task 5.22: Preflight Diagnostic Subcommand (`pithos doctor`)

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Implement a preflight connection diagnostic subcommand (`pithos doctor`) that checks the setup and accessibility of all downstream services (MCP servers, API keys) before running the main book generation pipelines.

## User Review Required

> [!NOTE]
> This command is purely diagnostic and does not modify the manifest or generate book assets.

## Proposed Changes

### CLI Layer
- Implement `doctor.go` under `cmd/pithos/` to add `pithos doctor` command.
- Command will:
  - Load Viper configuration.
  - Attempt to resolve and run configured MCP binary servers (`pw-mcp-imagegen`, `pw-mcp-kdp-math`, `pw-mcp-typst`).
  - Test LLM credentials (Gemini, OpenAI) by calling a cheap `Ping` or model list check.
  - Report structured success/error checklists in the terminal.

## Verification Plan

### Manual Verification
- Run `pithos doctor` with valid credentials and verify a clean health report.
- Temporarily rename an MCP server path or corrupt an API key in `.env` and verify that `pithos doctor` correctly flags the specific failure.
