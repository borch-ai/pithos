# plan: Task 1.2: Cobra & Viper CLI Scaffolding

**Status:** Open (Issue #[TBD])

Refactor the monolithic CLI scaffold in `cmd/pithos/main.go` into modular subcommands separated into individual files. Wire up Viper to parse, validate, and access a local `.pithos.toml` configuration file.

## Proposed Changes

### Command Scaffolding

#### [DELETE] [main.go](file:///Users/human/code/pithos/cmd/pithos/main.go)
- [ ] Remove the inline command stubs and handlers from the root main file, keeping only the initialization routines and CLI execution call.

#### [NEW] [root.go](file:///Users/human/code/pithos/cmd/pithos/root.go)
- [ ] Define the root `pithos` command.
- [ ] Configure `PersistentPreRun` or `PreRun` hook to initialize Viper configuration from `.pithos.toml` or fallback locations (`~/.config/pithos/config.toml`).

#### [NEW] [initiate.go](file:///Users/human/code/pithos/cmd/pithos/initiate.go)
- [ ] Define the `initiate` command.
- [ ] Implement flag options for `--output` directory.
- [ ] Wire up stub execution for pipeline workspace initialization.

#### [NEW] [brew.go](file:///Users/human/code/pithos/cmd/pithos/brew.go)
- [ ] Define the `brew` command.
- [ ] Implement flags for `--theme`, `--style`, and `--output`.
- [ ] Wire up stub execution pointing to the brew engine.

#### [NEW] [assemble.go](file:///Users/human/code/pithos/cmd/pithos/assemble.go)
- [ ] Define the `assemble` command.
- [ ] Implement flags for `--input`, `--format`, and `--bleed`.
- [ ] Wire up stub execution pointing to the assemble engine.

#### [NEW] [deploy.go](file:///Users/human/code/pithos/cmd/pithos/deploy.go)
- [ ] Define the `deploy` command.
- [ ] Implement flag options (e.g. `--input`).
- [ ] Wire up stub execution pointing to the deploy engine.

#### [NEW] [config.go](file:///Users/human/code/pithos/internal/config/config.go)
- [ ] Define configuration structures representing `.pithos.toml`.
- [ ] Implement `LoadConfig()` using `spf13/viper`.
- [ ] Add basic validation ensuring configured paths for MCP server binaries exist or default to system PATH.

---

## Verification Plan

### Automated Tests
- [ ] Run command: `go test ./internal/config/...`
- [ ] Validate configuration loading routines with mock toml files.
- [ ] Run command: `make lint` to verify that modular imports compile cleanly.

### Manual Verification
- [ ] Run `make build` and execute `./bin/pithos --help` to verify the help description.
- [ ] Verify that running commands without configuring `.pithos.toml` outputs a fallback warning.
