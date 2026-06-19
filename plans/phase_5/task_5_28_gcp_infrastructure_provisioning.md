# plan: Task 5.28: Provision GCP Cloud Storage & IAM

**Status:** Completed
**Go Version:** 1.26.4
**Date Completed:** 2026-06-18
**Unit Test Coverage:** N/A (Infrastructure Provisioning)

## Goal Description

Provision the Google Cloud Storage (GCS) and IAM components required to host visual style seeds, character profiles, and character reference illustrations. 

Since visual reference prompts (such as `cref` parameters) need to be publicly reachable by upstream model providers, the GCS bucket must be configured to allow public read access for uploaded objects. A dedicated, minimal-privilege Service Account will be created to authenticate `pw-mcp-cloud` and write to the bucket.

---

## User Review Required

> [!CAUTION]
> **Key Security**: The generated private key file (e.g. `.gcp-sa-key.json`) grants full write/delete permissions to the GCS bucket. It must **never** be checked into version control. Ensure it is added to `.gitignore`.

> [!WARNING]
> **Public Bucket Access**: This bucket is configured with public-read permissions on its objects. Do not upload sensitive or personal files to this bucket.

---

## Provisioning Guide

Follow these steps using the `gcloud` CLI tool (or corresponding Google Cloud Console actions) to set up the infrastructure.

### 1. Authenticate and Set Project
Verify your active account and configure your target project:
```bash
gcloud auth login
gcloud config set project <YOUR_PROJECT_ID>
```

### 2. Create the Cloud Storage Bucket
Create a GCS bucket in your preferred region (e.g. `us-central1`):
```bash
gcloud storage buckets create gs://pithos-books --location=us-central1
```

### 3. Enable Public Read Access
Grant the public (`allUsers`) read access to objects in the bucket so that external APIs (like OpenAI DALL-E or Midjourney) can fetch the character references:
```bash
gcloud storage buckets add-iam-policy-binding gs://pithos-books \
    --member="allUsers" \
    --role="roles/storage.objectViewer"
```

### 4. Create the Dedicated Service Account
Provision a Service Account specifically for uploading Pithos book assets:
```bash
gcloud iam service-accounts create pithos-uploader \
    --display-name="Pithos Storage Uploader"
```

### 5. Grant the Service Account Write Permissions
Grant the Service Account `Storage Object Admin` permissions to allow it to upload, read, and delete files inside the `pithos-books` bucket:
```bash
gcloud storage buckets add-iam-policy-binding gs://pithos-books \
    --member="serviceAccount:pithos-uploader@<YOUR_PROJECT_ID>.iam.gserviceaccount.com" \
    --role="roles/storage.objectAdmin"
```

### 6. Export the Service Account JSON Key
Generate and download the private key JSON file into your local Pithos workspace root directory:
```bash
gcloud iam service-accounts keys create .gcp-sa-key.json \
    --iam-account="pithos-uploader@<YOUR_PROJECT_ID>.iam.gserviceaccount.com"
```

---

## Workspace Integration

### 1. Update Git Ignore
Add the private key filename to `.gitignore` to prevent accidental commits:
```bash
echo ".gcp-sa-key.json" >> .gitignore
```

### 2. Configure Local Environment
Add the following credentials to your local `.env` or `.pithos.toml`:

#### `.env` option:
```env
PITHOS_CLOUD_PROVIDER="gcs"
PITHOS_CLOUD_BUCKET="pithos-books"
PITHOS_CLOUD_CREDENTIALS_PATH="/absolute/path/to/pithos/.gcp-sa-key.json"
PITHOS_CLOUD_PROJECT_ID="<YOUR_PROJECT_ID>"
```

#### `.pithos.toml` option:
```toml
[cloud]
provider = "gcs"
bucket = "pithos-books"
credentials_path = "/absolute/path/to/pithos/.gcp-sa-key.json"
project_id = "<YOUR_PROJECT_ID>"
```

---

## Verification Plan

### Manual Verification
1. **Pithos Pre-flight Check**:
   Run `pithos doctor` and verify that the `pw-mcp-cloud` plugin connects successfully:
   ```bash
   ./bin/pithos doctor
   ```
2. **Verify File Upload**:
   Run a test manuscript generation with `pithos brew` using character reference:
   ```bash
   ./bin/pithos brew --output books/test_book
   ```
   Check the output logs to verify the character seed portrait is generated, uploaded to the bucket, and returned as a public `https://storage.googleapis.com/pithos-books/...` URL.
