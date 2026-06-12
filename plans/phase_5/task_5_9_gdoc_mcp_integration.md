# plan: Task 5.9: Google Doc MCP Integration

**Status:** Open

Integrate Pithos with the new `pw-mcp-gdoc` Model Context Protocol (MCP) server. This enables cloud-based manuscript reviews where Pithos generates the book manuscript, uploads it to a Google Doc, pauses for editing, and imports the updated manuscript back into `manifest.json` on resume.

## User Review Required

> [!NOTE]
> This feature requires the `pw-mcp-gdoc` server binary from Powerword to be configured in Pithos's Viper settings (`.pithos.toml`).

> [!WARNING]
> **Manuscript Structure Formatting**:
> When exporting to Google Docs, we will write clear page indicators (e.g. `--- Page 1 ---`). The importer will parse these page indicators. The user must not alter these headers when editing the document, otherwise Pithos will not be able to map stanzas back to specific page indices.

---

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](../../internal/config/config.go)
- Add a new config property for the GDocs MCP plugin binary path, e.g. `config.Cfg.MCP.GDocPath`.

### State manifest

#### [MODIFY] [manifest.go](../../internal/manifest/manifest.go)
- Add `GDocID string` with json tag `json:"gdoc_id,omitempty"` to the `Progress` struct in the manifest to persist the Google Doc reference between execution runs.

### CLI Layer

#### [MODIFY] [brew.go](../../cmd/pithos/brew.go)
- Add the `--gdoc` boolean flag to the `brew` command.
- Add an optional `--gdoc-id <id>` string flag to override or link to an existing document.
- Pass `GDoc bool` and `GDocID string` to `pipeline.BrewOptions`.

### Pipeline Core

#### [MODIFY] [brew.go](../../internal/pipeline/brew.go)
- Add `GDoc bool` and `GDocID string` to `BrewOptions`.
- Implement `exportManuscriptToGDoc(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error`:
  - Connect to `pw-mcp-gdoc` client.
  - Call `gdoc_create` to create a document containing formatted stanzas.
  - Save the returned `doc_id` to `m.Progress.GDocID` and call `m.Save()`.
- Implement `importManuscriptFromGDoc(ctx context.Context, m *manifest.Manifest, opts BrewOptions) (bool, error)`:
  - Connect to `pw-mcp-gdoc` client.
  - Call `gdoc_read` with `m.Progress.GDocID`.
  - Parse stanzas, update `manifest.json` for modified stanzas, and reset corresponding page statuses and image paths.
- Update `generateManuscript(ctx context.Context, m *manifest.Manifest, opts BrewOptions) error`:
  - If `opts.GDoc` is true and `m.Progress.ManuscriptGenerated` is false:
    1. Generate stanzas via LLM and save them.
    2. Invoke `exportManuscriptToGDoc` to create the Google Doc.
    3. Print the Google Doc edit URL and exit with instructions.
- Update `Brew` entrypoint:
  - Before starting image generation, check if `m.Progress.GDocID` is set and `opts.GDoc` is enabled.
  - If so, call `importManuscriptFromGDoc` to pull edits, sync manifest, and then proceed.

---

## Verification Plan

### Automated Tests
- Run `go test -v ./internal/pipeline/...`
- Add unit tests for `importManuscriptFromGDoc` and `exportManuscriptToGDoc` using a mock MCP transport that returns successful document payloads and IDs.

### Manual Verification
- Configure `pw-mcp-gdoc` path in `.pithos.toml`.
- Run `pithos brew --gdoc` on a new book theme. Verify a Google Doc is created, its URL is printed to the terminal, and Pithos exits.
- Open the Google Doc in your browser, modify a page stanza.
- Run `pithos brew` again. Verify the revised stanza is downloaded, page status updates in `manifest.json`, and its image is regenerated.
