# plan: Task 5.43: Developer Auto-Setup Command (pithos setup)

**Status:** Proposed
**Go Version:** 1.26.4

This task implements a `pithos setup` subcommand to automate local developer onboarding by locating, compiling, and symlinking/installing sibling MCP plugin binaries from the `powerword` repository.

## User Review Required

> [!NOTE]
> This command expects the sibling repository `powerword` to exist at `../powerword` relative to the Pithos directory. If not found, it will guide the user to download precompiled binaries or install them manually.

## Proposed Changes

### Command Layer

#### [NEW] [setup.go](file://../../cmd/pithos/setup.go)
- Create Cobra command `setupCmd` for `pithos setup`.
- Short: "Auto-compiles and sets up local MCP dependencies"
- Executes `pipeline.SetupMCPBinaries()` and displays colored progress/output.

### Pipeline Engine

#### [NEW] [setup.go](file://../../internal/pipeline/setup.go)
- Define default install path: `~/.local/share/pithos/bin/`.
- Implement `SetupMCPBinaries(options SetupOptions) error`:
  - Locate `../powerword` sibling repository.
  - For each required MCP server (imagegen, kdp-math, seo, viral, typst, cloud, pdfcheck):
    - Verify subdirectory exists under `../powerword/cmd/`.
    - Run `go build -o ~/.local/share/pithos/bin/<mcp_name> .` asynchronously or sequentially.
    - If the build succeeds, log success.
    - If `powerword` is not found, verify if system binaries exist in `$PATH`.
  - Print recommended `.pithos.toml` configuration template pointing to the local `~/.local/share/pithos/bin/` paths.

---

## Verification Plan

### Automated Tests
- Implement unit tests under `internal/pipeline/setup_test.go` verifying build triggers and path resolutions using mocked commands or directories.

### Manual Verification
- Run `pithos setup` in the terminal.
- Verify binaries compile and are placed in `~/.local/share/pithos/bin/`.
