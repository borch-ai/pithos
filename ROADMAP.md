# Pithos Roadmap

This roadmap defines the engineering journey to build **Pithos**, the automated dark parody book pipeline.

---

## Ecosystem Context

Pithos is the **book factory** of the Borch-AI publishing stack. It is a deterministic pipeline, not a free-form agent.

- **Upstream**: **Kiln** orchestrates Pithos as a subprocess (`kiln forge`). Kiln supplies the validated niche concept; Pithos executes production.
- **Downstream**: Pithos invokes `pw-mcp-imagegen`, `pw-mcp-kdp-math`, `pw-mcp-typst` (pending) from the Powerword MCP plugin network.
- **Monitoring**: **Lamplighter** (Pithos Phase 5.1) provides mobile approval checkpoints during long brew/assemble runs.

> [!IMPORTANT]
> **The core functional deliverable of this tool is Task 4.2 (Typst PDF Layout Assembly) and Task 5.20 (Kiln Foundry State Integration)**. All Phase 6 work is frozen until Tasks 4.2 and 5.20 ship.

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
*   [ ] **Task 4.4: Mixed Layout Templates**
    *   Support left-page text / right-page image mixed layouts per page in the layout engine.
    *   [Implementation Plan](plans/phase_4/task_4_4_mixed_layout_templates.md)

---

## Phase 5: Telemetry & Kiln Integration
Focus: Remote monitoring, human-in-the-loop approvals, and Kiln state manifest integration.

### Core Pipeline & Ecosystem Integration
*   [ ] **Task 5.1: Lamplighter Integration**
    *   Integrate Firebase/WebRTC signaling to connect the Pithos process to the Lamplighter Android app.
    *   Implement interactive approval checkpoints (e.g., pausing the pipeline to wait for a human to approve the cover art on their phone) backed by state manifest updates.
    *   [Implementation Plan](plans/phase_5/task_5_1_lamplighter_integration.md)
*   [ ] **Task 5.20: Kiln Foundry State Integration**
    *   Implement the state contract by writing pipeline milestone updates (`initiate_complete`, `brew_complete`, `assemble_complete`) and accumulated costs to the Pithos workspace manifest (`manifest.json`), which Kiln reads to sync book status.
    *   [Implementation Plan](plans/phase_5/task_5_20_kiln_integration.md)
*   [ ] **Task 5.21: Character Invariant Injection**
    *   Inject visual invariant descriptors to preserve character consistency in generated illustration prompts.
    *   [Implementation Plan](plans/phase_5/task_5_21_character_invariant_injection.md)
*   [ ] **Task 5.22: Preflight Diagnostic Subcommand (`pithos doctor`)**
    *   Implement a `doctor` subcommand to run connectivity, credential, and backend health checks for all configured MCP plugins.
    *   [Implementation Plan](plans/phase_5/task_5_22_preflight_diagnostics_doctor.md)
*   [ ] **Task 5.23: Dynamic Image Model Discovery**
    *   Query model capability list dynamically from provider APIs to select the best available model version.
    *   [Implementation Plan](plans/phase_5/task_5_23_dynamic_image_model_discovery.md)
*   [ ] **Task 5.24: Graceful Image Backend Fallbacks**
    *   Implement fallback handling from OpenAI to Gemini Imagen (and vice-versa) when image generation fails due to credential or permission errors.
    *   [Implementation Plan](plans/phase_5/task_5_24_graceful_image_backend_fallbacks.md)

### Interactive UI & Quality Enhancements
*   [x] **Task 5.10: Visual Prompt Expansion for Character Consistency**
    *   Modify stanzas generation to produce parodic poems alongside character-consistent illustration prompts. Integrate prompts in the manifest and the markdown review loop.
    *   [Implementation Plan](plans/phase_5/task_5_10_visual_prompt_expansion.md)
*   [x] **Task 5.11: LLM-Driven Style & Character Seeds**
    *   Implement automated visual style and character profile generation as a dedicated preceding step of `brew`.
    *   Feed the generated style/character seed into the manuscript generator as prompt context to ensure stanzas and illustration prompts align.
    *   Configure a global character style reference flag (`--style`) to allow manual override.
    *   [Implementation Plan](plans/phase_5/task_5_11_global_style_character_seeds.md)
*   [x] **Task 5.12: Robust JSON Output Parsing**
    *   Implement an LLM response sanitization helper to strip markdown code blocks (e.g. ` ```json ... ``` `) and protect Pithos against parsing errors.
    *   [Implementation Plan](plans/phase_5/task_5_12_robust_json_parsing.md)
