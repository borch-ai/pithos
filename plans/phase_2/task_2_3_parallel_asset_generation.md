# plan: Task 2.3: Parallel Asset Generation

**Status:** Open (Issue #[TBD])

Implement concurrent image generation workers in the Pithos brew engine, allowing multiple page illustrations to be requested and downloaded in parallel, reducing overall pipeline execution latency.

## User Review Required

> [!WARNING]
> **API Rate Limits**:
> Parallel generation may exceed rate limits for certain image generation backends (e.g. Midjourney or OpenAI). Users should be able to control concurrency via configuration or flags (defaulting to sequential execution/concurrency of 1 if rate-limited).

---

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](../../internal/pipeline/brew.go)
- [ ] Introduce a concurrency worker pool in `generateIllustrations`.
- [ ] Add a new CLI flag/config parameter `Concurrency` (defaulting to `1` or `2`).
- [ ] Launch worker goroutines using a sync/errgroup to process pending pages concurrently.
- [ ] Protect access to the MCP client (if stateful) or synchronize the `CallTool` calls if the server process handles concurrent RPC requests over a single stdio stream.
- [ ] Handle partial failures gracefully: if a worker fails to generate an image for a specific page, log the error but allow other workers to complete, updating the manifest file atomically for all successful pages.

#### [MODIFY] [brew.go](../../cmd/pithos/brew.go)
- [ ] Add a `--concurrency` flag to the `brew` command.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Add unit tests verifying that parallel image generation executes tasks concurrently, using a mock MCP transport that simulates concurrent requests.

### Manual Verification
- [ ] Run `pithos brew --concurrency 3` on a test book and verify that images are generated and saved in parallel.
