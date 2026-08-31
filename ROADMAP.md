# Pithos Roadmap

This roadmap defines the engineering journey to build **Pithos**, the automated dark parody book pipeline.

---

## Ecosystem Context

Pithos is the **book factory** of the Borch-AI publishing stack. It is a deterministic pipeline, not a free-form agent.

- **Upstream**: **Kiln** orchestrates Pithos as a subprocess (`kiln forge`). Kiln supplies the validated niche concept; Pithos executes production.
- **Downstream**: Pithos invokes `pw-mcp-imagegen`, `pw-mcp-kdp-math`, `pw-mcp-typst` (pending) from the Powerword MCP plugin network.
- **Monitoring**: **Lamplighter** (Pithos Phase 5.1) provides mobile approval checkpoints during long brew/assemble runs.
- **Telemetry**: **Lighthouse** receives pipeline execution costs, token counts, and stage durations via the shared `pkg/telemetry` HTTP adapter (Powerword Task 4.16). Opt-in via `LIGHTHOUSE_URL`.
- **Sister Factory**: **Aeolian** operates as a parallel factory for the music segment; Pithos remains strictly focused on books, preserving decoupling.

> [!NOTE]
> **Core Deliverables Achieved**: Both Task 4.2 (Typst PDF Layout Assembly) and Task 5.20 (Kiln Foundry State Integration) have shipped. Phase 6 work is now defrosted and open for development.

---

## Phase 1: Foundation
Focus: Bootstrapping the CLI application, establishing the documentation, configuration layer, command structure, and state management.

*   [x] **Task 1.1: Project Initialization & Architecture**
    *   Set up the Go project structure (`cmd/pithos`), Makefile, and linters.
    *   Draft `VISION.md`, `ROADMAP.md`, and `GEMINI.md`.
    *   [Implementation Plan](plans/phase_1/task_1_1_project_initialization.md)
*   [x] **Task 1.2: Cobra & Viper CLI Scaffolding**
    *   Refactor monolithic `main.go` to split subcommands into separate files under `cmd/pithos/` (e.g. `initiate.go`, `brew.go`, `assemble.go`, `deploy.go`).
    *   Integrate `spf13/viper` to load configuration details (like server binary paths for Powerword MCP) from a local `.pithos.toml` or global file.
    *   [Implementation Plan](plans/phase_1/task_1_2_cobra_viper_scaffolding.md)
*   [x] **Task 1.3: Project State & Resumability Manifest**
    *   Define the structure of `manifest.json` (or `state.json`) to serve as a persistent checkpoint state.
    *   Implement read/write/resume routines to allow command executions to run incrementally and recover from interruptions.
    *   [Implementation Plan](plans/phase_1/task_1_3_manifest_state.md)

---

## Phase 2: Powerword Integration
Focus: Integrating with the `pw-mcp-*` ecosystem to perform core content generation.

*   [x] **Task 2.1: MCP Client Integration**
    *   Implement the `modelcontextprotocol/go-sdk` to allow Pithos to launch and connect to local Powerword MCP binaries via stdio based on pathways defined in Viper configuration.
    *   [Implementation Plan](plans/phase_2/task_2_1_mcp_integration.md)
*   [x] **Task 2.2: The `initiate` & `brew` Engine**
    *   Implement `pithos initiate` to scaffold the project workspace directory and write an empty state manifest.
    *   Implement `pithos brew` to incrementally write/generate thematic poetry using LLMs, and invoke `pw-mcp-imagegen` for page illustrations (supporting `--sref` parameters).
    *   Save generated stanzas and image references directly to the manifest to enable resumption after failure or human approval pauses.
    *   [Implementation Plan](plans/phase_2/task_2_2_brew_engine.md)
*   [x] **Task 2.3: Parallel Asset Generation**
    *   Implement concurrent image generation workers in the Pithos brew engine, allowing multiple page illustrations to be requested and downloaded in parallel.
    *   [Implementation Plan](plans/phase_2/task_2_3_parallel_asset_generation.md)

