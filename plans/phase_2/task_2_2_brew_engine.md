# plan: Task 2.2: The `initiate` & `brew` Engine

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-11
**Unit Test Coverage:** 92.3% (meets strict >91% threshold requirement)

Implement the logic for the `initiate` (workspace setup) and `brew` (manuscript & image generation) commands, orchestrating LLM text generation and MCP image generation.

## Implementation Details

### LLM Integration
- Directly consumes Gemini/OpenAI HTTP REST API endpoints (`/v1beta/models/gemini-1.5-flash:generateContent` and `/v1/chat/completions` respectively) via Go's standard `net/http` package to avoid heavyweight third-party SDK dependencies.
- Restricts responses to structured JSON outputs conforming to a specific schema containing a list of parodic stanzas.

### MCP Client Lifecycle & Resume Protection
- Integrates the MCP Client from Task 2.1 to launch, communicate, and shut down MCP servers (`pw-mcp-imagegen`).
- Implements `needsIllustrationGen` scan logic before starting the MCP client to verify if any illustrations are actually needed. If a pipeline run is resumed and all images are already completed, the MCP process is completely skipped to conserve resources and avoid startup latency.
- Implements style registration by invoking the `imagegen_register_style` MCP tool with Midjourney style reference (`--sref`) links/prompts parsed from the command arguments.

### File Operations
- Copies generated assets from the MCP output paths to the workspace's local `/images` directory using a clean, custom `copyFile` helper.
- Standardizes on standard Go `io.Copy` combined with `os.OpenFile` (with `gosec` G302/G304 rule exclusions explicitly audited and documented).

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Pipeline Core

#### [NEW] [initiate.go](file://../../internal/pipeline/initiate.go)
- [x] Implement command execution logic to scaffold a new book directory structure under the specified path.
- [x] Create subdirectories for assets (`/images`) and final output (`/release`).
- [x] Write an initial, empty `manifest.json` file defining book properties (theme, style, format, target page count) and structure template.

#### [NEW] [brew.go](file://../../internal/pipeline/brew.go)
- [x] Parse `--theme`, `--style`, and `--output` directory paths.
- [x] Load the existing `manifest.json` state from the target directory. If it is a resumed run, detect what steps are already completed.
- [x] If manuscript text is not yet generated:
  - [x] Query the LLM provider (Gemini/GPT) to write a parodic poem in strict rhythmic meter.
  - [x] Parse stanzas and record them as raw text pages inside the `manifest.json` state file. Save state immediately.
- [x] Iterate through pages in the manifest:
  - [x] If a page illustration is already successfully generated, skip it.
  - [x] Otherwise, query `pw-mcp-imagegen` to generate the illustration, enforcing style consistency via `--sref` arguments.
  - [x] Save the generated image files locally under the `/images` folder, and write the path reference into `manifest.json` immediately.
- [x] Ensure that failures at any step gracefully save the current `manifest.json` so the pipeline can resume execution directly from the failure checkpoint.

#### [NEW] [llm.go](file://../../internal/pipeline/llm.go)
- [x] Implement Gemini & OpenAI direct HTTP/REST client integrations.

#### [MODIFY] [main.go](file://../../cmd/pithos/main.go) / [initiate.go](file://../../cmd/pithos/initiate.go) / [brew.go](file://../../cmd/pithos/brew.go)
- [x] Wire up the command routing from Cobra subcommands to point to these new functions.

---

## Verification Plan

### Automated Tests
- [x] Run command: `make check-coverage` (or `go test -v -race -cover ./...`)
- [x] Unit test the manifest parsing, incremental state updating, and resume-checking logic.
- [x] Unit test the manuscript parser.
- [x] Unit test direct Gemini & OpenAI client wrappers using `httptest.NewServer`.

### Manual Verification
- [x] Trigger `pithos initiate` to verify directory layout and initial state manifest generation.
- [x] Run a dry-run of `pithos brew` using mock LLM and MCP responses. Kill the process halfway through page image generation, and verify that rerunning it successfully skips already-generated pages.

