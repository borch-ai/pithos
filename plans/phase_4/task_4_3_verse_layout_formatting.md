# plan: Task 4.3: Verse Layout Formatting in Typst

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-16
**Unit Test Coverage:** 91.40%

Preserve stanza verse line breaks in Typst layouts instead of collapsing them into paragraphs. Parody stanzas are written in structured rhythm (AABB/ABAB) and must keep their line breaks to render as poetry.

## User Review Required

> [!NOTE]
> None. The downstream `pw-mcp-typst` compiler already formats stanza verses. Pithos ensures the manuscript file is auto-exported if missing.

## Open Questions

None.

## Design Decision & Implementation Details

During research, we verified that the downstream `pw-mcp-typst` compiler already contains the parsing logic to replace single newlines (`\n`) in stanzas with Typst break syntax (` \\\n `) inside paragraphs before compiling. 

Therefore, Pithos's key responsibility is to ensure that the manuscript file (`manuscript.md`) is guaranteed to exist on disk before triggering the assembly command (e.g. in headless runs where `--review` was not run). 

We added a check in the assembly step to automatically export the manuscript from the manifest's generated pages if it's missing on disk.

## Proposed Changes

### Pre-processing Manuscript
Update Pithos layout assembly step to pre-process the manuscript text before sending it to the `pw-mcp-typst` compiler. Alternatively, update the compiler args to parse line breaks.

#### [MODIFY] [assemble.go](file://../../internal/pipeline/assemble.go)
- Check if `manuscript.md` is present in the input directory. If not, automatically export it using the pages in the manifest.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline/...` to verify that `Assemble` automatically exports `manuscript.md` if it doesn't exist, and compiles successfully.

### Manual Verification
- Compile Pithos, run `pithos assemble`, and open `interior.pdf` to verify that text contains distinct lines corresponding to the stanzas in `manuscript.md`.
