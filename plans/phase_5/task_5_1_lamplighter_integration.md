# plan: Task 5.1: Lamplighter Integration

**Status:** Open (Issue #[TBD])

Integrate real-time WebRTC and Firebase signaling into Pithos so the long-running pipeline can be monitored and approved from the Lamplighter Android application.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Telemetry Subsystem

#### [NEW] [lamplighter.go](file://../../internal/telemetry/lamplighter.go)
- [ ] Implement a background daemon that runs parallel to the pipeline execution.
- [ ] Use Firebase Admin SDK (or a lightweight REST client) to advertise the active Pithos session to the user's paired mobile device.
- [ ] Stream standard output and token usage metrics (cost accounting) to Lamplighter.

### Pipeline Core Updates

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- [ ] Add support for "Approval Checkpoints" (e.g., halting the pipeline after generating the cover art or after manuscript drafting).
- [ ] Send an `APPROVAL_REQUIRED` payload to Lamplighter with the asset reference.
- [ ] Record the checkpoint status (e.g., `awaiting_approval`) in `manifest.json` so the state persists.
- [ ] Wait asynchronously for a `CONFIRM` or `REJECT` action from the mobile WebRTC tunnel.
- [ ] On approval, update `manifest.json` state to `approved` and proceed. On rejection, reset the step in the manifest and regenerate.


---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/telemetry/...`
- [ ] Unit test the WebRTC signaling logic with a mock peer.

### Manual Verification
- [ ] Verify the pipeline accurately halts and resumes execution when external signals are received from the Lamplighter dashboard.
