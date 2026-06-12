# plan: Task 5.4: Speculative: Homebrew Formula for Dependency Management

**Status:** Speculative

This plan outlines migrating dependency installation instructions from direct Go installation (`go install`) to using Homebrew (`brew`) for managing and installing the Powerword MCP plugins (`pw-mcp-kdp-math`, `pw-mcp-imagegen`, `pw-mcp-seo`, etc.).

## User Review Required

> [!NOTE]
> None. This is a speculative plan.

## Proposed Changes

### Setup and Documentation

#### [MODIFY] [README.md](file://../../README.md)
- Update "Getting Started" / "Prerequisites" section.
- Replace Go installation commands with:
  ```bash
  brew tap borch-ai/tap
  brew install pw-mcp-kdp-math pw-mcp-imagegen pw-mcp-seo pw-mcp-video
  ```

#### [MODIFY] [Makefile](file://../../Makefile)
- Add a setup check target `check-deps` that runs `command -v` on each of the required binaries and prints a helpful recommendation to run the `brew install` commands if any are missing.

---

## Verification Plan

### Manual Verification
- Simulate a fresh environment without the dependencies installed, and verify the `check-deps` target correctly detects missing binaries and prompts with Homebrew install commands.
