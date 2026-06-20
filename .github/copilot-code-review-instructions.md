# GitHub Copilot Code Review Instructions - Pithos

When reviewing pull requests in this repository, please enforce the following architectural principles, error handling patterns, and testing standards:

## 1. Strict State Boundaries & Manifest Integration
* **Manifest Ownership**: Pithos owns the `manifest.json` schema. Kiln only reads it. 
* **State Updates**: Ensure pipeline milestones, page processing statuses, and token execution costs are correctly and atomically written to `manifest.json`.
* **State Resumability**: Verify that pipeline resume logic does not double-generate stanzas or illustration assets that are already marked as completed.

## 2. Decoupling & MCP Transport Mocks
* **MCP Decoupling**: Defer complex external operations (image gen, math calculations, Typst compilations) to MCP server wrappers.
* **Test Isolation**: Unit tests must never invoke live external MCP binaries or make live HTTP requests. All pipeline steps must be tested using mock transport interfaces (`mcpsdk.Transport` or mock HTTP servers).

## 3. Error Handling & Resilience
* **No Panics**: Always return clean errors instead of panicking.
* **Error Wrapping**: Wrap errors using `%w` to preserve root-cause diagnostic context.
* **Resource Cleanup**: Clean up temporary files, subprocesses, and sockets via deferred close/stop functions immediately after instantiation.

## 4. Configuration Layer & CLI Flags
* **Viper/Cobra Bindings**: New Cobra CLI command flags must be explicitly bound to Viper configurations.
* **Env Prefix**: Environment variables overriding configuration keys must be prefixed with `PITHOS_` (via `v.SetEnvPrefix("PITHOS")`).
* **Home Directory Portability**: Default file paths (workspaces, logs, caches) must resolve under the home directory `~/.local/share/pithos/` to avoid polluting the repository tree.

## 5. Coding & Testing Standards
* **Go Version**: Target Go version is 1.26+. All code must be formatted with `gofmt` and pass `golangci-lint` check rules.
* **Strict Unit Test Coverage**: We enforce a strict **91% unit test coverage** requirement. PR additions must include corresponding table-driven `_test.go` config files.