---

## Phase 3: Local Quality Gates & Critic
Focus: Automated review tools, local critic hooks, metric instrumentation, and plan template linting.

*   [x] **Task 3.1: Plan Conformance Verification**
    *   Build a local structure check validator tool to verify all implementation plans under `plans/` conform to the project standard template format.
    *   [Implementation Plan](plans/phase_3/task_3_1_plan_conformance.md)
*   [x] **Task 3.2: Local Critic Review Subsystem**
    *   Implement a `pithos review` CLI command and local critic engine to verify local Git diffs against proposed implementation plans.
    *   Set up a pre-push Git hook to run local builds, tests, and coverage checks, calling the Critic LLM before code is pushed to remote.
    *   [Implementation Plan](plans/phase_3/task_3_2_local_critic.md)
*   [x] **Task 3.3: Token Telemetry & Cost Accounting**
    *   Implement token usage and cost accounting within the LLM client wrapper and the MCP client wrapper.
    *   Save accumulated execution costs to the `manifest.json` state manifest file.
    *   [Implementation Plan](plans/phase_3/task_3_3_cost_accounting.md)
*   [x] **Task 3.4: Interactive Local Review Mode**
    *   Implement an interactive mode that pauses execution after manuscript generation to let the user approve, edit, or regenerate generated stanzas in the terminal.
    *   [Implementation Plan](plans/phase_3/task_3_4_interactive_local_review.md)
*   [x] **Task 3.5: Selective Page Redo / Overrides**
    *   Support selective regeneration of specific pages or stanzas in the `brew` command, allowing users to rerun the pipeline for only a subset of pages.
    *   [Implementation Plan](plans/phase_3/task_3_5_selective_page_redo.md)
*   [x] **Task 3.6: Workspace Git Checkpoints**
    *   Integrate a local, automated Git checkpoint system into Pithos workspace management, committing state changes and assets automatically using the shared `pkg/gitutil` package from Powerword.
    *   [Implementation Plan](plans/phase_3/task_3_6_workspace_git_checkpoints.md)


---

## Phase 4: Assembly & Layout
Focus: Turning raw text and image assets into valid, print-ready files.

*   [x] **Task 4.1: The `assemble` Engine**
    *   Implement hard layout validation checks (such as enforcing the KDP hardcover minimum limit of 75 pages).
    *   Connect to `pw-mcp-kdp-math` to calculate exact PDF geometries (margins, bleed, spine) and record calculations in the manifest.
    *   Generate layout instructions and compile final PDFs locally or prepare manifest for external layout engine consumption.
    *   [Implementation Plan](plans/phase_4/task_4_1_assemble_engine.md)
*   [x] **Task 4.2: Typst PDF Layout Assembly**
    *   Integrate with a standalone `pw-mcp-typst` MCP plugin to compile the book manuscript stanzas and generated page illustrations into a print-ready PDF file.
    *   [Implementation Plan](plans/phase_4/task_4_2_typst_pdf_assembly.md)
*   [x] **Task 4.3: Verse Layout Formatting**
    *   Format stanza line breaks in Typst layouts instead of collapsing them into paragraphs.
    *   [Implementation Plan](plans/phase_4/task_4_3_verse_layout_formatting.md)
*   [x] **Task 4.4: Mixed Layout Templates**
    *   Support left-page text / right-page image mixed layouts per page in the layout engine.
    *   [Implementation Plan](plans/phase_4/task_4_4_mixed_layout_templates.md)
*   [x] **Task 4.5: Print-Ready PDF Preflight Validation (`pw-mcp-pdfcheck`)**
    *   After compiling the final interior PDF, invoke the `pw-mcp-pdfcheck` plugin to run automated preflight checks (validation of PDF page geometry, margins, safe zones, embedded fonts, image DPI, and grayscale color space). Fail the assembly pipeline if critical errors are found.
    *   [Implementation Plan](plans/phase_4/task_4_5_pdfcheck_integration.md)

---

