# plan: Task 3.2: Local Critic Review Subsystem

**Status:** Open (Issue #[TBD])

Implement the Local Critic subsystem to run quality gates (linting, compilation, and test coverage checks) and leverage an LLM to verify repository changes against the proposed implementation plans before they are pushed to GitHub.

## User Review Required

> [!NOTE]
> **Plan Format Integration**:
> The critic will parse implementation plans directly from the local `plans/` directory (e.g. searching for plans matching `plans/task_*.md` that are in the git diff).

> [!IMPORTANT]
> **Git pre-push Hook**:
> Pushes to GitHub will be checked locally. If validation fails or the critic rejects the diff, the push is aborted and detailed feedback is written to `.pithos-critic.md` locally.

---

## Proposed Changes

### Configuration

#### [MODIFY] [config.go](../internal/config/config.go)
- Add fields to the Viper configuration struct:
  * `CriticProvider` (e.g., `gemini`, `openai`, `ollama`)
  * `CriticModel` (e.g., `gemini-1.5-flash`, `gpt-4o`)
  * `CriticEndpoint` (for custom LLM APIs / Ollama)
- Bind them to default values and environment variables.

### Review Component

#### [NEW] [critic.go](../internal/review/critic.go)
- Implement `ExtractGitDiff(ctx context.Context)`:
  * Run `git diff origin/main...HEAD` (committed but unpushed changes) and `git diff` (uncommitted modifications).
- Implement `LoadActivePlan(diff string) (*Plan, error)`:
  * Locate plans modified or added in the diff (e.g. `plans/task_*.md`).
  * Parse sections like `Goal`, `Proposed Changes`, and `Verification Plan`.
- Implement `VerifyWorkspace(ctx context.Context, cfg *config.Config)`:
  * Run local checks: `make lint`, `make build`, and `make check-coverage`.
  * If local builds fail, write log details to `.pithos-critic.md` and exit.
  * Extract the git diff and send it to the Critic LLM alongside the parsed plans.
  * Instruct the Critic LLM to check if the changes align with the plan and output a `VERDICT: ACCEPT` or `VERDICT: REJECT`.
  * Write feedback to `.pithos-critic.md` and exit with code `1` if rejected.

### Command Line Interface

#### [NEW] [review.go](../cmd/pithos/review.go)
- Register `pithos review` Cobra command with flags:
  * `--local`: Run local build/test validations and review against local plans.
  * `--plan`: Explicitly specify which implementation plan file to review against.

### Hooks & build files

#### [NEW] [pre-push](../scripts/git-hooks/pre-push)
- A Git hook script that runs `pithos review --local` and blocks pushes if it exits with an error status.

#### [MODIFY] [Makefile](../Makefile)
- Add target `install-hooks` to copy `scripts/git-hooks/pre-push` into `.git/hooks/pre-push` and make it executable.

---

## Verification Plan

### Automated Tests
- Run command: `go test ./internal/review/...`
- Unit tests verifying:
  * Extraction of diff outputs.
  * Plan parsing logic (parsing sections of implementation plans).
  * Mock LLM client responses triggering `ACCEPT`/`REJECT` behaviors.

### Manual Verification
- Attempt to push a change that violates a plan guideline or drops unit test coverage.
- Verify that the pre-push hook runs, runs `make check-coverage`, writes feedback to `.pithos-critic.md`, and blocks the push.
- Fix the issue and verify that the hook successfully cleans up `.pithos-critic.md` and allows the push.
