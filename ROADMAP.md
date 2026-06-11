# Pithos Roadmap

This roadmap defines the engineering journey to build **Pithos**, the automated dark parody book pipeline.

---

## Phase 1: Foundation
Focus: Bootstrapping the CLI application, establishing the documentation, configuration layer, command structure, and state management.

*   [x] **Task 1.1: Project Initialization & Architecture**
    *   Set up the Go project structure (`cmd/pithos`), Makefile, and linters.
    *   Draft `VISION.md`, `ROADMAP.md`, and `GEMINI.md`.
*   [ ] **Task 1.2: Cobra & Viper CLI Scaffolding**
    *   Refactor monolithic `main.go` to split subcommands into separate files under `cmd/pithos/` (e.g. `initiate.go`, `brew.go`, `assemble.go`, `deploy.go`).
    *   Integrate `spf13/viper` to load configuration details (like server binary paths for Powerword MCP) from a local `.pithos.toml` or global file.
*   [ ] **Task 1.3: Project State & Resumability Manifest**
    *   Define the structure of `manifest.json` (or `state.json`) to serve as a persistent checkpoint state.
    *   Implement read/write/resume routines to allow command executions to run incrementally and recover from interruptions.

---

## Phase 2: Powerword Integration
Focus: Integrating with the `pw-mcp-*` ecosystem to perform core content generation.

*   [ ] **Task 2.1: MCP Client Integration**
    *   Implement the `modelcontextprotocol/go-sdk` to allow Pithos to launch and connect to local Powerword MCP binaries via stdio based on pathways defined in Viper configuration.
    *   [Implementation Plan](plans/task_2_1_mcp_integration.md)
*   [ ] **Task 2.2: The `initiate` & `brew` Engine**
    *   Implement `pithos initiate` to scaffold the project workspace directory and write an empty state manifest.
    *   Implement `pithos brew` to incrementally write/generate thematic poetry using LLMs, and invoke `pw-mcp-imagegen` for page illustrations (supporting `--sref` parameters).
    *   Save generated stanzas and image references directly to the manifest to enable resumption after failure or human approval pauses.
    *   [Implementation Plan](plans/task_2_2_brew_engine.md)

---

## Phase 3: Assembly & Layout
Focus: Turning raw text and image assets into valid, print-ready files.

*   [ ] **Task 3.1: The `assemble` Engine**
    *   Implement hard layout validation checks (such as enforcing the KDP hardcover minimum limit of 75 pages).
    *   Connect to `pw-mcp-kdp-math` to calculate exact PDF geometries (margins, bleed, spine) and record calculations in the manifest.
    *   Generate layout instructions and compile final PDFs locally or prepare manifest for external layout engine consumption.
    *   [Implementation Plan](plans/task_3_1_assemble_engine.md)

---

## Phase 4: Telemetry & Deployment
Focus: Remote monitoring, human-in-the-loop approvals, and Amazon KDP uploads.

*   [ ] **Task 4.1: Lamplighter Integration**
    *   Integrate Firebase/WebRTC signaling to connect the Pithos process to the Lamplighter Android app.
    *   Implement interactive approval checkpoints (e.g., pausing the pipeline to wait for a human to approve the cover art on their phone) backed by state manifest updates.
    *   [Implementation Plan](plans/task_4_1_lamplighter_integration.md)
*   [ ] **Task 4.2: The `deploy` Engine**
    *   Connect to `pw-mcp-seo` for keyword/metadata generation (no direct API scraping inside Pithos).
    *   Connect to `pw-mcp-video` for generating promotional assets.
    *   Package all assets, metadata, and generated files into a unified release zip file for KDP submission.
    *   [Implementation Plan](plans/task_4_2_deploy_engine.md)
