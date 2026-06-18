# plan: Task 5.14: Bubbletea TUI-Based Interactive Review Loop

**Status:** Open
**Go Version:** 1.26.4

Replace the raw markdown-file-editing review loop with an interactive terminal-based review dashboard using the Bubbletea library. This enables developers and creators to directly approve pages, edit stanzas and illustration prompts, edit the global style guide/character seed description, request selective or global page regeneration, and monitor image downloading progress in real-time without leaving the terminal process.

## User Review Required

> [!NOTE]
> The TUI dashboard will be optional and triggered via a CLI flag or interactive launch. The fallback to the markdown-file review loop (`manuscript.md`) will remain fully supported for text-editor-only environments.

## Proposed Changes

### Dependencies

#### [MODIFY] [go.mod](file://../../go.mod)
- Add Bubbletea and related Lipgloss libraries for terminal UI formatting:
  ```go
  github.com/charmbracelet/bubbletea v1.1.0
  github.com/charmbracelet/lipgloss v0.13.0
  github.com/charmbracelet/bubbles v0.20.0
  ```

### CLI Command Layer

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add a new `--tui` flag to the `brew` command.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Add `TUI bool` to `BrewOptions`.
- If `TUI` is requested during `brew` and review checkpoint is active:
  - Initialize the Bubbletea program with the manuscript pages state and the global style/character seed.
  - Render a dashboard view containing:
    * A list of all manuscript pages.
    * A dedicated **Global Style / Character Seed** panel.
    * Panels showing current stanza text and expanded illustration prompt.
    * Keystroke actions: `e` to edit page text/prompt, `s` to edit global style/character seed description, `r` to trigger selective page redo, `v` or Arrow Keys to cycle/select active image variations, `a` to approve, `q` to quit.
  - If a user requests a redo on a page (`r` key):
    * Reset the page status to pending.
    * Dynamically spin up the image generator and render a progress bar on the dashboard.
  - If a user edits the global style (`s` key):
    * Open a text input prompt to edit the style string.
    * On save, update `m.BookProperties.Style`, reset **all** pages to pending, clear all image paths, and trigger illustration regeneration.
  - If a user cycles image variations (`v` or Arrow Keys):
    * Update the page's active `ImagePath` and `SelectedModel` in the manifest.
    * Copy the selected variation candidate file to `images/page_<index>.png` to update terminal rendering.
  - **Live Web Preview Sync**: On any state change (editing stanzas, changing image variations, style updates), automatically call `GenerateWebPreview` to regenerate `data.js` in the preview workspace, allowing live reloading in the browser.

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Mock Bubbletea model structures and verify updates to the manifest state based on mocked TUI keystroke events.

### Manual Verification
1. Run Pithos with the TUI flag:
   ```bash
   ./bin/pithos brew --output tui-test --review --tui
   ```
2. Interact with the TUI to edit a stanza prompt, trigger page regeneration, and verify manifest.json state changes.
3. Edit the global style in the TUI, confirm it triggers a reset on all pages and starts regenerating illustrations.
