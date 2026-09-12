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
- Set `pipeline.HeadlessMode` in `PersistentPreRunE` when `IsHeadless()` is true.

#### [NEW] [headless.go](file://../../cmd/pithos/headless.go)
- Expose `IsHeadless() bool` detecting `--headless`, `--non-interactive`, truthy `PITHOS_HEADLESS`, and truthy `CI` (`"1"`, `"true"`, `"yes"`, `"on"`).

### Command Handlers & Pipeline

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Check `IsHeadless()` before invoking the `huh` interactive theme wizard; return an error if missing.
- Check `IsHeadless()` when target directory already exists; reject overwrite immediately without prompting on stdin.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Propagate headless setting into pipeline options.
- Fail fast if interactive page selection (`--select`) is attempted in headless mode.
- In headless mode, automatically default `silent` to `true` and disable interactive review/TUI flags.

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- In `pipeline.Brew`, return an error immediately if `Headless` and `Select` are both enabled.

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Ensure assemble propagates `Headless` to pipeline options to suppress browser preview launch.

#### [MODIFY] [browser.go](file://../../internal/pipeline/browser.go)
- Check `HeadlessMode` and `PITHOS_HEADLESS` in `isHeadlessBrowser()` before launching browsers.
- CI environment detection is isolated to CLI initialization to ensure pipeline unit test stubs run cleanly.

## Verification Plan

### Automated Tests
- Run unit test suite:
  ```bash
  make test
  ```
- Unit tests verify:
  - `IsHeadless()` returns true when `--headless` flag, `--non-interactive` flag, or truthy `CI` / `PITHOS_HEADLESS` env (`1`, `true`, `yes`, `on`) is set.
  - `pithos initiate` in headless mode fails fast on missing theme or existing output directory.
  - `pithos brew` in headless mode rejects `--select`, suppresses browser preview launch, and bypasses interactive review.
  - `internal/pipeline.Brew` returns an error when `Headless` and `Select` are both passed.
- Verify coverage threshold:
  ```bash
  make check-coverage
  ```
  Coverage achieved: 91.3% (meets >= 91.0% requirement).
