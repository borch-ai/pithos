# plan: Task 3.1: Plan Conformance Verification

**Status:** Completed
**Go Version:** Go 1.26
**Date Completed:** 2026-06-11

Implement a validation tool to enforce structural consistency across all implementation plans in the `plans/` directory, ensuring they strictly match the standard templates.
- **Unit Test Coverage:** 92.2% (meets the 91% threshold constraint)

## User Review Required

> [!NOTE]
> **Integration into Linter Loop**:
> Plan conformance checks will be integrated directly into the `Makefile` under the `lint` target. Any PR with malformed plan headers or structures will fail local quality gates automatically.

---

## Proposed Changes

### Scripting & Tooling

#### [NEW] main.go
- Create a lightweight helper utility in Go that:
  * Reads the template structure from `plans/TEMPLATE.md`.
  * Scans the `plans/` folder for files matching `task_*.md`.
  * Verifies each file contains key structural headings: `# plan: Task ...`, `## Proposed Changes`, `## Verification Plan`.
  * Inspects headings for correct file link references (using the `file:///` format) to ensure links resolve to valid codebase files.
  * Outputs detailed formatting warnings or exits with code `1` if check fails.

#### [MODIFY] [main.go](file:///Users/human/code/pithos/scripts/check_coverage/main.go)
- Relocated the check_coverage script to a subdirectory to avoid package `main` redeclaration conflicts when running `golangci-lint` over multiple files in the same directory.

### Build Integration

#### [MODIFY] [Makefile](file:///Users/human/code/pithos/Makefile)
- Add a new target `check-plans` running `go run scripts/validate_plans/main.go`.
- Update `check-coverage` to run `go run scripts/check_coverage/main.go`.
- Append `check-plans` to the global `make lint` target to verify plan structure during local and remote review builds.

---

## Verification Plan

### Automated Tests
- Created a temporary invalid plan in `plans/` lacking a `Verification Plan` section, containing relative links, and mismatching filenames.
- Ran `make check-plans` and confirmed the command failed with a non-zero exit status and printed diagnostic errors pointing to the malformed plan.
- Deleted the temporary file, ran `make check-plans` again, and confirmed it completed successfully.
- Ran `make lint` and verified that everything runs and lint errors are resolved (0 issues).
- Ran `make check-coverage` and verified that coverage is 92.2% (above 91%).