## Phase 5: Pipeline Hardening & Ecosystem Integration
Focus: Remote monitoring, Kiln state manifest integration, testing, and ecosystem tooling.

### Core Pipeline & Kiln Integration
*   [ ] **Task 5.1: Lamplighter Integration**
    *   Integrate Firebase/WebRTC signaling to connect the Pithos process to the Lamplighter Android app.
    *   Implement interactive approval checkpoints (e.g., pausing the pipeline to wait for a human to approve the cover art on their phone) backed by state manifest updates.
    *   [Implementation Plan](plans/phase_5/task_5_1_lamplighter_integration.md)
*   [x] **Task 5.20: Kiln Foundry State Integration**
    *   Implement the state contract by writing pipeline milestone updates (`initiate_complete`, `brew_complete`, `assemble_complete`) and accumulated costs to the Pithos workspace manifest (`manifest.json`), which Kiln reads to sync book status.
    *   [Implementation Plan](plans/phase_5/task_5_20_kiln_integration.md)
*   [x] **Task 5.21: Character Invariant Injection**
    *   Inject visual invariant descriptors to preserve character consistency in generated illustration prompts.
    *   [Implementation Plan](plans/phase_5/task_5_21_character_invariant_injection.md)
*   [x] **Task 5.22: Preflight Diagnostic Subcommand (`pithos doctor`)**
    *   Implement a `doctor` subcommand to run connectivity, credential, and backend health checks for all configured MCP plugins.
    *   [Implementation Plan](plans/phase_5/task_5_22_preflight_diagnostics_doctor.md)
*   [x] **Task 5.23: Dynamic Image Model Discovery**
    *   Query model capability list dynamically from provider APIs to select the best available model version.
    *   [Implementation Plan](plans/phase_5/task_5_23_dynamic_image_model_discovery.md)
*   [x] **Task 5.24: Graceful Image Backend Fallbacks**
    *   Implement fallback handling from OpenAI to Gemini Imagen (and vice-versa) when image generation fails due to credential or permission errors.
    *   [Implementation Plan](plans/phase_5/task_5_24_graceful_image_backend_fallbacks.md)
*   [x] **Task 5.25: Opt-out Brainstorming during Initiate**
    *   Automatically query the LLM to generate the book style (`book_properties.style`) and character profile immediately during `pithos initiate` if a theme is provided. Provide a `--no-brainstorm` flag to opt out of this behavior.
    *   [Implementation Plan](plans/phase_5/task_5_25_opt_out_initiate_brainstorming.md)
*   [x] **Task 5.26: Automatic Browser Preview Opening**
    *   Automatically open the generated web preview visualizer in the default system browser after `pithos brew` or `pithos assemble` completes. Provide a `--silent` flag to allow opting out of this behavior in headless/CI environments.
    *   [Implementation Plan](plans/phase_5/task_5_26_auto_open_preview.md)
*   [x] **Task 5.27: GCP Cloud Storage Configuration Support**
    *   Enable GCS configuration natively in Pithos via `.pithos.toml` and `.env`, and propagate variables to `pw-mcp-cloud` to support character-consistent illustration generation.
    *   [Implementation Plan](plans/phase_5/task_5_27_gcp_cloud_storage_configuration.md)
*   [x] **Task 5.28: Provision GCP Cloud Storage & IAM**
    *   Provision the GCS storage bucket with public-read object access, configure a dedicated GCP Service Account with minimal permissions, and export the credentials JSON file.
    *   [Implementation Plan](plans/phase_5/task_5_28_gcp_infrastructure_provisioning.md)
*   [x] **Task 5.29: Imagegen Backend Capability Validation**
    *   Query the `pw-mcp-imagegen` server's capabilities handshake before starting image generation. If the active backend (e.g. Google Imagen or OpenAI) does not support image-based character references (cref) but a character profile is defined, fail fast with a descriptive error to prevent API cost waste.
    *   [Implementation Plan](plans/phase_5/task_5_29_imagegen_capability_validation.md)
