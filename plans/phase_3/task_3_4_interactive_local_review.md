# plan: Task 3.4: Local Markdown File Review Loop

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-12
**Unit Test Coverage:** 91.90% (meets strict >91% threshold requirement)

Instead of a console/terminal input loop, implement a local Markdown-based review workflow. When running `pithos brew` with a `--review` flag, Pithos will export the generated manuscript to a local `manuscript.md` file in the workspace directory and pause execution. The user can then edit the stanzas at their own pace using any text editor, and run `pithos brew` again to import changes, reset modified page states, and proceed to illustration generation.

## User Review Required

> [!NOTE]
> This local review process is optional and only triggered when running `pithos brew` with the `--review` flag.

> [!WARNING]
> **Page Reset on Sync**:
> During import, any page whose stanza text has been modified in `manuscript.md` relative to the manifest will have its `ImagePath` cleared and its `Status` set back to `pending`. This ensures downstream image generation generates fresh illustrations for updated stanzas.

---

## Proposed Changes

### CLI Layer

#### [MODIFY] [brew.go](file://../../cmd/pithos/brew.go)
- Add the `--review` boolean flag to the `brew` command.
- Pass `Review: brewReview` in `pipeline.BrewOptions`.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Add `Review bool` to `BrewOptions`.
- Implement `exportManuscriptToMarkdown(outputDir string, pages []manifest.PageState) error`:
  - Formats stanzas into a clean Markdown structure with page headers (e.g. `# Page 1`, `# Page 2`).
  - Prepends instructions inside Markdown comments informing the user how to edit the stanzas and resume.
- Implement `importManuscriptFromMarkdown(outputDir string, m *manifest.Manifest) (bool, error)`:
  - Reads and parses stanzas from the `manuscript.md` file by looking for `# Page N` headers.
  - Matches stanzas against existing pages in `m.Progress.Pages`.
  - If any stanza text differs, updates it in the manifest, resets the page status to `pending`, and clears its `ImagePath`.
  - Returns `true` if changes were detected and manifest was updated.
- Update `generateManuscript(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error`:
  - If `Review` is requested and `m.Progress.ManuscriptGenerated` is false:
    1. Invoke LLM to generate stanzas.
    2. Save to manifest and set `m.Progress.ManuscriptGenerated = true`.
    3. Call `exportManuscriptToMarkdown`.
    4. Return a special error/signal (or exit cleanly) indicating that the process has paused for review: `"Manuscript generated and exported to manuscript.md. Please review/edit the file and run brew again to continue."`
- Update `Brew` entrypoint:
  - Before starting image generation, check if `manuscript.md` exists in the output directory.
  - If it exists, call `importManuscriptFromMarkdown` to import any edits and save the manifest.

---

## Verification Plan

### Automated Tests
- Run `go test -v ./internal/pipeline/...`
- Add unit tests for `exportManuscriptToMarkdown` and `importManuscriptFromMarkdown` using temporary directories.
- Verify that modified stanzas trigger page resets (status back to `pending`, image path cleared).
- Verify that unmodified stanzas keep their completed status and image paths.

### Manual Verification
- Run `pithos brew --review` on a new book. Verify `manuscript.md` is created and Pithos exits with a review prompt.
- Edit `manuscript.md` (e.g., change page 2's poem).
- Run `pithos brew`. Verify page 2's status is reset and its image is regenerated, while other pages remain completed.
