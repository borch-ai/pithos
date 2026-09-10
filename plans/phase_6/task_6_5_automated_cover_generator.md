# plan: Task 6.5: Automated Cover Art & Title Layout Generator

**Status:** Complete
**Go Version:** 1.26+

Automated parodic book cover generation by creating a unified workflow that queries the LLM for cover art ideas, generates a high-resolution cover illustration matching the style guide, compiles KDP geometry specifications (margins, bleed, spine width), and produces a print-ready PDF cover wrap.

## Summary of Implementation

1. **LLM Cover Design Generation**:
   - Added `CoverDesign` struct containing `Title`, `Subtitle`, `Author`, `BackCoverBlurb`, and `CoverPrompt`.
   - Added `GenerateCoverDesign` to `LLMClient` interface and implemented structured JSON extraction in `PowerwordClientAdapter`.
   - Integrated optional `--title` and `--author` CLI flags in `cmd/pithos initiate` and `InitiateOptions` to allow explicit overrides.
   - In `generateManuscript` (`brew`), automatically generates missing title, subtitle, author, back-cover blurb, and cover illustration prompt via the LLM adapter (or deterministic mocks for simulated/dry-run modes).

2. **Cover Art Generation in Brew**:
   - Extended `generateIllustrations` in `internal/pipeline/brew.go` with `needCover` logic.
   - When cover generation is needed, generates `images/cover.png` using the active MCP `PluginImageGen` client and registered style profile.
   - Reuses active MCP client sessions, preventing redundant socket reconnections or EOF errors on in-memory test transports.
   - Fully supports `--dry-run` simulation mode with dummy PNG generation and telemetry tracking.

3. **KDP Full Cover Wrap PDF Compilation in Assemble**:
   - Added `compileCoverPDF` in `internal/pipeline/assemble.go` invoking the `compile_cover` tool on `pw-mcp-typst`.
   - Passes KDP spine width, bleed, trim size, paper type, front cover image path, title, subtitle, author, and back cover blurb.
   - Records the compiled cover wrap PDF in `AssetRegistry["cover_pdf"]` and updates `Kiln.CoverPDFPath`.
   - Shared typst client lifecycle between interior and cover PDF compilation in `Assemble`.

4. **Web Preview Cover Wrap View**:
   - Added Cover Wrap tab and spread view in `web_preview/preview.html`.
   - Dynamically visualizes back cover (with back cover blurb), spine (with vertical title), and front cover art (with title/author overlay).
   - Draws visual KDP wrap bleed guidelines and safety margins using geometry values stored in `manifest.json`.

5. **Testing & Coverage**:
   - Enforced strict 91% code coverage check (`make check-coverage`) achieving 91.3% statement coverage.
   - All tests run hermetically and cleanly with 0 lint issues (`make lint`).
