# plan: Task 3.1: Plan Conformance Verification

**Status:** Completed
**Go Version:** Go 1.26
**Date Completed:** 2026-06-11

Implement a validation tool to enforce structural consistency across all implementation plans in the `plans/` directory. (Note: Originally implemented as a local script in Pithos, this has been deleted and migrated to the global Powerword `powerword review` command as of Task 5.5 to avoid duplication).
- **Unit Test Coverage:** 92.2% (originally, before migrating to Powerword)

## User Review Required

> [!NOTE]
> **Migration to Powerword Developer Critic**:
> Conformance checks and plan validation have been migrated to the Powerword repository's developer critic command (`powerword review --local`), which runs as a Git pre-push hook. All local check scripts (`validate_plans/main.go`, `TEMPLATE.md`) and the Makefile's `check-plans` target have been deleted to avoid duplicate code.

---

## Proposed Changes

### Tooling Migration

#### [DELETE] validate_plans/main.go
- Deleted the local validator script. Conformance checking is now handled by the global `powerword review` command.

#### [DELETE] TEMPLATE.md
- Deleted the local plan template. A standardized template is now packaged directly within Powerword's critic tool.

#### [MODIFY] check_coverage/main.go
- Deleted this script as well, replacing its functionality with the project-agnostic `powerword check-coverage` subcommand.

### Build & Hook Integration

#### [MODIFY] [Makefile](../../Makefile)
- Removed the `check-plans` target and its dependency on `make lint`.
- Updated `check-coverage` to invoke `powerword check-coverage` instead of the local coverage script.

---

## Verification Plan

### Automated Validation
- Run the Git pre-push hook or `powerword review --local` to verify that plan conformance and workspace checks are executed by the global Powerword critic.
- Verify that `make check-coverage` successfully delegates statement coverage checking to `powerword check-coverage` and enforces the coverage threshold.