*   [x] **Task 5.30: Preflight Diagnostics Doctor Extensions**
    *   Extend the `pithos doctor` diagnostic checklist to verify the presence, configuration, and handshake success of the `pw-mcp-pdfcheck` server. Check and warn if manual overrides (`force_cref`, `force_sref`) are configured.
    *   [Implementation Plan](plans/phase_5/task_5_30_doctor_pdfcheck_check.md)
*   [x] **Task 5.31: Imagegen Capability Overrides Configuration**
    *   Add configuration keys (`force_cref` and `force_sref`) in `.pithos.toml` and `.env` / environment variables. Parse them in config and propagate them or bypass capability checks in Pithos to allow manual capability overrides.
    *   [Implementation Plan](plans/phase_5/task_5_31_imagegen_capability_overrides.md)
*   [x] **Task 5.32: Adopt Local Workspaces & Cache Structure**
    *   Migrate default output and cache paths to use a dynamic home directory-based structure (`~/.local/share/pithos/`).
    *   Update config defaults and path resolution logic in Pithos pipeline to resolve relative paths under `~/.local/share/pithos/workspaces/`.
    *   [Implementation Plan](plans/phase_5/task_5_32_local_workspaces_structure.md)
*   [x] **Task 5.33: Interactive CLI Prompt Wizards via Huh**
    *   Integrate `github.com/charmbracelet/huh` (Charm-native interactive form library, built on Bubbletea) for interactive user prompts, replacing the previously planned `AlecAivazis/survey` dependency.
    *   Implement an interactive scaffolding wizard for `pithos initiate` when flags are omitted, using `huh.Form` with `NewSelect`, `NewInput`, and `NewConfirm` components.
    *   Implement interactive page multi-selection for selective redo in `pithos brew --pages` via `huh.NewMultiSelect`.
    *   [Implementation Plan](plans/phase_5/task_5_33_interactive_huh_prompts.md)
*   [x] **Task 5.34: List Local Book Workspaces (pithos ls)**
    *   Implement a `pithos ls` CLI command to list all books in local workspace directories.
    *   Scan the workspace root directory, locate subdirectories containing `manifest.json`, and parse book details (Theme, Format, Milestones, and Cost).
    *   Display the compiled book list in a clean, formatted table styled with `charmbracelet/lipgloss`.
    *   [Implementation Plan](plans/phase_5/task_5_34_list_workspaces_cmd.md)
*   [x] **Task 5.35: CLI Visual Styling & Diagnostics Formatting via Lipgloss**
    *   Integrate `github.com/charmbracelet/lipgloss` styling framework.
    *   Format `pithos doctor` connectivity check list with colored success/error indicator badges.
    *   Style terminal summary cards (telemetry, token counts, and billable USD cost summaries) at the end of `brew` and `assemble` runs.
    *   [Implementation Plan](plans/phase_5/task_5_35_cli_styling_lipgloss.md)
*   [x] **Task 5.36: Run Budget Limits & Cost Guardrails**
    *   Add `--budget` CLI flag and `max_cost_usd` Viper config option.
    *   Implement pre-generation cost estimates and halt pipeline execution if expected costs exceed budget constraints.
    *   [Implementation Plan](plans/phase_5/task_5_36_cost_guardrails.md)
*   [x] **Task 5.37: E2E Pipeline Simulation & Dry-Run Mode**
    *   Implement a `--dry-run` CLI flag to bypass live API and MCP invocations.
    *   Mock LLM/image/video generator pipeline stages to verify structural commands, pathing, Typst compilation layouts, and preview generation logic.
    *   [Implementation Plan](plans/phase_5/task_5_37_dry_run_simulation.md)
*   [x] **Task 5.38: Workspace Status Diagnostics & State Repair Utility**
    *   Implement a `pithos status <book>` subcommand to print a formatted summary of book manifest checkpoints and completion states.
    *   Implement a `pithos clean <book>` subcommand to clean up orphaned assets or reset selected page errors in `manifest.json`.
    *   [Implementation Plan](plans/phase_5/task_5_38_workspace_repair_status.md)

