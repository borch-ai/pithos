# plan: Task 8.10: Interactive Style Revision & Diff Reviewer

**Status:** Open
**Go Version:** 1.26.4

Implement an interactive revision review loop for the terminal. When the Editorial Style Critic (Task 6.9) suggests automated style or grammar fixes on chapter drafts, Pithos will present a colored, line-by-line diff of the proposed modifications and allow the author to interactively accept, reject, or manually edit each revision.

## User Review Required

> [!NOTE]
> **Minimal Runtime Requirements**:
> To ensure compatibility across ssh sessions and various terminal emulators, the reviewer will use standard terminal I/O and ANSI color codes rather than heavy full-screen TUI libraries.

## Proposed Changes

### CLI Command Layer

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add `--interactive` flag to `brew` command to enable interactive critique reviews:
  ```bash
  pithos brew --output [book_dir] --interactive
  ```

---

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Modify the review checkpoint logic to detect if `--interactive` is enabled.
- If enabled, fetch the suggested changes from the critic runner, compute the differences, and invoke the interactive reviewer.

#### [NEW] [diff.go](file://../../internal/review/diff.go)
- Implement a line-by-line diffing helper (e.g., Myers diff algorithm or a simple LCS line matcher) to compute additions and deletions between the original section text and the critic's proposed text.
- Implement `ReviewDiffs(originalText, proposedText string) (acceptedText string, changed bool, err error)`:
  - Print the diff lines to the terminal:
    * Deletions: Red (`\033[31m- ...\033[0m`)
    * Additions: Green (`\033[32m+ ...\033[0m`)
    * Unchanged: Normal (`  ...`)
  - Prompt the user:
    `[a] Accept this change, [r] Reject and keep original, [e] Open default terminal editor, [q] Quit and resume later: `
  - Process input:
    * `a`: Accept and return `proposedText` as `acceptedText`.
    * `r`: Reject and return `originalText`.
    * `e`: Detect terminal editor command (e.g. from `$EDITOR` environment variable, falling back to `vi` or `nano`), save proposed text to a temporary file, launch the editor subcommand, wait for exit, read the edited file, and return it.
    * `q`: Exit review loop immediately and pause the pipeline.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  - Diff matcher correctly identifies edits (additions/deletions) between strings.
  - Command execution is properly mocked when invoking terminal editors.
  - Stream inputs (`io.Reader`) are correctly read to simulate user keypresses (`a`, `r`, `e`, `q`).

### Manual Verification
1. Run a test book build with `--interactive`:
   ```bash
   ./bin/pithos brew --output interactive-test --interactive
   ```
2. Wait for the style critique to complete.
3. Verify that the terminal shows a clear red/green line diff.
4. Press `a` to accept, and verify that the manifest/manuscript is updated with the accepted revisions.
5. Press `e` to verify it opens the configured terminal text editor.
