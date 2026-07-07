# plan: Task 5.47: Workspace Cache Management Command (pithos cache)

**Status:** Proposed
**Go Version:** 1.26.4

This task adds a `pithos cache` subcommand to help developers and users inspect and clean up local space inside `~/.local/share/pithos/`.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [cache.go](file://../../cmd/pithos/cache.go)
- Create a new Cobra command `cacheCmd` for `pithos cache`.
- Expose subcommands:
  - `status`: Calculate and print the size of cache directories (`~/.local/share/pithos/workspaces` and internal caches).
  - `prune`: Remove temp files and intermediate build folders from local workspaces while keeping `manifest.json` and generated print layouts intact.

### Pipeline Engine

#### [NEW] [cache.go](file://../../internal/pipeline/cache.go)
- Implement `GetCacheStatus() (CacheStatus, error)`:
  - Walk the root directory `~/.local/share/pithos/`.
  - Calculate total directories, files, and disk usage in bytes.
- Implement `PruneCache(dryRun bool) (int64, error)`:
  - Find and delete `.tmp` folders, orphaned build artifacts, and cached image files that are not referenced by any active workspace `manifest.json`.
  - Return total freed space in bytes.

---

## Verification Plan

### Automated Tests
- Test path resolution and file-walk calculations in `internal/pipeline/cache_test.go` using a temporary directory structure.
- Verify `PruneCache` doesn't delete active manifests or referenced images.

### Manual Verification
- Run `pithos cache status` and verify size output matches disk utility measurements.
- Run `pithos cache prune` and verify orphaned assets are successfully removed.
