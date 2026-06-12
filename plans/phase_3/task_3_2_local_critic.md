# plan: Task 3.2: Local Critic Review Subsystem (Direct Powerword Hook)

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-11
**Unit Test Coverage:** 92.20% total coverage (meets the 91% threshold)

Leverage the generalized `powerword review --local` command directly in Pithos's Git pre-push hook. This avoids duplicating any review commands, configuration, or MCP client logic within the Pithos codebase, maintaining a pristine codebase while still enforcing quality and critic gates.

## User Review Required

> [!IMPORTANT]
> **Powerword Installation**:
> This assumes the `powerword` CLI tool is globally installed or available in the path (e.g. at `/Users/human/go/bin/powerword`).

> [!NOTE]
> **Code Cleanup**:
> Under this plan, all local review command and helper files in Pithos will be deleted, keeping the codebase free of unnecessary local developer-tooling code.

---

## Proposed Changes

### Git Hooks & Makefile

#### [MODIFY] [pre-push](../../scripts/git-hooks/pre-push)
- Update the script to check if the `powerword` CLI is installed.
- Call `powerword review --local` directly instead of `./bin/pithos review --local`.

---

### Clean Up Redundant Review Subsystem

#### [DELETE] [review.go](../../cmd/pithos/review.go)
- Delete the file registering the `pithos review` Cobra subcommand.

#### [DELETE] [critic.go](../../internal/review/critic.go)
- Delete the stub/implementation file for workspace verification.

#### [DELETE] [critic_test.go](../../internal/review/critic_test.go)
- Delete the corresponding unit tests for the stub workspace verification.

---

## Verification Plan

### Automated Tests
- Run `make lint` and `make check-coverage` in Pithos to ensure that deleting these files leaves the codebase compile-clean and test-coverage-passing.

### Manual Verification
- Install the updated pre-push hook by running `make install-hooks`.
- Make a minor local modification and trigger `git push` to verify that `powerword review --local` runs, invokes the critic server, and performs validations as expected.

