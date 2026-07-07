# plan: Task 5.36: Run Budget Limits & Cost Guardrails

**Status:** Completed — merged in [PR #57](https://github.com/borch-ai/pithos/pull/57) (squash commit `443f63ca`)
**Date Completed:** 2026-07-07
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.1%

This task adds cost verification checks and user budget constraints to Pithos. It prevents accidental overspend and rate cap breaches during long automated runs of `pithos brew` or `pithos assemble`.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add `MaxCostUSD` configuration key under a new `budget` section in `.pithos.toml` (defaulting to e.g., `5.00`).
- Update Viper configuration binding.

### Command Line Flags

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add `--budget` CLI flag to override the configured max cost limit.

### Pipeline Guardrails

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Implement cost estimate checks before generating manuscripts or illustrations.
- Prior to calling the MCP servers, evaluate:
  - Already-incurred cost (from `manifest.json`).
  - Expected cost of remaining pages to be generated (based on model pricing rates).
- If the estimated run exceeds the remaining budget:
  - Print a detailed warning summary showing the estimated excess cost.
  - Interactively prompt the user to confirm whether to proceed, abort, or set a temporary override (using `survey.Confirm`).
  - If in headless mode (`--silent` or non-interactive stdout), fail-fast with a descriptive error.
- Implement live dynamic budget checks:
  - During concurrent generation of page text and illustrations, check the accumulated actual cost dynamically after each model call finishes.
  - If the live cost exceeds the budget limits, immediately abort any remaining pending pages in the loop to prevent further cost leakage.

---

## Verification Plan

### Automated Tests
- Test budget check calculations using a mock manifest and custom pricing structures.
- Verify that the pipeline throws an error in non-interactive mode if the budget is exceeded.

### Manual Verification
- Set `budget.max_cost_usd = 0.10` in `.pithos.toml`.
- Run `pithos brew` on a new book structure.
- Verify the CLI prompts for confirmation because the predicted cost of generating stanzas + 15 page illustrations exceeds $0.10.
