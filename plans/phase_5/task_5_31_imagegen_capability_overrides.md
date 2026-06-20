# plan: Task 5.31: Imagegen Capability Overrides Configuration

**Status:** Completed
**Date Completed:** 2026-06-20
**Go Version:** 1.26.4
**Unit Test Coverage:** 91.3% (Actual)

This task implements custom configuration options in Pithos to allow manual overrides for image generation backend capabilities (`imagegen_force_cref` and `imagegen_force_sref`). This acts as an escape hatch to prevent contract rot if model capabilities are updated upstream before a code release is pushed.

## User Review Required

> [!NOTE]
> **Escape Hatch:** Bypassing capabilities validation with local overrides allows developers to force execution with newer upstream models without waiting for a new Pithos version release.

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Extend `Config` and `MCPConfig` structures:
  ```go
  type MCPConfig struct {
      // ...
      ImageGenForceCref bool `mapstructure:"imagegen_force_cref"`
      ImageGenForceSref bool `mapstructure:"imagegen_force_sref"`
  }
  ```
- Bind defaults in `LoadConfig`:
  ```go
  v.SetDefault("mcp.imagegen_force_cref", false)
  v.SetDefault("mcp.imagegen_force_sref", false)
  ```

### Brew Pipeline

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Update `checkBackendCapabilities` to check overrides:
  ```go
  if config.Cfg != nil && config.Cfg.MCP.ImageGenForceCref {
      caps.SupportsCref = true
  }
  ```
  This bypasses fail-fast blocks.

### Diagnostics Check

#### [MODIFY] [doctor.go](file://../../internal/pipeline/doctor.go)
- In `DiagnoseMCPPlugins`, apply configuration overrides to capability values before warnings checks:
  ```go
  if config.Cfg != nil && config.Cfg.MCP.ImageGenForceCref {
      caps.SupportsCref = true
  }
  ```
- Indicate if capabilities are overridden in the doctor output message (e.g. `"(cref: SUPPORTED [overridden], sref: UNSUPPORTED)"`).

---

## Verification Plan

### Automated Tests
- Test configuration parsing from `.pithos.toml` to verify `imagegen_force_cref` binds correctly.
- Add unit tests verifying `checkBackendCapabilities` passes even when backend returns `SupportsCref = false` if the override is enabled.

### Manual Verification
- Set `PITHOS_MCP_IMAGEGEN_FORCE_CREF=true` in `.env`.
- Run `pithos doctor` and verify the output displays the override state and passes without warning.
