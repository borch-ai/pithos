# plan: Task 4.3: Verse Layout Formatting in Typst

**Status:** Open
**Go Version:** 1.26.4
**Date Completed:** —
**Unit Test Coverage:** —

Preserve stanza verse line breaks in Typst layouts instead of collapsing them into paragraphs. Parody stanzas are written in structured rhythm (AABB/ABAB) and must keep their line breaks to render as poetry.

## Proposed Changes

### Pre-processing Manuscript
Update Pithos layout assembly step to pre-process the manuscript text before sending it to the `pw-mcp-typst` compiler. Alternatively, update the compiler args to parse line breaks.

#### [MODIFY] [assemble.go](file:///Users/human/code/pithos/internal/pipeline/assemble.go)
- Format lines containing stanzas to translate newline characters (`\n`) into Typst line breaks (` \ `) or wrap them in block elements.

---

## Verification Plan

### Automated Tests
- Run `go test ./internal/pipeline/...` to verify that formatted stanzas contain Typst break characters.

### Manual Verification
- Compile Pithos, run `pithos assemble`, and open `interior.pdf` to verify that text contains distinct lines corresponding to the stanzas in `manuscript.md`.