### Tooling, Testing & DevOps
*   [x] **Task 5.40: E2E CLI Subprocess Integration Test Suite**
    *   Implement end-to-end integration tests that build the `pithos` binary on-the-fly and execute subprocess CLI commands.
    *   Assert correct exit codes, stdout/stderr formatting, flag parsing, and interactive stdin prompt responses.
    *   [Implementation Plan](plans/phase_5/task_5_40_e2e_cli_subprocess_tests.md)
*   [ ] **Task 5.43: Developer Auto-Setup Command (pithos setup)**
    *   Automate discovery, compilation, and symlinking of local Powerword MCP binaries to simplify workspace onboarding. Fallback to downloading precompiled binaries from GitHub Releases if sibling repository is missing.
    *   [Implementation Plan](plans/phase_5/task_5_43_developer_setup.md)
*   [ ] **Task 5.50: Automated CI/CD & GoReleaser Pipeline**
    *   Set up GitHub Actions to run the test suite and enforce coverage on pull requests.
    *   Configure GoReleaser to automatically cross-compile the Pithos binary for macOS, Linux, and Windows and publish GitHub Releases on tag.
    *   [Implementation Plan](plans/phase_5/task_5_50_automated_ci_cd.md)
*   [ ] **Task 5.39: Hot-Reloading Workspace File Watcher**
    *   Implement a `pithos preview --watch` command using `fsnotify` to monitor local manuscript or config updates.
    *   Automatically trigger Typst recompilation and web preview regenerations on file saves.
    *   [Implementation Plan](plans/phase_5/task_5_39_preview_live_watcher.md)
*   [ ] **Task 5.42: Lighthouse Telemetry Integration**
    *   Confirm that Pithos sends pipeline execution metrics to Lighthouse via the `pkg/telemetry` HTTP adapter (Powerword Task 4.16). Verify that `brew` and `assemble` stage completions produce correct `SystemTelemetry` records (`project="pithos"`, `command=<stage>`, duration, cost, tokens). Add integration test assertions. Blocked on Powerword Task 4.16.
    *   [Implementation Plan](plans/phase_5/task_5_42_lighthouse_telemetry.md)
*   [x] **Task 5.41: Character Seed Generation and Review Command**
    *   Implement a dedicated `pithos character` subcommand to generate/regenerate the main character reference seed portrait based on the manifest profile.
    *   Integrate visual seed validation and cloud storage upload prior to executing brew page illustration jobs.
    *   [Implementation Plan](plans/phase_5/task_5_41_character_seed_review.md)
*   [x] **Task 5.44: Manifest Versioning & Migration Guardrails**
    *   Introduce schema version tracking in `manifest.json` and automatic structure migrations on load to prevent breaking Kiln or Lamplighter.
    *   [Implementation Plan](plans/phase_5/task_5_44_manifest_migrations.md)
*   [x] **Task 5.45: Shell Autocompletion Support (Zsh/Oh-My-Zsh)**
    *   Implement Cobra autocompletion subcommands and automate Zsh/Oh-My-Zsh completion script setup via the `completion` command's `--install` flag.
    *   [Implementation Plan](plans/phase_5/task_5_45_shell_completion.md)
*   [x] **Task 5.46: Structured Pipeline Logging via charmbracelet/log**
    *   Integrate `github.com/charmbracelet/log` to replace raw `fmt.Printf` pipeline output with levelled, color-coded, structured log lines.
    *   Configure log level at root command level (`--debug` flag), with automatic plain-text fallback for non-TTY/CI environments.
    *   Emit structured key-value fields (page, model, cost) alongside human-readable messages during `brew`, `assemble`, and `initiate` pipeline stages.
    *   [Implementation Plan](plans/phase_5/task_5_46_structured_logging_charm.md)
*   [ ] **Task 5.47: Workspace Cache Management Command (pithos cache)**
    *   Add a `cache` subcommand with `status` and `prune` actions to inspect and clean local workspace files.
    *   [Implementation Plan](plans/phase_5/task_5_47_cache_management.md)
