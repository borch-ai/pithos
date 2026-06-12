# plan: Task 3.4: Interactive Local Review Mode

**Status:** Open (Issue #[TBD])

Implement an interactive mode in the Pithos brew engine that pauses execution after manuscript generation to let the user approve, edit, or regenerate generated stanzas in the terminal before committing to downstream image generation.

## User Review Required

> [!NOTE]
> None. This is an optional workflow optimization that is only triggered when the `--interactive` flag is explicitly provided.

---

## Proposed Changes

### Pipeline Core

#### [MODIFY] [brew.go](../../internal/pipeline/brew.go)
- [ ] Parse the `--interactive` option.
- [ ] During manuscript generation, check if interactive mode is enabled.
- [ ] If enabled, prompt the user for each stanza in the console:
  - `[A]pprove`: Approve stanza and proceed.
  - `[E]dit`: Edit stanza text manually in the terminal.
  - `[R]egenerate`: Request regeneration of this stanza from the LLM.
- [ ] Update the manifest dynamically with the user's edits or the regenerated stanza.

#### [MODIFY] [brew.go](../../cmd/pithos/brew.go)
- [ ] Add the `--interactive` boolean flag to the `brew` command.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test -v ./internal/pipeline/...`
- [ ] Test the interactive input loop using a mock stdin reader.

### Manual Verification
- [ ] Run `pithos brew --interactive` and test the input options to confirm manuscript edits save properly in `manifest.json`.
