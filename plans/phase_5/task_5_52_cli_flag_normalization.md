# plan: Task 5.52: CLI Flag Normalization & Directory Alias (`--dir` / `-d`)

**Status:** Open
**Go Version:** 1.26+

## Overview

Reconcile command-line flag contracts across all Pithos subcommands to ensure seamless integration with the Powerword MCP wrapper (`pw-mcp-pithos`) and upstream orchestrators like Kiln.

Currently, `pw-mcp-pithos` invokes Pithos subcommands with `--dir <path>`:
- `pithos initiate --dir <path>`
- `pithos brew --dir <path>`
- `pithos assemble --dir <path>`

However, within Pithos:
- `pithos brew` uses `--output`.
- `pithos assemble` uses `--input`.
- Other commands (`preview`, `status`, `clean`, `character`) vary between positional arguments and workspace flags.

This task standardizes all workspace-targeting subcommands to accept `--dir` and shorthand `-d` alongside their legacy flags, ensuring full backward compatibility while satisfying the MCP execution contract.

## User Review Required

> [!NOTE]
> **Backward Compatibility**:
> Existing flags (`--output` on `brew`, `--input` on `assemble`) will be retained as aliases. If both `--dir` and `--output`/`--input` are passed, `--dir` takes precedence.

## Open Questions

None.

## Proposed Changes

### CLI Scaffolding & Command Bindings

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add `--dir` / `-d` flag to `brewCmd`.
- Bind flag resolution logic: if `--dir` is provided, use it as the target workspace; otherwise fallback to `--output`.

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Add `--dir` / `-d` flag to `assembleCmd`.
- Bind flag resolution logic: if `--dir` is provided, use it as the target workspace; otherwise fallback to `--input`.

#### [MODIFY] [preview.go](file://../../cmd/pithos/preview.go)
- Support `--dir` / `-d` flag in addition to positional book slug/path.

#### [MODIFY] [status.go](file://../../cmd/pithos/status.go)
- Support `--dir` / `-d` flag in addition to positional book slug.

#### [MODIFY] [clean.go](file://../../cmd/pithos/clean.go)
- Support `--dir` / `-d` flag in addition to positional book slug.

#### [MODIFY] [character.go](file://../../cmd/pithos/character.go)
- Support `--dir` / `-d` flag in addition to positional book slug/path.

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Ensure `--dir` / `-d` flag behaves identically whether initiating in a custom path or default workspace.

## Verification Plan

### Automated Tests
- Run unit test suite:
  ```bash
  make test
  ```
- Add CLI flag parsing test suite covering:
  - `pithos brew --dir <path>` vs `pithos brew --output <path>`
  - `pithos assemble --dir <path>` vs `pithos assemble --input <path>`
  - Shorthand `-d` verification across commands.
  - `pithos status --dir <path>`, `clean --dir <path>`, `character --dir <path>`
  - Precedence verification when both `--dir` and legacy flags are passed.
- Run coverage verification:
  ```bash
  make check-coverage
  ```