*   [ ] **Task 5.48: Aggregated Telemetry Reporting Command (pithos telemetry)**
    *   Add a `telemetry` subcommand to summarize historical model usage, token count, and total USD cost across all book workspaces.
    *   [Implementation Plan](plans/phase_5/task_5_48_aggregated_telemetry.md)
*   [ ] **Task 5.49: Doctor System Dependency Diagnostics**
    *   Extend `pithos doctor` to check for essential external binaries (ffmpeg, typst) and verify Service Account credential / cloud storage permissions.
    *   [Implementation Plan](plans/phase_5/task_5_49_doctor_dependency_diagnostics.md)

---

## Phase 6: Interactive Terminal UX
Focus: Enhancing the developer and author experience with rich TUI (Terminal User Interface) reviews, inline graphics, and multi-model fallbacks.

*   [x] **Task 6.10: Visual Prompt Expansion for Character Consistency** (Formerly Task 5.10)
    *   Modify stanzas generation to produce parodic poems alongside character-consistent illustration prompts. Integrate prompts in the manifest and the markdown review loop.
*   [x] **Task 6.11: LLM-Driven Style & Character Seeds** (Formerly Task 5.11)
    *   Implement automated visual style and character profile generation as a dedicated preceding step of `brew`.
    *   Feed the generated style/character seed into the manuscript generator as prompt context to ensure stanzas and illustration prompts align.
    *   Configure a global character style reference flag (`--style`) to allow manual override.
*   [x] **Task 6.12: Robust JSON Output Parsing** (Formerly Task 5.12)
    *   Implement an LLM response sanitization helper to strip markdown code blocks (e.g. ` ```json ... ``` `) and protect Pithos against parsing errors.
*   [x] **Task 6.19: Interactive Web-Based Book Preview (HTML/CSS)** (Formerly Task 5.19)
    *   Generate a static web-based preview folder containing an interactive flipbook player to visually review books locally in any browser.

*   [x] **Task 6.1: Bubbletea TUI-Based Interactive Review Loop**
    *   Replace the raw file-editing loop with an interactive terminal review dashboard, enabling users to edit stanzas, customize prompts, and trigger select regeneration.
    *   [Implementation Plan](plans/phase_6/task_6_1_bubbletea_tui_review.md)
*   [ ] **Task 6.2: Multi-Model Illustration Variations & Selection**
    *   Generate illustration variations in parallel using multiple configured image models.
    *   Support manual variation selection via markdown reviews and hotkeys in the interactive TUI dashboard.
    *   [Implementation Plan](plans/phase_6/task_6_2_multi_model_image_variations.md)
*   [ ] **Task 6.3: Inline Terminal Graphics Previews in TUI**
    *   Integrate terminal image rendering protocols (Kitty, Sixel) within the Bubbletea review TUI to display visual illustration previews directly in the console.
    *   [Implementation Plan](plans/phase_6/task_6_3_inline_terminal_previews.md)
*   [ ] **Task 6.4: LLM-Driven Stanza Refinement & Feedback Loop**
    *   Implement selective stanza regeneration based on user text feedback prompts during review, allowing the LLM to rewrite individual stanzas interactively.
    *   [Implementation Plan](plans/phase_6/task_6_4_llm_stanza_refinement.md)
*   [ ] **Task 6.5: Automated Cover Art & Title Layout Generator**
    *   Automate cover generation by prompting the LLM for cover art matching the style guide, brewing the assets, and compiling KDP-conforming cover wraps.
    *   [Implementation Plan](plans/phase_6/task_6_5_automated_cover_generator.md)
*   [ ] **Task 6.6: Multi-Provider LLM Fallback & Retries**
    *   Implement dynamic retries with exponential backoff and automatic provider switching (e.g., fall back to OpenAI if Gemini fails) to avoid rate limit halts in headless runs. Apply fallback logic to both initiate-stage brainstorming and brew-stage manuscript generation.
    *   [Implementation Plan](plans/phase_6/task_6_6_multi_provider_fallback.md)

