# plan: Task 5.16: Inline Terminal Graphics Previews in TUI

**Status:** Open
**Go Version:** 1.26.4

Integrate inline terminal graphics rendering (Kitty, iTerm2, and Sixel protocols) inside Pithos's Bubbletea-based review TUI dashboard. This allows creators to see real-time, high-fidelity inline visual previews of the generated/selected page illustrations directly within their terminal window, eliminating the need to toggle to an external image viewer.

## User Review Required

> [!NOTE]
> **Terminal Compatibility Fallbacks**:
> Terminal graphics protocols require terminal emulator support (e.g., Alacritty, iTerm2, Kitty, WezTerm). If the terminal does not support these protocols, the preview panel will gracefully fall back to a block-rendered Unicode/ANSI thumbnail or a descriptive text box.

## Proposed Changes

### Pipeline TUI Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Integrate with Powerword's inline graphics package (from Powerword Task 6.8).
- Update the TUI dashboard layout to include a dedicated `[Illustration Preview]` panel.
- On active page selection:
  - Detect terminal graphics support by querying env vars (like `TERM_PROGRAM`) or sending device attribute query sequences.
  - If support is detected (Kitty or Sixel):
    - Load the active image file (e.g. `images/page_<index>.png`).
    - Downscale the image to fit the panel boundaries (e.g., 200x200 pixels).
    - Convert the image bytes to Kitty or Sixel ESC sequences.
    - Render the graphic sequences inside the Bubbletea view loop.
  - If not supported:
    - Render a colorized ANSI block art preview or write the filename and resolution.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Mock terminal capability detection and verify that graphic conversions are called only when matching capabilities are returned.

### Manual Verification
1. Run the interactive review command in a graphics-supported terminal (e.g., WezTerm or iTerm2):
   ```bash
   ./bin/pithos brew --output terminal-gfx-test --review --tui
   ```
2. Verify the image preview renders correctly inside the Bubbletea dashboard panel.
3. Switch pages and verify that the graphic updates to match the newly selected page stanza.