*   [ ] **Task 5.13: Multi-Provider LLM Fallback & Retries**
    *   Implement dynamic retries with exponential backoff and automatic provider switching (e.g., fall back to OpenAI if Gemini fails) to avoid rate limit halts in headless runs.
    *   [Implementation Plan](plans/phase_5/task_5_13_multi_provider_fallback.md)
*   [ ] **Task 5.14: Bubbletea TUI-Based Interactive Review Loop**
    *   Replace the raw file-editing loop with an interactive terminal review dashboard, enabling users to edit stanzas, customize prompts, and trigger select regeneration.
    *   [Implementation Plan](plans/phase_5/task_5_14_bubbletea_tui_review.md)
*   [ ] **Task 5.15: Multi-Model Illustration Variations & Selection**
    *   Generate illustration variations in parallel using multiple configured image models.
    *   Support manual variation selection via markdown reviews and hotkeys in the interactive TUI dashboard.
    *   [Implementation Plan](plans/phase_5/task_5_15_multi_model_image_variations.md)
*   [ ] **Task 5.16: Inline Terminal Graphics Previews in TUI**
    *   Integrate terminal image rendering protocols (Kitty, Sixel) within the Bubbletea review TUI to display visual illustration previews directly in the console.
    *   [Implementation Plan](plans/phase_5/task_5_16_inline_terminal_previews.md)
*   [ ] **Task 5.17: LLM-Driven Stanza Refinement & Feedback Loop**
    *   Implement selective stanza regeneration based on user text feedback prompts during review, allowing the LLM to rewrite individual stanzas interactively.
    *   [Implementation Plan](plans/phase_5/task_5_17_llm_stanza_refinement.md)
*   [ ] **Task 5.18: Automated Cover Art & Title Layout Generator**
    *   Automate cover generation by prompting the LLM for cover art matching the style guide, brewing the assets, and compiling KDP-conforming cover wraps.
    *   [Implementation Plan](plans/phase_5/task_5_18_automated_cover_generator.md)
*   [ ] **Task 5.19: Interactive Web-Based Book Preview (HTML/CSS)**
    *   Generate a static web-based preview folder containing an interactive flipbook player to visually review books locally in any browser.
    *   [Implementation Plan](plans/phase_5/task_5_19_web_book_preview.md)

### Speculative & Dependency Management
*   [ ] **Task 5.3: Speculative: MCP Telemetry Migration**
    *   Migrate token telemetry and billing calculations from the imported module to a decoupled, external `pw-mcp-telemetry` MCP server.
    *   [Implementation Plan](plans/phase_5/task_5_3_mcp_telemetry_migration.md)
*   [ ] **Task 5.4: Speculative: Homebrew Formula for Dependency Management**
    *   Migrate dependency installation instructions to rely on Homebrew (`brew`) for installing required MCP plugin servers.
    *   [Implementation Plan](plans/phase_5/task_5_4_speculative_brew_dependencies.md)
*   [ ] **Task 5.5: Speculative: Publish Pithos as Homebrew Package with Dependencies**
    *   Publish pre-compiled Pithos binaries via Homebrew and list Powerword MCP plugins as package dependencies.
    *   [Implementation Plan](plans/phase_5/task_5_5_speculative_publish_pithos_brew.md)
*   [x] **Task 5.6: Unified LLM Integration**
    *   Refactor Pithos's LLM client layer to consume the unified powerword LLM client package, eliminating the local raw HTTP REST implementations.
    *   [Implementation Plan](plans/phase_5/task_5_6_unified_llm_integration.md)
*   [ ] **Task 5.7: Trend-Based Brainstorming**
    *   Implement the `brainstorm` command and connect to the external `pw-mcp-trends` MCP plugin to generate parodic themes, titles, and illustration styles.
    *   [Implementation Plan](plans/phase_5/task_5_7_trend_based_brainstorming.md)