---

## Phase 7: Ecosystem Expansion
Focus: Exploring new integrations like Google Docs, RAG workflows, EPUB exports, and broader publishing infrastructure.

*   [x] **Task 7.0: Unified LLM Integration** (Formerly Task 5.6)
    *   Refactor Pithos's LLM client layer to consume the unified powerword LLM client package, eliminating the local raw HTTP REST implementations.

*   [ ] **Task 7.1: Trend-Based Brainstorming**
    *   Implement the `brainstorm` command and connect to the external `pw-mcp-trends` MCP plugin to generate parodic themes, titles, and illustration styles.
    *   [Implementation Plan](plans/phase_7/task_7_1_trend_based_brainstorming.md)
*   [ ] **Task 7.2: EPUB / Digital Publication Export**
    *   Integrate with a standalone `pw-mcp-epub` MCP plugin to export the parodic manuscript and generated illustration assets into a valid EPUB file.
    *   [Implementation Plan](plans/phase_7/task_7_2_epub_digital_export.md)
*   [ ] **Task 7.3: Google Doc MCP Integration**
    *   Integrate with the `pw-mcp-gdoc` MCP plugin to export manuscripts to Google Docs for editing and import them back on resume.
    *   [Implementation Plan](plans/phase_7/task_7_3_gdoc_mcp_integration.md)
*   [ ] **Task 7.4: Speculative: MCP Telemetry Migration**
    *   Migrate token telemetry and billing calculations from the imported module to a decoupled, external `pw-mcp-telemetry` MCP server.
    *   [Implementation Plan](plans/phase_7/task_7_4_mcp_telemetry_migration.md)
*   [ ] **Task 7.5: Speculative: Homebrew Formula for Dependency Management**
    *   Migrate dependency installation instructions to rely on Homebrew (`brew`) for installing required MCP plugin servers.
    *   [Implementation Plan](plans/phase_7/task_7_5_speculative_brew_dependencies.md)
*   [ ] **Task 7.6: Speculative: Publish Pithos as Homebrew Package with Dependencies**
    *   Publish pre-compiled Pithos binaries via Homebrew and list Powerword MCP plugins as package dependencies.
    *   [Implementation Plan](plans/phase_7/task_7_6_speculative_publish_pithos_brew.md)

---

## Phase 8: Speculative — Long-Form & Serious Publishing

> [!WARNING]
> **This phase is defrosted as Tasks 4.2 and 5.20 are now complete.** The scope below represents a potential future direction — evolving Pithos from a children's book factory into a general-purpose publishing pipeline. This is a distinct product pivot, not a natural extension of the current mission. Do not begin any Phase 8 task without an explicit product decision to expand scope.

Focus: Evolving Pithos into a modular, outline-driven book generation tool for technical writing, self-help, and novels.

*   [ ] **Task 8.1: Virtual Author Profiles & Persona Manager**
    *   Introduce modular author profile definitions (`authors/*.toml`), allowing custom pen names, personas, writing rules, and TTS voice pairings to be swapped dynamically.
    *   [Implementation Plan](plans/phase_8/task_8_1_author_profiles_persona_manager.md)
*   [ ] **Task 8.2: Hierarchical Outline-Driven Book Scaffolder**
    *   Implement multi-tier book generation (`series` -> `volume` -> `chapters` -> `sections`), allowing structured planning and outline generation prior to writing text.
    *   [Implementation Plan](plans/phase_8/task_8_2_hierarchical_scaffolder.md)
*   [ ] **Task 8.3: Modular Multi-File Workspace**
    *   Support compiling books from structured sub-folders (e.g., `chapters/*.md`, `references.bib`) rather than a single `manuscript.md` file.
    *   [Implementation Plan](plans/phase_8/task_8_3_modular_workspace.md)
