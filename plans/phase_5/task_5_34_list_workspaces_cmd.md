# plan: Task 5.34: List Local Book Workspaces (pithos ls)

**Status:** Proposed
**Go Version:** 1.26.4

This task implements a `pithos ls` subcommand to inspect and list book workspaces. By scanning the configured workspace root directory (which defaults to `~/.local/share/pithos/workspaces/` after Task 5.32 is implemented), Pithos will parse the `manifest.json` for each workspace and print a formatted table showing the book's properties, current milestones/progress, and accumulated costs.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [ls.go](file://../../cmd/pithos/ls.go)
- Create a new Cobra command `lsCmd` for `pithos ls`.
- Short: "List all book workspaces"
- The command runner should:
  - Resolve the workspace root directory (checking config or dynamic defaults).
  - List all child directories.
  - For each directory, check if `manifest.json` exists.
  - Parse the manifest details.
  - Format the output using `github.com/charmbracelet/lipgloss` table styles to display columns:
    `DIRNAME | THEME | FORMAT | PAGES | MILESTONES | COST`

### Pipeline Engine

#### [NEW] [ls.go](file://../../internal/pipeline/ls.go)
- Implement a helper function `ListWorkspaces(workspaceRoot string) ([]BookSummary, error)` that scans a directory, reads `manifest.json` files, and returns parsed summaries.
- Define `BookSummary` struct:
  ```go
  type BookSummary struct {
      DirName     string
      Theme       string
      Format      string
      PageCount   int
      Milestones  []string
      TotalCost   float64
  }
  ```

---

## Verification Plan

### Automated Tests
- Create unit tests under `internal/pipeline/ls_test.go` verifying that `ListWorkspaces` correctly parses and extracts summaries from a mock directory containing valid/invalid/missing manifests.

### Manual Verification
- Run `pithos initiate --output book_a` and `pithos initiate --output book_b`.
- Run `pithos ls` and verify both books show up in the table with correct metadata.
