# Pithos: Gemini Agent Development Guide

Welcome, AI Agent! This document outlines the design patterns, architectural standards, and development guidelines for contributing to **Pithos**.

## Architectural Principles

1. **Don't Reinvent the Wheel:** Pithos is an orchestrator. For any complex operation (image generation, Amazon SEO, complex PDF math, file system operations), **defer to Powerword MCP Plugins**. If a capability doesn't exist in Pithos, check if it should be built as a `pw-mcp-*` plugin in the Powerword repository instead.
2. **Deterministic Pipelines over Free-form ReAct:** Unlike generic agents, Pithos executes a rigid, predetermined set of steps (`initiate` -> `brew` -> `assemble` -> `deploy`). The code should reflect this procedural reliability.
3. **Explicit Configuration:** Use Cobra and Viper for configuration. API keys and model parameters should be strictly managed through environment variables or a local `.pithos.toml` file.

## Cross-Project Boundaries

Pithos is deeply integrated with **Powerword** and **Lamplighter**.
- **Powerword:** Used for its MCP server plugins (`pw-mcp-imagegen`, `pw-mcp-seo`, etc.).
- **Lamplighter:** Used as the mobile frontend for remote pipeline monitoring and human-in-the-loop approvals.
- **Isolate Changes:** If Pithos needs a new capability, and that capability is better suited as a generic tool, you must create a separate implementation plan in the *Powerword* repository to build the MCP server, and only update Pithos to *consume* that server.

## Coding Guidelines

- **Go Version:** Go 1.26+
- **Format & Quality:** Ensure all Go code is formatted with `gofmt` and linted using `golangci-lint` (enforcing `gosec`, `bodyclose`, `noctx`). Run with `make lint`.
- **Testing:** We enforce a strict **91% unit test coverage**. Run `make test` locally to verify changes.
- **Dependencies:**
  - `github.com/spf13/cobra` (CLI)
  - `github.com/spf13/viper` (Config)
  - `github.com/modelcontextprotocol/go-sdk` (MCP integration)

## Automated Review

Before finalizing any task, ensure the `Makefile` targets execute cleanly:
```bash
make lint
make build
make check-coverage
```

## Pull Request & Merging Workflow

- **PR Required for Mainline Changes:** Direct pushes to the remote `main` branch are blocked. AI agents and human contributors must **never** push changes directly to `main`. All updates, bug fixes, features, and documentation edits must go through a Pull Request.
- **Pull Requests Can Only Be Squash Merged:** To keep git history clean, all pull requests must be squash-merged back to `main`.
- **Review Copilot Feedback:** Review and address all comments, suggestions, or issues flagged by the GitHub Copilot Code Review runner before finalizing a task.
- **Review Loop with Timer:** After opening or updating a Pull Request, the agent should pause (e.g., schedule a 30-second timer) to check for Copilot Code Review comments. If no comments are found and the total wait time has not exceeded a 7-minute timeout, the agent should repeat the 30-second check loop. Once comments are found, or the timeout is reached with no comments, the agent applies necessary refactors, pushes the fixes, and restarts the check loop for the new commit.
- **Review Loop Iteration Requirement:** The review-and-fix process is iterative. The agent MUST repeat the polling-pause and comment-fetch loop for every commit pushed to the PR branch. The agent is forbidden from declaring a task complete or requesting user merge approval until a full polling loop has been completed on the latest commit with either zero new comments returned or all comments addressed.
- **Update Implementation Plan:** Before declaring a task complete or requesting merge approval, the agent/contributor must update the corresponding implementation plan in the `plans/` directory to reflect the final choices, actual configurations, testing updates, and final Go version used.

