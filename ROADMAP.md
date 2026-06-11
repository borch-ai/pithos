# Pithos Roadmap

This roadmap defines the engineering journey to build **Pithos**, the automated dark parody book pipeline.

---

## Phase 1: Foundation
Focus: Bootstrapping the CLI application, establishing the documentation, configuration layer, command structure, and state management.

*   [x] **Task 1.1: Project Initialization & Architecture**
    *   Set up the Go project structure (`cmd/pithos`), Makefile, and linters.
    *   Draft `VISION.md`, `ROADMAP.md`, and `GEMINI.md`.
*   [x] **Task 1.2: Cobra & Viper CLI Scaffolding**
    *   Refactor monolithic `main.go` to split subcommands into separate files under `cmd/pithos/` (e.g. `initiate.go`, `brew.go`, `assemble.go`, `deploy.go`).
    *   Integrate `spf13/viper` to load configuration details (like server binary paths for Powerword MCP) from a local `.pithos.toml` or global file.
*   [x] **Task 1.3: Project State & Resumability Manifest**
    *   Define the structure of `manifest.json` (or `state.json`) to serve as a persistent checkpoint state.
    *   Implement read/write/resume routines to allow command executions to run incrementally and recover from interruptions.

---

## Phase 2: Powerword Integration
Focus: Integrating with the `pw-mcp-*` ecosystem to perform core content generation.

*   [x] **Task 2.1: MCP Client Integration**
    *   Implement the `modelcontextprotocol/go-sdk` to allow Pithos to launch and connect to local Powerword MCP binaries via stdio based on pathways defined in Viper configuration.
    *   [Implementation Plan](plans/task_2_1_mcp_integration.md)
*   [x] **Task 2.2: The `initiate` & `brew` Engine**
    *   Implement `pithos initiate` to scaffold the project workspace directory and write an empty state manifest.
    *   Implement `pithos brew` to incrementally write/generate thematic poetry using LLMs, and invoke `pw-mcp-imagegen` for page illustrations (supporting `--sref` parameters).
    *   Save generated stanzas and image references directly to the manifest to enable resumption after failure or human approval pauses.
    *   [Implementation Plan](plans/task_2_2_brew_engine.md)

---

## Phase 3: Local Quality Gates & Critic
Focus: Automated review tools, local critic hooks, metric instrumentation, and plan template linting.

*   [x] **Task 3.1: Plan Conformance Verification**
    *   Build a local structure check validator tool to verify all implementation plans under `plans/` conform to the project standard template format.
    *   [Implementation Plan](plans/task_3_1_plan_conformance.md)
*   [x] **Task 3.2: Local Critic Review Subsystem**
    *   Implement a `pithos review` CLI command and local critic engine to verify local Git diffs against proposed implementation plans.
    *   Set up a pre-push Git hook to run local builds, tests, and coverage checks, calling the Critic LLM before code is pushed to remote.
    *   [Implementation Plan](plans/task_3_2_local_critic.md)
*   [ ] **Task 3.3: Token Telemetry & Cost Accounting**
    *   Implement token usage and cost accounting within the LLM client wrapper and the MCP client wrapper.
    *   Save accumulated execution costs to the `manifest.json` state manifest file.
    *   [Implementation Plan](plans/task_3_3_cost_accounting.md)

---

## Phase 4: Assembly & Layout
Focus: Turning raw text and image assets into valid, print-ready files.

*   [x] **Task 4.1: The `assemble` Engine**
    *   Implement hard layout validation checks (such as enforcing the KDP hardcover minimum limit of 75 pages).
    *   Connect to `pw-mcp-kdp-math` to calculate exact PDF geometries (margins, bleed, spine) and record calculations in the manifest.
    *   Generate layout instructions and compile final PDFs locally or prepare manifest for external layout engine consumption.
    *   [Implementation Plan](plans/task_4_1_assemble_engine.md)

---

## Phase 5: Telemetry & Deployment
Focus: Remote monitoring, human-in-the-loop approvals, and Amazon KDP uploads.

*   [ ] **Task 5.1: Lamplighter Integration**
    *   Integrate Firebase/WebRTC signaling to connect the Pithos process to the Lamplighter Android app.
    *   Implement interactive approval checkpoints (e.g., pausing the pipeline to wait for a human to approve the cover art on their phone) backed by state manifest updates.
    *   [Implementation Plan](plans/task_5_1_lamplighter_integration.md)
*   [ ] **Task 5.2: The `deploy` Engine**
    *   Connect to `pw-mcp-seo` for keyword/metadata generation (no direct API scraping inside Pithos).
    *   Connect to `pw-mcp-video` for generating promotional assets.
    *   Package all assets, metadata, and generated files into a unified release zip file for KDP submission.
    *   [Implementation Plan](plans/task_5_2_deploy_engine.md)
*   [ ] **Task 5.3: MCP Telemetry Migration**
    *   Migrate token telemetry and billing calculations from the imported module to a decoupled, external `pw-mcp-telemetry` MCP server.
    *   [Implementation Plan](plans/task_5_3_mcp_telemetry_migration.md)
*   [ ] **Task 5.4: Speculative: Homebrew Formula for Dependency Management**
    *   Migrate dependency installation instructions to rely on Homebrew (`brew`) for installing required MCP plugin servers.
    *   [Implementation Plan](plans/task_5_4_speculative_brew_dependencies.md)
*   [ ] **Task 5.5: Speculative: Publish Pithos as Homebrew Package with Dependencies**
    *   Publish pre-compiled Pithos binaries via Homebrew and list Powerword MCP plugins as package dependencies.
    *   [Implementation Plan](plans/task_5_5_speculative_publish_pithos_brew.md)
