# plan: Task 5.26: Automatic Browser Preview Opening

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

## Goal Description

Make the generated web preview visualizer (`web_preview/preview.html`) open automatically in the default system browser after `pithos brew` or `pithos assemble` runs, improving developer loop speed. 

To prevent this from breaking automated, headless, or CI runs, provide a `--silent` boolean flag on both commands to opt out of the browser-opening behavior.

## User Review Required

> [!IMPORTANT]
> **Default Inversion**: Browser-opening is enabled by default to provide an immediate visual feedback loop. Headless parent tools (like Kiln) or CI environments must specify the `--silent` flag to avoid launching desktop browsers.

---

## Proposed Changes

### Command Layer

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add a new boolean flag variable `brewSilent` bound to `--silent`.
- Pass `Silent: brewSilent` in the `pipeline.BrewOptions` struct to `pipeline.Brew`.

#### [MODIFY] [assemble.go](file://../../cmd/pithos/assemble.go)
- Add a new boolean flag variable `assembleSilent` bound to `--silent`.
- Pass `Silent: assembleSilent` in the `pipeline.AssembleOptions` struct to `pipeline.Assemble`.

### Pipeline Layer

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Add `Silent bool` to `BrewOptions` struct.
- Inside `Brew`, if `opts.Silent` is `false` and the web preview is generated successfully, trigger the automatic browser opening helper.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Add `Silent bool` to `AssembleOptions` struct.
- Inside `Assemble`, if `opts.Silent` is `false` and the web preview is regenerated successfully, trigger the automatic browser opening helper.

#### [NEW] [browser.go](file://../../internal/pipeline/browser.go)
- Implement `openBrowser(urlStr string) error` using platform-specific commands:
  - macOS: `open`
  - Linux: `xdg-open`
  - Windows: `rundll32 url.dll,FileProtocolHandler`

---

## Verification Plan

### Automated Tests
- Add unit tests in `pipeline_test.go` asserting:
  - If `Silent` is `true`, no subprocess command to open the browser is executed.
  - If `Silent` is `false`, the platform-appropriate open command is constructed.

### Manual Verification
1. Run `./bin/pithos brew --output books/cyber_diogenes --review`.
2. Confirm that a new browser window/tab opens displaying the visualizer index page immediately on completion.
3. Run `./bin/pithos brew --output books/cyber_diogenes --review --silent`.
4. Confirm that the command exits successfully without launching any browser windows.
