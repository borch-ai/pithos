# plan: Task 5.34: List Local Book Workspaces (pithos ls)

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-22
**Unit Test Coverage:** 94.7%

This task implements a `pithos ls` subcommand to inspect and list book workspaces. By scanning the configured workspace root directory (which defaults to `~/.local/share/pithos/workspaces/`), Pithos will parse the `manifest.json` for each workspace and print a formatted table showing the book's properties, current milestones/progress, and accumulated costs.

## User Review Required

> [!NOTE]
> Adds `github.com/charmbracelet/lipgloss` dependency for styling the CLI table outputs.

## Proposed Changes

### Command Layer

#### [NEW] [ls.go](file://../../cmd/pithos/ls.go)
- Create a new Cobra command `lsCmd` for `pithos ls`.
- Short: "List all book workspaces"
- The command runner should:
  - Resolve the workspace root directory (checking config or dynamic defaults).
  - Call `pipeline.ListWorkspaces(root)` to get the summaries.
  - Format the output using `github.com/charmbracelet/lipgloss/table` styles to display columns:
    `DIRNAME | THEME | FORMAT | PAGES | MILESTONES | COST`

### Pipeline Engine

#### [NEW] [ls.go](file://../../internal/pipeline/ls.go)
- Implement a helper function `ListWorkspaces(workspaceRoot string) ([]BookSummary, error)` that scans a directory, reads `manifest.json` files, and returns parsed summaries.
- Define `BookSummary` struct:
  ```go
  type BookSummary struct {
      DirName         string
      Theme           string
      Format          string
      PageCount       int
      TargetPageCount int
      Milestones      []string
      TotalCost       float64
  }
  ```

---

## Verification Plan

### Automated Tests
- Create unit tests under `internal/pipeline/ls_test.go` verifying that `ListWorkspaces` correctly parses and extracts summaries from a mock directory containing valid/invalid/missing manifests.

### Manual Verification
- Run `pithos ls` and verify books show up in the table with correct metadata.
