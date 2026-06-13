# Pithos: Gemini Agent Development Guide & AI Soul

Welcome, AI Agent! This document outlines the design patterns, architectural standards, and development guidelines for contributing to **Pithos**, the automated dark parody book factory. Please read and adhere to these guidelines to ensure consistency, premium code quality, and alignment with the rest of the Borch-AI ecosystem.

---

## 🏺 The Diogenes Ethos (Philosophy)

> *"I am looking for a human."* — Diogenes of Sinope

`pithos` is built in the spirit of Diogenes: minimalist, cynical, and ruthlessly efficient. We strip away everything unnecessary until only the work remains.
*   **Target Aesthetic:** Cynical, parodic children's books for existential dread.
*   **The Mission:** We do not replace human creativity; we automate the tedious logistics (layout, bleed calculations, image crawls, preflight checks) that prevent creativity from being profitable.
*   **Agent Attitude:** Be direct, concise, and focused on functional code. Avoid generic placeholder comments, fluff, or hand-waving.

---

## 📂 Repository Overview

Pithos is a command-line utility built in Go. The directory layout follows clean Go CLI conventions:

```
pithos/
├── cmd/pithos/           # CLI subcommand entrypoints (initiate, brew, assemble)
├── internal/
│   ├── config/          # Cobra/Viper environment and TOML config management
│   ├── manifest/        # manifest.json schema and persistent state manager
│   ├── mcp/             # MCP stdio client transport wrapper
│   └── pipeline/        # Initiate, Brew, and Assemble pipeline engines
├── plans/               # Detailed implementation plans by phase/task
│   └── phase_*/
├── scripts/             # Git hooks and local utility scripts
├── VISION.md            # High-level overview of product vision
├── ROADMAP.md           # Project roadmap and task checklists
└── GEMINI.md            # This file
```

---

## 🏛️ Architectural Principles

1.  **Deterministic Pipelines over Free-form ReAct:**
    *   Unlike generic agents, Pithos executes a rigid, predetermined sequence of subcommands (`initiate` -> `brew` -> `assemble`). The codebase must reflect this procedural reliability.
2.  **Encapsulation via MCP Plugins:**
    *   Defer complex operations (image generation, Typst PDF compilation, print math calculations, preflight inspections) to Powerword MCP servers rather than adding direct system dependencies or writing custom libraries in Pithos.
3.  **Strict State Boundaries (manifest.json):**
    *   Pithos owns the `manifest.json` schema completely. It acts as the machine-readable state contract between Pithos and Kiln.
    *   *Unidirectional writing:* Pithos writes milestones and costs; Kiln only reads them. Kiln must never directly write to `manifest.json`.

---

## ⚡ Context Window Optimization

To prevent hitting token limits during long runs and keep the context footprint minimal, the pipeline implements:

1.  **Manuscript Segmentation:** Breaks book manuscripts into distinct page/stanza boundaries in the state manifest to allow prompt context to focus on individual pages.
2.  **Context Caching:** Leverages Gemini's context caching for persistent prompt templates and static guidelines to avoid redundant token transfers.
3.  **On-Demand Loading:** Loads only relevant manifest records during pipeline resumes instead of holding the entire session history in active execution memory.

---

## 📊 Token & Quota Tracking

Real-time cost tracking is written directly to the state manifest:

*   **Input/Output Tokens:** Accumulates token usage for text generation prompt structures.
*   **Image Generation Costs:** Tracks DALL-E/Midjourney invocation costs ($0.040 per image by default).
*   **Session Billing Metrics:** Automatically calculated and updated in `manifest.json` under `total_cost_usd` based on configured model rates (e.g., Gemini/OpenAI standard pricing).

---

## 🛠️ Configuring Custom Model Selectors

API keys and model parameters are managed via environment variables or a local `.pithos.toml` file in the project root:

```toml
[api]
gemini_key = "YOUR_GEMINI_API_KEY"
openai_key = "YOUR_OPENAI_API_KEY"

[llm]
provider = "gemini"               # Options: "gemini", "openai"
default_model = "gemini-1.5-pro"  # Default model selector
temperature = 0.3
```
> [!TIP]
> Keep the temperature low (`0.1 - 0.3`) during `brew` to enforce rhythm and meter conformance for parodic stanzas, and use a medium temperature (`0.5`) for brainstorm command suggestions.

---

## 💻 Coding Guidelines

*   **Go Version:** Go 1.26+
*   **Format & Quality:** All Go code must be formatted with `gofmt` and linted using `golangci-lint` (enforcing `gosec`, `bodyclose`, `noctx`). Run `make lint` before declaring work done.
*   **Dependencies:** Keep external dependencies minimal. Rely on Cobra, Viper, and the official MCP Go SDK.

---

## 🧪 Testing Standards

*   **Strict Coverage Threshold:** We enforce a strict **91% unit test coverage** check. No commit that lowers coverage below 91% is allowed. Runs can be verified via:
    ```bash
    make check-coverage
    ```
*   **Integrations & Mocks:** Test pipeline stages using mock transport interfaces to isolate MCP subprocess calls from unit tests. Ensure test suites run concurrently without shared global states.

---

## 🔄 Pull Request & Merging Workflow

*   **PR Required for Mainline Changes:** Direct pushes to the remote `main` branch are blocked. AI agents and human contributors must **never** push changes directly to `main`. All updates must go through a Pull Request.
*   **Pull Requests Can Only Be Squash Merged:** To keep git history clean, all pull requests must be squash-merged back to `main`.
*   **Review Loop with Timer:** After opening or updating a Pull Request:
    1.  Pause (schedule a 30-second timer) to check for Copilot Code Review comments.
    2.  If no comments are found and the total wait time has not exceeded a 7-minute timeout, repeat the 30-second check loop.
    3.  Once comments are found, apply necessary refactors, push the fixes, and restart the check loop for the new commit.
*   **Review Loop Iteration Requirement:** You are forbidden from declaring a task complete or requesting user merge approval until a full polling loop has been completed on the latest commit with either zero new comments returned or all comments addressed.
*   **Update Implementation Plan:** Before declaring a task complete, update the corresponding plan in the `plans/` directory to reflect the final choices, configuration schemas, testing updates, and final Go version used.

---

## 🛡️ Cross-Project Boundaries & Distinct Workflows

Pithos works in tandem with other applications (e.g., Powerword, Kiln, Lamplighter). When contributing, strictly adhere to project boundaries:
*   **Check Existing Plans First:** Before proposing or making any changes that might cross over into another repository, explicitly check the `plans/` directory and GitHub issues of *both* repositories. Do not duplicate work that is already planned or completed in the other project.
*   **Isolate Changes:** Keep project codebases distinct. If a feature requires changes in multiple repositories, create and maintain separate, isolated implementation plans in each respective repository. Do not merge their plans into a single file.

---

## 🔗 Quick Reference Links

*   [VISION.md](file:///Users/human/code/pithos/VISION.md) - Product definition and stack boundaries.
*   [ROADMAP.md](file:///Users/human/code/pithos/ROADMAP.md) - Detailed breakdown of phases and milestones.
*   [Plans Directory](file:///Users/human/code/pithos/plans) - Structured folder directory containing implementation plans.
