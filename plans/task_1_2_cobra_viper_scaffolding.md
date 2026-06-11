# plan: Task 1.2: Cobra & Viper CLI Scaffolding

**Status:** Completed

Refactored the monolithic CLI scaffold in `cmd/pithos/main.go` into modular subcommands separated into individual files. Wired up Viper to parse, validate, and access a local `.pithos.toml` configuration file and local `.env` environment credentials file.

- **Go Version:** Go 1.26.4
- **Viper Version:** github.com/spf13/viper v1.21.0
- **Cobra Version:** github.com/spf13/cobra v1.10.2
- **Unit Test Coverage:** 94.40% total coverage (meets the 91% threshold constraint)

## Proposed Changes

### Command Scaffolding

#### [DELETE] [main.go](file:///Users/human/code/pithos/cmd/pithos/main.go)
- [x] Remove the inline command stubs and handlers from the root main file, keeping only the initialization routines and CLI execution call.

#### [NEW] [root.go](file:///Users/human/code/pithos/cmd/pithos/root.go)
- [x] Define the root `pithos` command.
- [x] Configure `PersistentPreRun` or `PreRun` hook to initialize Viper configuration from `.pithos.toml` or fallback locations (`~/.config/pithos/config.toml`).

#### [NEW] [initiate.go](file:///Users/human/code/pithos/cmd/pithos/initiate.go)
- [x] Define the `initiate` command.
- [x] Implement flag options for `--output` directory.
- [x] Wire up stub execution for pipeline workspace initialization.

#### [NEW] [brew.go](file:///Users/human/code/pithos/cmd/pithos/brew.go)
- [x] Define the `brew` command.
- [x] Implement flags for `--theme`, `--style`, and `--output`.
- [x] Wire up stub execution pointing to the brew engine.

#### [NEW] [assemble.go](file:///Users/human/code/pithos/cmd/pithos/assemble.go)
- [x] Define the `assemble` command.
- [x] Implement flags for `--input`, `--format`, and `--bleed`.
- [x] Wire up stub execution pointing to the assemble engine.

#### [NEW] [deploy.go](file:///Users/human/code/pithos/cmd/pithos/deploy.go)
- [x] Define the `deploy` command.
- [x] Implement flag options (e.g. `--input`).
- [x] Wire up stub execution pointing to the deploy engine.

#### [NEW] [config.go](file:///Users/human/code/pithos/internal/config/config.go)
- [x] Define configuration structures representing `.pithos.toml` and `.env` credentials.
- [x] Implement `LoadConfig()` using `spf13/viper` and custom `.env` loader.
- [x] Add basic validation ensuring configured paths for MCP server binaries exist or default to system PATH.

#### [NEW] [.env.example](file:///Users/human/code/pithos/.env.example)
- [x] Document sensitive environment variable templates.

#### [MODIFY] [.pithos.toml.example](file:///Users/human/code/pithos/.pithos.toml.example)
- [x] Remove secret API keys block.

---

## Verification Plan

### Automated Tests
- [x] Run command: `go test ./internal/config/...`
- [x] Validate configuration loading routines with mock toml and env files.
- [x] Run command: `make lint` to verify that modular imports compile cleanly.

### Manual Verification
- [x] Run `make build` and execute `./bin/pithos --help` to verify the help description.
- [x] Verify that running commands without configuring `.pithos.toml` outputs a fallback warning.
