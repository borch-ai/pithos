# plan: Task 5.53: Headless & Batch Execution Mode (`--headless` / `--non-interactive`)

**Status:** Complete
**Go Version:** 1.26+

## Overview

Provide headless and non-interactive batch execution support across all Pithos commands. When Pithos is driven by automated orchestrators (such as `kiln forge`, cron runners, or headless CI/CD pipelines), execution must never stall on interactive `stdin` prompts or attempt to open GUI desktop applications (like web browsers).

Currently:
- `pithos initiate` launches an interactive Charm `huh` wizard if required flags are omitted.
- `pithos brew` can pause for stanza review and automatically launches the system default web browser to view HTML previews unless `--silent` is passed.
- `pithos brew --pages` launches an interactive multi-select prompt if page arguments are not explicitly given.

This task introduces a global `--headless` flag (aliased with `--non-interactive`) with automatic detection of CI environments (accepting truthy values such as `"1"` or `"true"` for `CI` or `PITHOS_HEADLESS`), ensuring strictly non-interactive, headless-safe execution.

## User Review Required

> [!IMPORTANT]
> **Strict Non-Interactive Validation**:
> In headless mode, missing required flags or parameters will immediately terminate the command with exit code `1` and print clear flag guidance instead of falling back to interactive terminal wizards.

> [!NOTE]
> **Preview Auto-Open Suppression**:
> Headless mode automatically implies `--silent`, preventing browser launch routines from spawning detached GUI processes in headless server environments.

## Open Questions

None.

## Proposed Changes

### Configuration & Root Command

#### [MODIFY] [root.go](file://../../cmd/pithos/root.go)
- Register persistent flags `--headless` and `--non-interactive` on root command.
- Bind `PITHOS_HEADLESS` environment variable and auto-detect truthy `CI` / `PITHOS_HEADLESS` (`"1"`, `"true"`, `"yes"`, `"on"`) or non-TTY `stdin` (`!term.IsTerminal(os.Stdin.Fd())`).
- Expose a helper `IsHeadless() bool` accessible across all subcommand handlers.

### Command Handlers

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Check `IsHeadless()` before invoking the `huh` interactive wizard.
- If headless and required arguments are missing, return an error specifying the missing flags.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Propagate headless setting into pipeline options.
- If headless, bypass interactive stanza review and interactive page selector prompts.
- Default `silent` to `true` when headless is enabled.

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Ensure assemble respects headless mode and suppresses any desktop UI hooks.

#### [MODIFY] [preview.go](file://../../internal/pipeline/preview.go)
- Ensure `openBrowser` routine checks pipeline headless option before attempting system commands (`open`, `xdg-open`, `start`).

## Verification Plan

### Automated Tests
- Run unit test suite:
  ```bash
  make test
  ```
- Add unit tests verifying:
  - `IsHeadless()` returns true when `--headless` flag, `--non-interactive` flag, or truthy `CI` / `PITHOS_HEADLESS` env (e.g. `1` or `true`) is set.
  - `pithos initiate` in headless mode without flags fails immediately with exit code 1 and error message rather than blocking on stdin.
  - `pithos brew` in headless mode suppresses browser preview launch and interactive review gates.
- Verify coverage threshold:
  ```bash
  make check-coverage
  ```
