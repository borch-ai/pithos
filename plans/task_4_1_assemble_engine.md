# plan: Task 4.1: The `assemble` Engine

**Status:** Completed
**Go Version:** Go 1.26.4
**Date Completed:** 2026-06-11
**Unit Test Coverage:** 92.5%

Implement the `assemble` command to calculate print constraints and generate a valid KDP layout manifest from the raw brewed assets.

## User Review Required

> [!NOTE]
> None.

## Actual Choices & Configurations

- **Go Version Used**: Go 1.26.4
- **Math MCP Client**: Integrated with `pw-mcp-kdp-math` to resolve KDP geometry calculations via MCP Tool `kdp_calculate_geometry`.
- **Validation Constraints**: Added hardcover minimum limit check (>= 75 pages).
- **Manifest Storage**: Expanded `KDPLayout` to store all margins, cover dimensions, overhang height, hinge/wrap width, and guides.

## Proposed Changes

### Pipeline Core

#### [NEW] [assemble.go](../internal/pipeline/assemble.go)
- [ ] Scan the book directory, load `manifest.json`, and count the total pages.
- [ ] Implement validation rules matching KDP specifications:
  - [ ] If `--format hardcover` is requested, verify that the page count is at least 75 pages. If less, abort execution with a clear validation error.
- [ ] Initialize an MCP client connection to `pw-mcp-kdp-math`.
- [ ] Request the KDP math plugin to calculate precise spine width, bleed dimensions, and safe zones based on the page count and target format.
- [ ] Update `manifest.json` with calculated dimensions, safe zones, and bounding boxes for text overlays.
- [ ] If a local rendering engine is configured, pass the updated manifest to compile the final PDF; otherwise, output the manifest layout parameters for external layout services (e.g. BookBolt, Inkfluence AI).


#### [MODIFY] [main.go](../cmd/pithos/main.go)
- [ ] Wire up the `assemble` Cobra command execution to point to the new assemble function.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/pipeline/...`
- [ ] Validate the generated `manifest.json` schema matches the expected strict structure.

### Manual Verification
- [ ] Ensure the KDP math plugin is successfully invoked during `pithos assemble` and the correct dimensions are injected into the manifest.
