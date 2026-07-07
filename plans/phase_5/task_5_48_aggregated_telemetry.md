# plan: Task 5.48: Aggregated Telemetry Reporting Command (pithos telemetry)

**Status:** Proposed
**Go Version:** 1.26.4

This task implements a centralized `pithos telemetry` command to aggregate and report total spent, tokens used, and image counts compiled across all local book workspaces.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Command Layer

#### [NEW] [telemetry.go](file://../../cmd/pithos/telemetry.go)
- Create a new Cobra command `telemetryCmd` for `pithos telemetry`.
- Reads and aggregates milestones, token usages, and costs from all workspaces.
- Supports formats:
  - Table: Standard styled terminal table using `lipgloss` and `table`.
  - JSON: Simple raw output for automated CI scripts (`--json` flag).

### Pipeline Engine

#### [NEW] [telemetry.go](file://../../internal/pipeline/telemetry.go)
- Implement `AggregateTelemetry() (AggregatedStats, error)`:
  - Walk `~/.local/share/pithos/workspaces/` to find all subdirectories containing `manifest.json`.
  - Parse each manifest's `Telemetry` block.
  - Compile aggregate metrics:
    - Total cost in USD.
    - Token usages grouped by model provider and model ID.
    - Total image generations.
    - Total completed books vs. books in progress.

---

## Verification Plan

### Automated Tests
- Write unit tests in `internal/pipeline/telemetry_test.go` verifying the aggregation logic using multiple mock workspace manifests.

### Manual Verification
- Run `pithos telemetry` in the terminal and verify formatted output.
- Run `pithos telemetry --json` and verify the output compiles into a parseable JSON structure.
