# plan: Task 5.27: GCP Cloud Storage Configuration Support

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-18
**Unit Test Coverage:** 91.50%

## Goal Description

Enable Pithos to configure Google Cloud Storage (GCS) account parameters natively in `.pithos.toml` and `.env`. 

Since character-invariant image generation (Imagen/DALL-E) requires public URLs for reference images, Pithos delegates the upload of character seed portraits to the `pw-mcp-cloud` MCP server. By allowing GCS parameters to be configured in Pithos and forwarding them to the MCP sub-processes, users can seamlessly generate consistent character illustrations without manually setting up a separate `powerword.toml`.

## User Review Required

> [!IMPORTANT]
> **Subprocess Environment Propagation**: Pithos will parse the `[cloud]` config block (or `PITHOS_CLOUD_` environment variables) and automatically export the canonical `POWERWORD_` and `GOOGLE_` environment variables so the `pw-mcp-cloud` plugin client inherits them dynamically.

---

## Proposed Changes

### Configuration Layer

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add `CloudConfig` struct and field to `Config`:
  ```go
  type Config struct {
      ...
      Cloud CloudConfig `mapstructure:"cloud"`
  }

  type CloudConfig struct {
      Provider        string `mapstructure:"provider"`
      Bucket          string `mapstructure:"bucket"`
      CredentialsPath string `mapstructure:"credentials_path"`
      ProjectID       string `mapstructure:"project_id"`
  }
  ```
- Set default values in `LoadConfig`:
  - `cloud.provider` -> `"noop"`
  - `cloud.bucket` -> `""`
  - `cloud.credentials_path` -> `""`
  - `cloud.project_id` -> `""`
- Bind environment variables:
  - `PITHOS_CLOUD_PROVIDER` -> `cloud.provider`
  - `PITHOS_CLOUD_BUCKET` -> `cloud.bucket`
  - `PITHOS_CLOUD_CREDENTIALS_PATH` -> `cloud.credentials_path`
  - `PITHOS_CLOUD_PROJECT_ID` -> `cloud.project_id`
- In `finalizeLoad`, propagate these configurations to standard environment variables inherited by MCP sub-processes:
  - Set `POWERWORD_CLOUD_PROVIDER` to `cloud.Provider`
  - Set `POWERWORD_CLOUD_BUCKET` to `cloud.Bucket`
  - Set `POWERWORD_CLOUD_CREDENTIALS_PATH` to `cloud.CredentialsPath`
  - Set `GOOGLE_APPLICATION_CREDENTIALS` to `cloud.CredentialsPath`
  - Set `GOOGLE_CLOUD_PROJECT` to `cloud.ProjectID`

---

## Verification Plan

### Automated Tests
- In `config_test.go`, assert that:
  - GCS config structures are unmarshaled correctly from TOML configuration.
  - Environment variables prefixed with `PITHOS_CLOUD_` bind correctly and are exported with `POWERWORD_` and `GOOGLE_` prefixes.

### Manual Verification
1. Configure GCP credentials in local `.pithos.toml` or `.env`:
   ```env
   PITHOS_CLOUD_PROVIDER="gcs"
   PITHOS_CLOUD_BUCKET="your-gcs-bucket"
   PITHOS_CLOUD_CREDENTIALS_PATH="/path/to/sa-key.json"
   PITHOS_CLOUD_PROJECT_ID="your-project-id"
   ```
2. Run `pithos doctor` to verify cloud connection diagnostic check succeeds.
3. Run `pithos brew --output books/cyber_diogenes` and verify that:
   - The character seed portrait is successfully generated.
   - The seed portrait is successfully uploaded to the GCS bucket.
   - Subsequent pages are generated using the returned GCS URL as the character reference.
