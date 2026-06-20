# plan: Task 5.33: Interactive CLI Prompt Wizards via Survey

**Status:** Proposed
**Go Version:** 1.26.4

This task integrates `github.com/AlecAivazis/survey` into Pithos to provide interactive wizards and select inputs for key CLI subcommands. This reduces reliance on manually typing complex flags or editing JSON files in hidden directories.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Import `github.com/AlecAivazis/survey/v2` and download dependencies.

### Interactive Scaffolding

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Refactor the command runner to check if flags are set.
- If run without required flags, prompt the user interactively for:
  - Theme
  - Format (paperback, hardcover) using a `Select` list
  - Trim size (8.5x8.5, 6x9, etc.) using a `Select` list
  - Target page count
- Replace the custom confirmation logic with a clean `survey.Confirm` prompt.

### Interactive Selective Redo

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Implement a `--select` or dynamic prompt mode for `--pages` when running `pithos brew`.
- Display a `MultiSelect` list displaying stanzas and their completion status.
- Allow the user to check/uncheck pages to regenerate.

---

## Verification Plan

### Automated Tests
- Implement unit tests for options parsing and conditional fallback.
- Mock console inputs for the prompts inside test cases.

### Manual Verification
- Run `pithos initiate` in the terminal and confirm the prompt selections guide you through setup.
- Run `pithos brew --pages` and verify you can check/uncheck stanzas using keyboard arrow keys and spacebar.
