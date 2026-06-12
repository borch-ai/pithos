# plan: Task 5.2: The `deploy` Engine

**Status:** Open (Issue #[TBD])

Implement the `deploy` command to generate metadata and package the final book for Amazon KDP upload.

## User Review Required

> [!NOTE]
> None.

## Proposed Changes

### Pipeline Core

#### [NEW] [deploy.go](../../internal/pipeline/deploy.go)
- [ ] Read the target book's `manifest.json` state.
- [ ] Boot up the `pw-mcp-seo` plugin.
- [ ] Send the manuscript poem content and themes to the SEO tool to generate optimal Amazon KDP keywords, categories, and "A+ Content" markdown/HTML description.
- [ ] Save the returned metadata output as `metadata.json`.
- [ ] Call `pw-mcp-video` (passing cover art, page assets, and a text script/TTS voiceover instructions) to generate a 15-second ASMR trailer. Save the output `.mp4` file.
- [ ] Zip the `manifest.json`, `metadata.json`, final compiled PDF (if compiled), and the promotional `.mp4` into a single release bundle `./books/<theme>/release.zip`.


#### [MODIFY] [main.go](../../cmd/pithos/main.go)
- [ ] Wire up the `deploy` Cobra command execution to point to the new deploy function.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/pipeline/...`

### Manual Verification
- [ ] Dry-run the deploy step with mock files.
- [ ] Validate that the final `.zip` contains all necessary artifacts for a KDP upload.
