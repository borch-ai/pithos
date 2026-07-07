# plan: Task 5.33: Interactive CLI Prompt Wizards via Huh

**Status:** Completed
**Date Completed:** 2026-06-26
**Unit Test Coverage:** 91.00%
**Go Version:** 1.26.4

This task integrates `github.com/charmbracelet/huh` into Pithos to provide interactive wizards and select inputs for key CLI subcommands. `huh` is a high-level interactive form library built on Bubbletea, native to the Charm ecosystem we already use for Lipgloss. This reduces reliance on manually typing complex flags or editing JSON files in hidden directories.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Import `github.com/charmbracelet/huh` and download dependencies.

### Interactive Scaffolding

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Refactor the command runner to check if flags are set.
- If run without required flags, prompt the user interactively using a `huh.Form` for:
  - Theme (`huh.NewInput`)
  - Format (paperback, hardcover) using `huh.NewSelect`
  - Trim size (8.5x8.5, 6x9, etc.) using `huh.NewSelect`
  - Target page count (`huh.NewInput` with integer validation)
- Replace the custom confirmation logic with a clean `huh.NewConfirm` prompt.

### Interactive Selective Redo

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Implement a `--select` or dynamic prompt mode for `--pages` when running `pithos brew`.
- Display a `huh.NewMultiSelect` list showing stanzas and their completion status.
- Allow the user to check/uncheck pages to regenerate using arrow keys and spacebar.

---

## Verification Plan

### Automated Tests
- Implement unit tests for options parsing and conditional fallback.
- Mock console inputs for the prompts inside test cases (huh supports accessible mode for non-TTY/test environments).

### Manual Verification
- Run `pithos initiate` in the terminal and confirm the prompt selections guide you through setup.
- Run `pithos brew --pages` and verify you can check/uncheck stanzas using keyboard arrow keys and spacebar.

