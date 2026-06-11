# plan: Task 2.2: The `initiate` & `brew` Engine

**Status:** Open (Issue #[TBD])

Implement the logic for the `initiate` (workspace setup) and `brew` (manuscript & image generation) commands, orchestrating LLM text generation and MCP image generation.

## Proposed Changes

### Pipeline Core

#### [NEW] [initiate.go](file:///Users/human/code/pithos/internal/pipeline/initiate.go)
- [ ] Implement command execution logic to scaffold a new book directory structure under the specified path.
- [ ] Create subdirectories for assets (`/images`) and final output (`/release`).
- [ ] Write an initial, empty `manifest.json` file defining book properties (theme, style, format, target page count) and structure template.

#### [NEW] [brew.go](file:///Users/human/code/pithos/internal/pipeline/brew.go)
- [ ] Parse `--theme`, `--style`, and `--output` directory paths.
- [ ] Load the existing `manifest.json` state from the target directory. If it is a resumed run, detect what steps are already completed.
- [ ] If manuscript text is not yet generated:
  - [ ] Query the LLM provider (Gemini/GPT) to write a 15-page parodic poem in strict rhythmic meter.
  - [ ] Parse stanzas and record them as raw text pages inside the `manifest.json` state file. Save state immediately.
- [ ] Iterate through pages in the manifest:
  - [ ] If a page illustration is already successfully generated, skip it.
  - [ ] Otherwise, query `pw-mcp-imagegen` to generate the illustration, enforcing style consistency via `--sref` arguments.
  - [ ] Save the generated image files locally under the `/images` folder, and write the path reference into `manifest.json` immediately.
- [ ] Ensure that failures at any step gracefully save the current `manifest.json` so the pipeline can resume execution directly from the failure checkpoint.

#### [MODIFY] [main.go](file:///Users/human/code/pithos/cmd/pithos/main.go)
- [ ] Wire up the command routing from Cobra subcommands to point to these new functions.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/pipeline/...`
- [ ] Unit test the manifest parsing, incremental state updating, and resume-checking logic.
- [ ] Unit test the manuscript parser.

### Manual Verification
- [ ] Trigger `pithos initiate` to verify directory layout and initial state manifest generation.
- [ ] Run a dry-run of `pithos brew` using mock LLM and MCP responses. Kill the process halfway through page image generation, and verify that rerunning it successfully skips already-generated pages.

