# plan: Task 5.39: Hot-Reloading Workspace File Watcher

**Status:** Completed (Issue #62)
**Date Completed:** 2026-07-13
**Unit Test Coverage:** 91.1%
**Go Version:** 1.26.5

This task implements a `pithos preview --watch` command that listens for edits to local manuscript draft files or configurations, automatically compiles the changes via the Typst pipeline, and updates the browser visualizer in real-time.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [preview.go](file://../../cmd/pithos/preview.go)
- Create a new Cobra command `previewCmd` for `pithos preview <book>`.
- Supports the flag `--watch`.
- Launches a directory file watcher that stays alive in the foreground.

### Directory Watcher

#### [NEW] [watcher.go](file://../../internal/pipeline/watcher.go)
- Integrate `github.com/fsnotify/fsnotify`.
- Monitor the book's workspace folder for changes to:
  - `manuscript.md`
  - `.pithos.toml` (inside the workspace)
- On change detection:
  - Re-trigger the import parser to update stanzas in `manifest.json`.
  - Re-run `GenerateWebPreview` to refresh the visualizer's `data.js` database.
  - Re-compile the Typst PDF layout (if `pw-mcp-typst` path is configured).
  - Print a styled Lipgloss alert showing that recompilation succeeded.

---

## Verification Plan

### Automated Tests
- Test that modifying files triggers the fsnotify handlers in isolation.
- Ensure file watchers are cleanly closed when the context is cancelled.

### Manual Verification
- Run `pithos preview book_name --watch`.
- Open the web previewer in the browser.
- Open `manuscript.md` in a separate text editor.
- Make a change to a stanza and save the file.
- Verify the terminal prints the recompilation log and the browser preview is updated upon refreshing.
