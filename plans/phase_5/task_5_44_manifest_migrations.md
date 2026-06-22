# plan: Task 5.44: Manifest Versioning & Migration Guardrails

**Status:** Proposed
**Go Version:** 1.26.4

This task introduces explicit schema version tracking in `manifest.json` and automatic structure migrations on load. This prevents changes to Pithos manifest formats from breaking active integrations with Kiln or Lamplighter.

## User Review Required

> [!NOTE]
> Older manifests without a root-level version will be initialized at Version `1`.

## Proposed Changes

### Manifest Schema Layer

#### [MODIFY] [manifest.go](file:///Users/human/code/pithos/internal/manifest/manifest.go)
- Add a root-level `SchemaVersion int json:"schema_version"` to the `Manifest` struct.
- Define a package constant `CurrentSchemaVersion = 2`.
- Implement a series of migration functions:
  ```go
  var migrations = []func(*Manifest) error{
      migrateV1ToV2,
  }
  ```
  - `migrateV1ToV2` might populate default layout settings or ensure newly introduced fields (like trim size options or telemetries) are cleanly structured.
- Update `LoadManifest`:
  - Upon unmarshalling, check `m.SchemaVersion`.
  - If `m.SchemaVersion < CurrentSchemaVersion`, run outstanding migrations in order.
  - Automatically save the migrated manifest back to disk.

---

## Verification Plan

### Automated Tests
- Implement unit tests in `internal/manifest/manifest_test.go`:
  - Load a raw JSON string matching a Version 1 schema.
  - Verify that `LoadManifest` upgrades the struct to Version 2.
  - Assert that new fields are properly defaulted and saved.

### Manual Verification
- Create a mock older manifest file in a workspace.
- Run `pithos status` or `pithos ls` and verify it logs the migration and updates `manifest.json` with the new version and structures.
