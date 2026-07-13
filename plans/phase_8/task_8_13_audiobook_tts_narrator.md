# plan: Task 8.13: Automated Audiobook Synthesis & TTS Narrator

**Status:** Open
**Go Version:** 1.26.4

Implement automated audiobook synthesis for completed books. Pithos will parse chapter markdown drafts, strip visual and code block layouts to form a clean narrative voice script, query a text-to-speech (TTS) MCP plugin (such as OpenAI TTS or ElevenLabs) to synthesize audio tracks, and combine them into a single, standard `.m4b` or `.mp3` audiobook file with embedded chapter marks. The default voice configuration is loaded dynamically from the active Author Profile (Task 6.1).

## User Review Required

> [!WARNING]
> **Audiobook Synthesis API Costs**:
> Synthesizing a full book (e.g. 50,000 words) using premium neural voices can result in significant API billing charges. Users must verify they have configured a preferred, cost-effective voice model (such as OpenAI's standard TTS or local offline engines) in `.pithos.toml` before initiating synthesis.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add configurations for TTS operations:
  ```toml
  [api.tts]
  provider = "openai" // options: "openai", "elevenlabs"
  default_voice = "onyx"
  output_format = "m4b"
  ```

### CLI Command Layer

#### [NEW] [narrate.go](file://../../cmd/pithos/narrate.go)
- Introduce the `narrate` command:
  ```bash
  pithos narrate --output [book_dir]
  ```

### Pipeline Core

#### [NEW] [narrate.go](file://../../internal/pipeline/narrate.go)
- Implement `NarrateBook(ctx context.Context, outputDir string, m *manifest.Manifest) error`:
  - Create an `audio/` directory in the book workspace.
  - Read the active `AuthorProfile.TTSVoice` from the manifest. If empty, fall back to the config default voice.
  - For each chapter in the manifest:
    - Extract text from sections.
    - Strip structural syntax: remove markdown code blocks, Mermaid diagrams, image links, and bibliography citations to form a clean, readable text flow.
    - Check if the chapter audio file `audio/chapter_<index>.mp3` already exists.
    - If it does not exist:
      - Call the TTS MCP plugin (e.g. `tts_synthesize` tool) or query the OpenAI TTS API endpoint using the designated author voice parameter.
      - Save the synthesized audio file to `audio/chapter_<index>.mp3`.
  - Compile the audiobook:
    - Use `ffmpeg` or `mp4v2` commands locally (via the CLI execution wrapper) to combine the chapter MP3s, embed the book cover image `images/cover.png` as metadata, inject chapter timestamp bookmarks, and produce a unified `book.m4b` file.
    - Record audio generation metrics and costs in manifest telemetry.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Markdown script stripper correctly removes code blocks, flowcharts, and markdown symbols.
  * Audio synthesis loops execute sequentially and use the voice parameter configured in the active Author Profile.

### Manual Verification
1. Run the narration command on a completed project:
   ```bash
   ./bin/pithos narrate --output serious-test
   ```
2. Confirm the `books/serious-test/audio/` directory contains files like `chapter_1.mp3`.
3. Confirm that a final, unified `books/serious-test/book.m4b` is generated containing metadata and the embedded cover art.
4. Play the audiobook in Apple Books, VLC, or Audacity to confirm voice consistency.
