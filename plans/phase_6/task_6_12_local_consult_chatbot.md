# plan: Task 6.12: Local "Consult" RAG Chatbot Subcommand

**Status:** Open
**Go Version:** 1.26.4

Implement a local Retrieval-Augmented Generation (RAG) consult tool. Pithos will index the project's completed chapters, glossary, and Master Bible to provide an interactive CLI chatbot command (`pithos consult`). Creators can query this bot for tactical advice on real-world management scenarios, and the bot will reply with custom playbooks written in the exact voice and lexicon of the active Author Profile (Task 6.1).

## User Review Required

> [!NOTE]
> **Privacy & Offline Security**:
> The consult tool indexes local markdown files only. It does not upload files to external servers except when querying the configured LLM API (Gemini or OpenAI) to synthesize answers. All context chunks remain strictly local.

## Proposed Changes

### CLI Command Layer

#### [NEW] [consult.go](file://../../cmd/pithos/consult.go)
- Create `consultCmd` subcommand:
  ```bash
  pithos consult [query] --output [book_dir]
  ```
- Command options:
  - Interactive mode (`--interactive` or `-i`) to start a live terminal chat session.

### Consulting Engine

#### [NEW] [consult.go](file://../../internal/consult/consult.go)
- Implement an index and retrieval engine:
  - Load the book `manifest.json`, `Bible.md`, and all files in `chapters/*.md`.
  - Parse terms and text blocks, chunking sections into paragraphs.
  - Implement a simple local keyword index (BM25 or TF-IDF ranker) to retrieve context blocks relevant to the user query.
  - Construct the prompt:
    - Load the active `AuthorProfile` properties from the manifest.
    - Inject the author's Name, Persona, and custom ToneRules.
    - Inject relevant glossary definition contexts.
    - Inject matching chapter context paragraphs.
    - Append the user's specific scenario query.
  - Query the LLM client (using the shared client, fallback providers, and token accounting).
  - Stream the synthesized response back to the terminal.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Chunking and index retrieval returns matching sections.
  * Prompt assembler successfully integrates context snippets and glossary definitions.

### Manual Verification
1. Build the Pithos binary:
   ```bash
   make build
   ```
2. Run a query against a completed technical book:
   ```bash
   ./bin/pithos consult "My peer is slow-walking a database upgrade, how do I apply the Priority Budget?" --output serious-test
   ```
3. Verify the output is written in the custom author persona and refers to the proper glossary definitions.
4. Run in interactive mode:
   ```bash
   ./bin/pithos consult -i --output serious-test
   ```
   Confirm you can chat interactively.
