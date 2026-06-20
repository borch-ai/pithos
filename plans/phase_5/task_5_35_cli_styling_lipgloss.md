# plan: Task 5.35: CLI Visual Styling & Diagnostics Formatting via Lip Gloss

**Status:** Proposed
**Go Version:** 1.26.4

This task integrates `github.com/charmbracelet/lipgloss` into Pithos to provide visual structure, borders, layouts, and colors for command-line outputs. It styles tables, execution telemetry summaries, and diagnostics checklists.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Import `github.com/charmbracelet/lipgloss` and download dependencies.

### Command Line Diagnostics

#### [MODIFY] [doctor.go](file://../../cmd/pithos/doctor.go)
- Format check results using colored badge prefixes:
  - `[ OK ]` (bold white on green background)
  - `[WARN]` (bold white on orange/yellow background)
  - `[FAIL]` (bold white on red background)
- Align checking components left to maintain a uniform column layout.

### Telemetry Summary

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Style the token telemetry and cost accounting receipt card at the end of runs using double borders, custom foreground highlights, and padded text layouts.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Apply matching style templates to the assembly cost summaries.

---

## Verification Plan

### Automated Tests
- Verify that command execution works normally with the new packages.
- Ensure styling functions return correct strings when raw ANSI escape codes are stripped or matched.

### Manual Verification
- Run `pithos doctor` and verify the output uses the new colored status badges.
- Complete a `pithos brew` run and verify the final cost summary displays within a clean bordered receipt card.