*   [ ] **Task 5.8: EPUB / Digital Publication Export**
    *   Integrate with a standalone `pw-mcp-epub` MCP plugin to export the parodic manuscript and generated illustration assets into a valid EPUB file.
    *   [Implementation Plan](plans/phase_5/task_5_8_epub_digital_export.md)
    *   [ ] **Task 5.8.1: Configuration and EPUB Plugin CLI Registration**
        *   Add configuration settings in `config.go` for locating `pw-mcp-epub` binary and bind Viper keys.
    *   [ ] **Task 5.8.2: Assembly Pipeline Integration**
        *   Connect `pithos assemble` pipeline to launch the `pw-mcp-epub` client and call `compile_epub`.
    *   [ ] **Task 5.8.3: E2E Subprocess Integration Test Suite**
        *   Implement an integration test that builds `pw-mcp-epub` from powerword sibling directory and performs complete pipeline compile checks.
    *   [ ] **Task 5.8.4: EPUB Standards Structural Validation**
        *   Implement validation in the test suite to unpack the resulting `.epub` and verify strict compliance (uncompressed mimetype, container.xml, content.opf manifest, and toc.xhtml).
*   [ ] **Task 5.9: Google Doc MCP Integration**
    *   Integrate with the `pw-mcp-gdoc` MCP plugin to export manuscripts to Google Docs for editing and import them back on resume.
    *   [Implementation Plan](plans/phase_5/task_5_9_gdoc_mcp_integration.md)
*   [x] **Task 5.10: Visual Prompt Expansion for Character Consistency**
    *   Modify stanzas generation to produce parodic poems alongside character-consistent illustration prompts. Integrate prompts in the manifest and the markdown review loop.
    *   [Implementation Plan](plans/phase_5/task_5_10_visual_prompt_expansion.md)
*   [x] **Task 5.11: LLM-Driven Style & Character Seeds**
    *   Implement automated visual style and character profile generation as a dedicated preceding step of `brew`.
    *   Feed the generated style/character seed into the manuscript generator as prompt context to ensure stanzas and illustration prompts align.
    *   Configure a global character style reference flag (`--style`) to allow manual override.
    *   [Implementation Plan](plans/phase_5/task_5_11_global_style_character_seeds.md)
*   [x] **Task 5.12: Robust JSON Output Parsing**
    *   Implement an LLM response sanitization helper to strip markdown code blocks (e.g. ` ```json ... ``` `) and protect Pithos against parsing errors.
    *   [Implementation Plan](plans/phase_5/task_5_12_robust_json_parsing.md)
*   [ ] **Task 5.13: Multi-Provider LLM Fallback & Retries**
    *   Implement dynamic retries with exponential backoff and automatic provider switching (e.g., fall back to OpenAI if Gemini fails) to avoid rate limit halts in headless runs.
    *   [Implementation Plan](plans/phase_5/task_5_13_multi_provider_fallback.md)
*   [ ] **Task 5.14: Bubbletea TUI-Based Interactive Review Loop**
    *   Replace the raw file-editing loop with an interactive terminal review dashboard, enabling users to edit stanzas, customize prompts, and trigger select regeneration.
    *   [Implementation Plan](plans/phase_5/task_5_14_bubbletea_tui_review.md)
*   [ ] **Task 5.15: Multi-Model Illustration Variations & Selection**
    *   Generate illustration variations in parallel using multiple configured image models.
    *   Support manual variation selection via markdown reviews and hotkeys in the interactive TUI dashboard.
    *   [Implementation Plan](plans/phase_5/task_5_15_multi_model_image_variations.md)
*   [ ] **Task 5.16: Inline Terminal Graphics Previews in TUI**
    *   Integrate terminal image rendering protocols (Kitty, Sixel) within the Bubbletea review TUI to display visual illustration previews directly in the console.
    *   [Implementation Plan](plans/phase_5/task_5_16_inline_terminal_previews.md)
*   [ ] **Task 5.17: LLM-Driven Stanza Refinement & Feedback Loop**
    *   Implement selective stanza regeneration based on user text feedback prompts during review, allowing the LLM to rewrite individual stanzas interactively.
    *   [Implementation Plan](plans/phase_5/task_5_17_llm_stanza_refinement.md)
*   [ ] **Task 5.18: Automated Cover Art & Title Layout Generator**
    *   Automate cover generation by prompting the LLM for cover art matching the style guide, brewing the assets, and compiling KDP-conforming cover wraps.
    *   [Implementation Plan](plans/phase_5/task_5_18_automated_cover_generator.md)
*   [ ] **Task 5.19: Interactive Web-Based Book Preview (HTML/CSS)**
    *   Generate a static web-based preview folder containing an interactive flipbook player to visually review books locally in any browser.
    *   [Implementation Plan](plans/phase_5/task_5_19_web_book_preview.md)

---

## Phase 6: Long-Form & Serious Publishing
Focus: Evolving Pithos into a modular, outline-driven book generation tool for technical writing, self-help, and novels.