*   [ ] **Task 8.4: Typst Professional Book Compilation & Templates**
    *   Integrate professional Typst layout templates for non-fiction (margins, headers, footers, table of contents) and novels (front-matter, chapter drop caps).
    *   [Implementation Plan](plans/phase_8/task_8_4_typst_professional_compilation.md)
*   [ ] **Task 8.5: EPUB Ebook Compilation & Formatting**
    *   Package the modular chapters, metadata, style guides, and cover image into standard, clean, validation-passing EPUB files for digital distribution.
    *   [Implementation Plan](plans/phase_8/task_8_5_epub_ebook_compilation.md)
*   [ ] **Task 8.6: Technical Diagram & Schematic Generation**
    *   Connect to MCP servers (`pw-mcp-diagram`) to generate vector diagrams (Mermaid, SVG, Graphviz) from text prompts and embed them in technical chapters.
    *   [Implementation Plan](plans/phase_8/task_8_6_technical_diagram_generation.md)
*   [ ] **Task 8.7: Automated Lorebook & Technical Glossary Manager**
    *   Maintain a global terminology/lore glossary in the manifest, feeding it as context to the LLM to prevent inconsistent terms in sci-fi/fantasy (lore-drift) or technical guides.
    *   [Implementation Plan](plans/phase_8/task_8_7_lorebook_glossary_manager.md)
*   [ ] **Task 8.8: Chapter Takeaways & Review Exercises Generator**
    *   Parse chapter drafts and prompt the LLM to generate learning summaries, review quizzes, and exercises to append to each chapter.
    *   [Implementation Plan](plans/phase_8/task_8_8_chapter_takeaways_review_generator.md)
*   [ ] **Task 8.9: Editorial Style Critic & Code Snippet Validator**
    *   Build an automated editorial critic that reviews drafts for reading level, voice, passive/active verb checks, and compile-verifies technical code snippets.
    *   [Implementation Plan](plans/phase_8/task_8_9_editorial_critic_validator.md)
*   [ ] **Task 8.10: Interactive Style Revision & Diff Reviewer**
    *   Implement an interactive terminal diff tool allowing authors to review, accept, or reject editorial style critic recommendations side-by-side.
    *   [Implementation Plan](plans/phase_8/task_8_10_interactive_diff_reviewer.md)
*   [ ] **Task 8.11: Bibliography, Citations & References Manager**
    *   Support ingesting BibTeX (`references.bib`) citations, passing citation targets to the LLM during drafting, and compiling formatted bibliographies.
    *   [Implementation Plan](plans/phase_8/task_8_11_citations_reference_manager.md)
*   [ ] **Task 8.12: Local "Consult" RAG Chatbot Subcommand**
    *   Implement a local RAG consultant CLI command (e.g. `pithos consult`) querying completed book content to provide customized playbooks using your exact terminology.
    *   [Implementation Plan](plans/phase_8/task_8_12_local_consult_chatbot.md)
*   [ ] **Task 8.13: Automated Audiobook Synthesis & TTS Narrator**
    *   Connect to text-to-speech MCP plugins to synthesize high-quality voice audio for completed book chapters and package them into audiobook files.
    *   [Implementation Plan](plans/phase_8/task_8_13_audiobook_tts_narrator.md)


---

## Phase 9: Speculative — Charm Ecosystem Enhancements

> [!NOTE]
> **This phase is speculative and deferred.** These tasks represent nice-to-have TUI/CLI polish that leverages the broader `github.com/charmbracelet` ecosystem. Do not begin any Phase 9 task until the core Phase 6 interactive review loop (Task 6.1) is complete and stable.

Focus: Elevating the Pithos terminal experience from functional to polished, using the full Charm toolkit.

*   [ ] **Task 9.1: SSH-Based Remote Pipeline Status via Wish**
    *   Integrate `github.com/charmbracelet/wish` to expose an optional, lightweight SSH server within the Pithos process.
    *   Allow remote querying of live pipeline status (current page, cost, errors) from a secondary terminal session or monitoring script, as an alternative to the Lamplighter WebRTC approach (Task 5.1).
