# plan: Task 5.37: E2E Pipeline Simulation & Dry-Run Mode

**Status:** Proposed
**Go Version:** 1.26.4

This task implements a `--dry-run` simulation mode to allow testing structural compilation, layout calculations, and preview generation without executing live API or MCP tool calls.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Line Flags

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add `--dry-run` CLI flag to bypass live calls during manuscript and illustration generation.

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Add `--dry-run` CLI flag to bypass live Typst and KDP math compilation.

### Pipeline Engines

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- When `--dry-run` is active:
  - Mock the LLM client to return predefined dummy stanzas and illustration prompts.
  - Bypass the MCP image generation client.
  - Write static/placeholders or pre-existing local image files into the images output path.
  - Update `manifest.json` statuses to simulated completed values.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- When `--dry-run` is active:
  - Bypass `pw-mcp-kdp-math` and write mock margins/spine geometry calculation parameters to `manifest.json`.
  - Bypass `pw-mcp-typst` and write a placeholder dummy text file as the final PDF output.

---

## Verification Plan

### Automated Tests
- Test that executing `Brew` or `Assemble` with `--dry-run` active completes successfully with zero real API keys or MCP binaries configured on the system.

### Manual Verification
- Run `pithos initiate --theme "Test theme" --output simulation_test`.
- Run `pithos brew --output simulation_test --dry-run`.
- Verify the manifest is updated and placeholder files are written to the workspace.
- Run `pithos assemble --output simulation_test --dry-run` and verify it finishes successfully.
