# plan: Task 2.3: Parallel Asset Generation

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-12
**Unit Test Coverage:** 91.70% (meets strict >91% threshold requirement)

Implement concurrent image generation workers in the Pithos brew engine, allowing multiple page illustrations to be requested and downloaded in parallel, reducing overall pipeline execution latency.

## User Review Required

> [!WARNING]
> **API Rate Limits**:
> Parallel generation may exceed rate limits for certain image generation backends (e.g. Midjourney or OpenAI). Users should be able to control concurrency via configuration or flags (defaulting to sequential execution/concurrency of 1 if rate-limited).

---

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- [x] Introduce a concurrency worker pool in `generateIllustrations`.
- [x] Add a new CLI flag/config parameter `Concurrency` (defaulting to `1` or `2`).
- [x] Launch worker goroutines using a semaphore channel to process pending pages concurrently.
- [x] Protect access to the MCP client (if stateful) or synchronize the `CallTool` calls if the server process handles concurrent RPC requests over a single stdio stream.
- [x] Handle partial failures gracefully: if a worker fails to generate an image for a specific page, log the error but allow other workers to complete, updating the manifest file atomically for all successful pages.

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- [x] Add a `--concurrency` flag to the `brew` command.

---

## Verification Plan

### Automated Tests
- [x] Run command: `go test -v ./internal/pipeline/...`
- [x] Add unit tests verifying that parallel image generation executes tasks concurrently, using a mock MCP transport that simulates concurrent requests (in `pipeline_test.go`).
- [x] Add integration tests that compile and run the real `pw-mcp-imagegen` subprocess client over stdio with mock API endpoint servers (in `integration_test.go`).

### Manual Verification
- [x] Run `pithos brew --concurrency 3` on a test book and verify that images are generated and saved in parallel.
