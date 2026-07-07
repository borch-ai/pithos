# plan: Task 5.49: Doctor System Dependency Diagnostics

**Status:** Proposed
**Go Version:** 1.26.4

This task extends the `pithos doctor` diagnostics check list to check for essential external binaries (`ffmpeg`, `typst`) and verify local Service Account credential files and GCS bucket access permissions.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Diagnostics Layer

#### [MODIFY] [doctor.go](file://../../internal/pipeline/doctor.go)
- Expand the checklist functions to include system check routines:
  - `DiagnoseSystemBinaries(ctx context.Context) DiagnosticResult`
    - Look up `ffmpeg` and `typst` in the local system path.
    - If found, run them with `--version` and record version details; if missing, fail the test and print onboarding instructions.
  - `DiagnoseCloudPermissions(ctx context.Context) DiagnosticResult`
    - Verify that the Service Account key file defined in the config exists and is readable.
    - Perform a lightweight probe to the target GCS bucket to verify write/read access.
- Register these checks in the main `RunDiagnostics` handler.

---

## Verification Plan

### Automated Tests
- Test system path lookup behaviors in `internal/pipeline/doctor_test.go` by mocking path responses.

### Manual Verification
- Run `pithos doctor` and verify the expanded diagnostic checks are printed in the styled terminal view.
