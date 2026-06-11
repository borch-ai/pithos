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
make test
```
