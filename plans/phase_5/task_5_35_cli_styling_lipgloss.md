# plan: Task 5.35: CLI Visual Styling & Diagnostics Formatting via Lipgloss

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-26
**Unit Test Coverage:** 91.30%

This task integrates `github.com/charmbracelet/lipgloss` into Pithos to provide visual structure, borders, layouts, and colors for command-line outputs. It styles tables, execution telemetry summaries, and diagnostics checklists. In response to PR review feedback, all Lipgloss styles were centralized in a shared `internal/ui` package to avoid style drift.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Import `github.com/charmbracelet/lipgloss` and download dependencies.

### Centralized Theme & Styles

#### [NEW] [style.go](file://../../internal/ui/style.go)
- Holds standard themes, colors, and badge definitions (OK, FAIL, WARN, SKIP) used across different commands.

#### [NEW] [style_test.go](file://../../internal/ui/style_test.go)
- Unit tests to verify correct style initialization and formatting, ensuring high statement coverage.

### Command Line Diagnostics

#### [MODIFY] [doctor.go](file://../../cmd/pithos/doctor.go)
- Format check results using colored badge prefixes from the central UI package.
- Align checking components left using `lipgloss.Width` to support non-ASCII characters.

### Telemetry Summary

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Style the token telemetry and cost accounting receipt card at the end of runs using centralized styles and padded text layouts.
- Falls back to plain text printing when stdout is redirected (non-TTY).

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Apply matching style templates to the assembly cost summaries.
- Falls back to plain text printing when stdout is redirected (non-TTY).

---

## Verification Plan

### Automated Tests
- Run `make check-coverage` to verify unit tests run correctly.
- Centralized UI styles are covered in `internal/ui/style_test.go`.
- Telemetry fallback code path is covered in `internal/pipeline/pipeline_test.go` by mocking the `isTTY` hook.

### Manual Verification
- Run `pithos doctor` and verify the output uses the new colored status badges.
- Complete a `pithos brew` run and verify the final cost summary displays within a clean bordered receipt card.