*   [ ] **Task 6.1: Virtual Author Profiles & Persona Manager**
    *   Introduce modular author profile definitions (`authors/*.toml`), allowing custom pen names, personas, writing rules, and TTS voice pairings to be swapped dynamically.
    *   [Implementation Plan](plans/phase_6/task_6_1_author_profiles_persona_manager.md)
*   [ ] **Task 6.2: Hierarchical Outline-Driven Book Scaffolder**
    *   Implement multi-tier book generation (`series` -> `volume` -> `chapters` -> `sections`), allowing structured planning and outline generation prior to writing text.
    *   [Implementation Plan](plans/phase_6/task_6_2_hierarchical_scaffolder.md)
*   [ ] **Task 6.3: Modular Multi-File Workspace**
    *   Support compiling books from structured sub-folders (e.g., `chapters/*.md`, `references.bib`) rather than a single `manuscript.md` file.
    *   [Implementation Plan](plans/phase_6/task_6_3_modular_workspace.md)
*   [ ] **Task 6.4: Typst Professional Book Compilation & Templates**
    *   Integrate professional Typst layout templates for non-fiction (margins, headers, footers, table of contents) and novels (front-matter, chapter drop caps).
    *   [Implementation Plan](plans/phase_6/task_6_4_typst_professional_compilation.md)
*   [ ] **Task 6.5: EPUB Ebook Compilation & Formatting**
    *   Package the modular chapters, metadata, style guides, and cover image into standard, clean, validation-passing EPUB files for digital distribution.
    *   [Implementation Plan](plans/phase_6/task_6_5_epub_ebook_compilation.md)
*   [ ] **Task 6.6: Technical Diagram & Schematic Generation**
    *   Connect to MCP servers (`pw-mcp-diagram`) to generate vector diagrams (Mermaid, SVG, Graphviz) from text prompts and embed them in technical chapters.
    *   [Implementation Plan](plans/phase_6/task_6_6_technical_diagram_generation.md)
*   [ ] **Task 6.7: Automated Lorebook & Technical Glossary Manager**
    *   Maintain a global terminology/lore glossary in the manifest, feeding it as context to the LLM to prevent inconsistent terms in sci-fi/fantasy (lore-drift) or technical guides.
    *   [Implementation Plan](plans/phase_6/task_6_7_lorebook_glossary_manager.md)
*   [ ] **Task 6.8: Chapter Takeaways & Review Exercises Generator**
    *   Parse chapter drafts and prompt the LLM to generate learning summaries, review quizzes, and exercises to append to each chapter.
    *   [Implementation Plan](plans/phase_6/task_6_8_chapter_takeaways_review_generator.md)
*   [ ] **Task 6.9: Editorial Style Critic & Code Snippet Validator**
    *   Build an automated editorial critic that reviews drafts for reading level, voice, passive/active verb checks, and compile-verifies technical code snippets.
    *   [Implementation Plan](plans/phase_6/task_6_9_editorial_critic_validator.md)
*   [ ] **Task 6.10: Interactive Style Revision & Diff Reviewer**
    *   Implement an interactive terminal diff tool allowing authors to review, accept, or reject editorial style critic recommendations side-by-side.
    *   [Implementation Plan](plans/phase_6/task_6_10_interactive_diff_reviewer.md)
*   [ ] **Task 6.11: Bibliography, Citations & References Manager**
    *   Support ingesting BibTeX (`references.bib`) citations, passing citation targets to the LLM during drafting, and compiling formatted bibliographies.
    *   [Implementation Plan](plans/phase_6/task_6_11_citations_reference_manager.md)
*   [ ] **Task 6.12: Local "Consult" RAG Chatbot Subcommand**
    *   Implement a local RAG consultant CLI command (e.g. `pithos consult`) querying completed book content to provide customized playbooks using your exact terminology.
    *   [Implementation Plan](plans/phase_6/task_6_12_local_consult_chatbot.md)
*   [ ] **Task 6.13: Automated Audiobook Synthesis & TTS Narrator**
    *   Connect to text-to-speech MCP plugins to synthesize high-quality voice audio for completed book chapters and package them into audiobook files.
    *   [Implementation Plan](plans/phase_6/task_6_13_audiobook_tts_narrator.md)








---

## Phase 6: Speculative — Long-Form & Serious Publishing

> [!WARNING]
> **This entire phase is frozen until Pithos Tasks 4.2 (Typst PDF Layout Assembly) and 5.20 (Kiln Foundry State Integration) are complete.** The scope below represents a potential future direction — evolving Pithos from a children's book factory into a general-purpose publishing pipeline. This is a distinct product pivot, not a natural extension of the current mission. Do not begin any Phase 6 task without an explicit product decision to expand scope.

