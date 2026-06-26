# plan: Task 5.40: E2E CLI Subprocess Integration Test Suite

**Status:** Proposed
**Go Version:** 1.26.4

This task implements a dedicated E2E integration test suite that builds the `pithos` command-line binary on-the-fly and executes it as a real subprocess to verify its behaviors at the CLI shell boundary.

## User Review Required

> [!NOTE]
> This task depends on **Task 5.37: E2E Pipeline Simulation & Dry-Run Mode**. To allow running these E2E subprocess tests in standard CI environments (where API keys and local MCP plugin installations are unavailable), the tests will invoke commands with the `--dry-run` flag.

## Proposed Changes

### Integration Testing Layer

#### [NEW] [cli_test.go](file://../../internal/pipeline/cli_test.go)
- Create a new integration test file to test the compiled binary.
- Implement helper function `buildPithosBinary(t *testing.T) string` which builds `./cmd/pithos` into a temporary directory on-the-fly and caches it for the duration of the test run.
- Write E2E tests:
  - `TestCLI_Initiate_Basic`: Execute the dynamically built binary with `initiate` and verify it scaffolds the directories and `manifest.json` correctly.
  - `TestCLI_Initiate_Brainstorm_OptOut`: Execute the dynamically built binary with `initiate --theme "turtle" --no-brainstorm` and verify the manifest has empty visual seeds.
  - `TestCLI_Initiate_Overwrite`: Verify overwrite prompts and responses on standard input (using `io.WriteString` to write "y\n" or "n\n" to standard input).
  - Verify exit codes are correct (e.g. exit code 0 on success, exit code 1 or 2 on flag/execution errors).

---

## Verification Plan

### Automated Tests
- Run the new integration tests:
  ```bash
  go test -v -tags=integration ./internal/pipeline/... -run="TestCLI_.*"
  ```
- Verify that test coverage is maintained or expanded.
