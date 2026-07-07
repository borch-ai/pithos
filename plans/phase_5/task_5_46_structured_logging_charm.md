# plan: Task 5.46: Structured Pipeline Logging via charmbracelet/log

**Status:** Completed
**Date Completed:** 2026-07-07
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.1%

This task integrates `github.com/charmbracelet/log` into Pithos to replace raw `fmt.Printf` debug/info printing across pipeline stages with levelled, color-coded, structured log output. The library is native to the Charm ecosystem, renders using Lipgloss styles, and automatically degrades gracefully to plain text when stdout is redirected to a non-TTY (e.g., CI logs or script output).

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Import `github.com/charmbracelet/log` and download dependencies.

### Centralized Logger Initialization

#### [MODIFY] [root.go](file://../../cmd/pithos/root.go)
- Initialize a global `*log.Logger` instance in the root command's `PersistentPreRunE` hook.
- Set the log level to `Info` by default, toggled to `Debug` when a `--verbose` or `--debug` CLI flag is provided.
- Configure output format: styled/colored for TTY, plain text for piped/redirected output.

### Pipeline Stage Logging

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Replace ad-hoc `fmt.Printf` progress messages (e.g., "Generating page X...") with structured `log.Info` / `log.Debug` calls.
- Emit structured key-value fields (e.g., `page`, `model`, `cost`) alongside human-readable messages.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Replace `fmt.Printf` stage announcements with `log.Info` calls.
- Log PDF compilation progress and preflight results with structured metadata.

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
- Replace workspace scaffolding print statements with structured log lines.

### Dry-Run Compatibility (Task 5.37)

- Ensure that `--dry-run` mode uses `log.Debug` to describe every action that *would* be executed without performing it, making dry-run output clean and parseable.

---

## Verification Plan

### Automated Tests
- Unit tests to ensure log level filtering respects the configured level.
- Verify that no ANSI escape codes appear in log output when stdout is non-TTY (test by redirecting output in subprocess tests).

### Manual Verification
- Run `pithos initiate --theme "dreadful-monsters" --output books/test_book --dry-run` and verify that color-coded `INFO` log messages appear cleanly in the terminal.
- Run `pithos initiate --theme "dreadful-monsters" --output books/test_book_debug --dry-run --debug` and confirm verbose `DEBU` diagnostic messages appear.
- Run `pithos initiate --theme "dreadful-monsters" --output books/test_book_nocolor --dry-run --debug 2>&1 | cat` and verify log output degrades to plain text with no ANSI escape sequences.
- Clean up any generated test directories afterwards: `rm -rf books/test_book*`.
