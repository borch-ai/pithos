# plan: Task 5.34: List Local Book Workspaces (pithos ls)

**Status:** Completed
**Go Version:** 1.26.5
**Date Completed:** 2026-07-15
**Unit Test Coverage:** 91.0%

This task implements a `pithos ls` subcommand to inspect and list book workspaces. By scanning the configured default workspace root directory and merging it with a global registry of custom workspace paths (stored in `~/.config/pithos/registry.json`), Pithos parses the `manifest.json` for each workspace and prints a formatted table showing the book's properties, current milestones/progress, and accumulated costs.

## User Review Required

> [!NOTE]
> Adds `github.com/charmbracelet/lipgloss` dependency for styling the CLI table outputs.
> Implements a global registry under `~/.config/pithos/registry.json` to keep track of books initialized outside of the default workspaces root.

## Proposed Changes

### Command Layer

#### [MODIFY] [ls.go](file://../../cmd/pithos/ls.go)
- Create a new Cobra command `lsCmd` for `pithos ls`.
- Short: "List all book workspaces"
- The command runner:
  - Resolves the default workspace root directory (checking config or dynamic defaults).
  - Calls `pipeline.ListWorkspaces(root)` to get the summaries.
  - Formats the output using `github.com/charmbracelet/lipgloss/table` styles to display columns:
    `DIRNAME | THEME | FORMAT | PAGES | MILESTONES | COST`

### Registry Layer

#### [NEW] [registry.go](file://../../internal/registry/registry.go)
- Implement a global registry manager to track custom book workspace paths.
- Operations:
  - `Load()`: Load registered workspace paths from the JSON database.
  - `Add(path)`: Add a new path, resolving it to its absolute representation and avoiding duplicates.
  - `Remove(path)`: Remove a path.
  - `Prune()`: Scan paths and remove any that are stale (do not exist on disk). Pruning only triggers if the file path stat returns `os.ErrNotExist` to avoid accidental loss on transient disk or permission errors.
- Updates are written atomically using a temporary file and `os.Rename`.

### Pipeline Engine

#### [MODIFY] [ls.go](file://../../internal/pipeline/ls.go)
- Update `ListWorkspaces(workspaceRoot string) ([]BookSummary, error)`:
  - Scans both the default workspace root directory and the custom paths from the global registry.
  - Deduplicates all paths and loads `manifest.json` summaries.
  - Propagates permission errors during scans and loads (for safety), but automatically prunes paths that return `os.ErrNotExist`.
- Expose registry updates:
  - Automatically register workspaces on `initiate` and `brew`.

---

## Verification Plan

### Automated Tests
- Created unit tests under `internal/registry/registry_test.go` verifying basic CRUD operations, duplicate checks, atomic saving, and safe prune behaviors.
- Created unit tests under `internal/pipeline/ls_test.go` verifying that `ListWorkspaces` correctly merges results, handles permission denied directories, handles permission denied workspaces, and prunes stale registry paths.

### Manual Verification
- Run `pithos ls` and verify books show up in the table with correct metadata.
- Initialize or brew a book in a custom path (e.g. `pithos initiate --theme "parody" ./books/my-book`) and verify it shows up in `pithos ls`.
- Delete a book directory and run `pithos ls` to verify it gets pruned from the global registry.

