# plan: Task 3.6: Workspace Git Checkpoints

**Status:** Open (Issue #[TBD])

Integrate a local, automated Git checkpoint system into Pithos workspace management, committing state changes and assets automatically so users can roll back files using the CLI.

## User Review Required

> [!WARNING]
> **Git Executable Dependency**:
> This assumes the user has `git` installed in their environment. If `git` is missing, the tool should fail gracefully by outputting a warning and proceeding without checkpointing.

---

## Proposed Changes

### Pipeline Core

#### [NEW] [git.go](../../internal/pipeline/git.go)
- [ ] Implement utility helpers to run git commands (`git init`, `git add`, `git commit`) via `os/exec`.
- [ ] Implement a `Checkpoint(dir string, message string)` function that automatically initializes a git repository if one doesn't exist, adds modified files, and commits them.

#### [MODIFY] [manifest.go](../../internal/manifest/manifest.go)
- [ ] Trigger the Git checkpoint helper after successful manifest updates (such as saving new manuscript text or a new image asset).

#### [NEW] [checkpoint.go](../../cmd/pithos/checkpoint.go)
- [ ] Add a `pithos checkpoint` subcommand to show history and restore the directory to a previous git commit or step.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Test the checkpoint utility inside temporary test directories using a mocked environment.

### Manual Verification
- [ ] Run `pithos brew`, verify a `.git` folder is initialized inside the output folder, and run git commands to confirm incremental commits exist.
