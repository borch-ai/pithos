# plan: Task 3.6: Workspace Git Checkpoints

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** June 15, 2026
**Unit Test Coverage:** 95.8% (git.go), 91.0% (total internal)

Integrate a local, automated Git checkpoint system into Pithos workspace management, committing state changes and assets automatically so users can roll back files using the CLI. This task consumes the shared `pkg/gitutil` utility package from Powerword to avoid redundant process execution logic.

## User Review Required

> [!WARNING]
> **Git Executable Dependency**:
> This assumes the user has `git` installed in their environment. If `git` is missing, the tool will fail gracefully by outputting a warning and proceeding without checkpointing.

---

## Proposed Changes

### Pipeline Core

#### [NEW] [git.go](file://../../internal/pipeline/git.go)
* Import `github.com/borch-ai/powerword/pkg/gitutil`.
* Implement a `Checkpoint(ctx context.Context, dir string, message string) error` function that:
  1. Checks if the system has git in PATH. If not, outputs a warning and returns `nil`.
  2. Resolves the directory path.
  3. Checks if the directory is inside a git repository via `gitutil.IsInsideWorkTree`.
  4. If not, initializes a git repository using `gitutil.Init`.
  5. Adds modified/untracked files using `gitutil.AddAll`.
  6. Commits changes using `gitutil.Commit` with the milestone message.

#### [MODIFY] [initiate.go](file://../../internal/pipeline/initiate.go)
* Trigger `Checkpoint(ctx, outputDir, "Initial workspace setup")` after setup.

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
* Trigger `Checkpoint(ctx, outputDir, "Generated stanzas and prompts")` after manuscript generation.
* Trigger `Checkpoint(ctx, outputDir, "Imported manuscript edits from review")` after importing edits.
* Trigger `Checkpoint(ctx, outputDir, "Completed illustration generation")` after image generation finishes.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
* Trigger `Checkpoint(ctx, outputDir, "Compiled print layouts and PDFs")` after assemble finishes compiling layouts/PDFs.

### Subcommand Interface

#### [NEW] [checkpoint.go](file://../../cmd/pithos/checkpoint.go)
* Add a `pithos checkpoint` CLI command hierarchy:
  * `pithos checkpoint list`: Runs git log inside the workspace directory to list past checkpoint milestones.
  * `pithos checkpoint restore <commit-hash>`: Performs a hard reset (`gitutil.ResetHard` and `gitutil.Clean`) on the workspace back to the specified checkpoint.

### Tests

#### [NEW] [git_test.go](file://../../internal/pipeline/git_test.go)
* Implement unit tests using mocked git setups to verify that checkpoint operations stage and commit as expected.
* Ensure coverage of `git.go` is fully validated to keep total project coverage `≥91.0%`.

---

## Verification Plan

### Automated Tests
* Run `go test -v ./internal/pipeline/...`
* Run `make check-coverage` to assert overall project statement coverage remains `≥91.0%`.

### Manual Verification
1. Run `pithos initiate books/manual-test-book`.
2. Inspect `books/manual-test-book/.git` and verify initial commit was made.
3. Run `pithos checkpoint list --book books/manual-test-book` to check output format.
4. Run `pithos checkpoint restore <hash> --book books/manual-test-book` to verify rollbacks.
