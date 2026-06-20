# plan: Task 5.32: Adopt Local Workspaces & Cache Structure

**Status:** Completed
**Date Completed:** 2026-06-20
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.4% (Actual)

This task migrates default book workspaces and configuration cache directories to the user home directory (`~/.local/share/pithos/`). This aligns with the Borch-AI standard (used by Kiln and Aeolian) to prevent cluttering the repository git tree and provide portability when running the Pithos CLI globally.

## User Review Required

> [!NOTE]
> All default outputs and temporary caches will write to `~/.local/share/pithos/workspaces/` and `~/.local/share/pithos/cache/` instead of the local repository directory. Users can still output locally by using `--output books/<name>` or configuring custom workspaces paths.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Set up a dynamic workspace root default if configuration paths are omitted.
- Add support for resolving `~` references or dynamic home paths in workspace options.

### Path Resolution

#### [MODIFY] [path.go](file://../../internal/pipeline/path.go)
- Modify `ResolveBookPath` to target `~/.local/share/pithos/workspaces/` instead of `books/` in the current working directory for relative outputs.
- Update test cases in `path_test.go` to assert dynamic resolving paths.

### Cleanup & Git

#### [MODIFY] [.gitignore](file://../../.gitignore)
- Retain `books/` and `.pithos-local-cache/` ignores for developers choosing custom local outputs, but clean up references to reflect that primary outputs are written to the home directory.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline/...` and ensure path resolving assertions reflect home directory defaults.

### Manual Verification
- Run `pithos initiate` without `--output` and verify the workspace is created under `~/.local/share/pithos/workspaces/book_<timestamp>`.
- Run `pithos initiate --output frog_book` and check that the folder resides under `~/.local/share/pithos/workspaces/frog_book`.