Focus: Evolving Pithos into a modular, outline-driven book generation tool for technical writing, self-help, and novels.

*   [ ] **Task 6.1: Virtual Author Profiles & Persona Manager**
    *   Introduce modular author profile definitions (`authors/*.toml`), allowing custom pen names, personas, writing rules, and TTS voice pairings to be swapped dynamically.
    *   [Implementation Plan](plans/phase_6/task_6_1_author_profiles_persona_manager.md)
*   [ ] **Task 6.2: Hierarchical Outline-Driven Book Scaffolder**
    *   Implement multi-tier book generation (`series` -> `volume` -> `chapters` -> `sections`), allowing structured planning and outline generation prior to writing text.
    *   [Implementation Plan](plans/phase_6/task_6_2_hierarchical_scaffolder.md)
*   [ ] **Task 6.3: Modular Multi-File Workspace**
    *   Support compiling books from structured sub-folders (e.g., `chapters/*.md`, `references.bib`) rather than a single `manuscript.md` file.
    *   [Implementation Plan](plans/phase_6/task_6_3_modular_workspace.md)
*   [ ] **Task 6.4: Typst Professional Book Compilation & Templates**
    *   Integrate professional Typst layout templates for non-fiction (margins, headers, footers, table of contents) and novels (front-matter, chapter drop caps).
    *   [Implementation Plan](plans/phase_6/task_6_4_typst_professional_compilation.md)
*   [ ] **Task 6.5: EPUB Ebook Compilation & Formatting**
    *   Package the modular chapters, metadata, style guides, and cover image into standard, clean, validation-passing EPUB files for digital distribution.
    *   [Implementation Plan](plans/phase_6/task_6_5_epub_ebook_compilation.md)
*   [ ] **Task 6.6: Technical Diagram & Schematic Generation**
    *   Connect to MCP servers (`pw-mcp-diagram`) to generate vector diagrams (Mermaid, SVG, Graphviz) from text prompts and embed them in technical chapters.
    *   [Implementation Plan](plans/phase_6/task_6_6_technical_diagram_generation.md)
*   [ ] **Task 6.7: Automated Lorebook & Technical Glossary Manager**
    *   Maintain a global terminology/lore glossary in the manifest, feeding it as context to the LLM to prevent inconsistent terms in sci-fi/fantasy (lore-drift) or technical guides.
    *   [Implementation Plan](plans/phase_6/task_6_7_lorebook_glossary_manager.md)
*   [ ] **Task 6.8: Chapter Takeaways & Review Exercises Generator**
    *   Parse chapter drafts and prompt the LLM to generate learning summaries, review quizzes, and exercises to append to each chapter.
    *   [Implementation Plan](plans/phase_6/task_6_8_chapter_takeaways_review_generator.md)
*   [ ] **Task 6.9: Editorial Style Critic & Code Snippet Validator**
    *   Build an automated editorial critic that reviews drafts for reading level, voice, passive/active verb checks, and compile-verifies technical code snippets.
    *   [Implementation Plan](plans/phase_6/task_6_9_editorial_critic_validator.md)
*   [ ] **Task 6.10: Interactive Style Revision & Diff Reviewer**
    *   Implement an interactive terminal diff tool allowing authors to review, accept, or reject editorial style critic recommendations side-by-side.
    *   [Implementation Plan](plans/phase_6/task_6_10_interactive_diff_reviewer.md)
*   [ ] **Task 6.11: Bibliography, Citations & References Manager**
    *   Support ingesting BibTeX (`references.bib`) citations, passing citation targets to the LLM during drafting, and compiling formatted bibliographies.
    *   [Implementation Plan](plans/phase_6/task_6_11_citations_reference_manager.md)
*   [ ] **Task 6.12: Local "Consult" RAG Chatbot Subcommand**
    *   Implement a local RAG consultant CLI command (e.g. `pithos consult`) querying completed book content to provide customized playbooks using your exact terminology.
    *   [Implementation Plan](plans/phase_6/task_6_12_local_consult_chatbot.md)
*   [ ] **Task 6.13: Automated Audiobook Synthesis & TTS Narrator**
    *   Connect to text-to-speech MCP plugins to synthesize high-quality voice audio for completed book chapters and package them into audiobook files.
    *   [Implementation Plan](plans/phase_6/task_6_13_audiobook_tts_narrator.md)








