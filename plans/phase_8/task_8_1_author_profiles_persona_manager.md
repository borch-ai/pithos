# plan: Task 8.1: Virtual Author Profiles & Persona Manager

**Status:** Open
**Go Version:** 1.26.4

Decouple the book generation and review systems from any single pen name or style. Introduce **Virtual Author Profiles** (configurable via TOML files) to manage distinct writing personas, allowing users to select or switch authors (e.g., `sinope`, `tech-academic`, `sci-fi-novelist`) during project scaffolding. Pithos will construct LLM system instructions, validation metrics, and text-to-speech voice selections dynamically based on the active author's configuration.

## User Review Required

> [!NOTE]
> **Author Assets Portability**:
> Virtual author definitions are stored as text configuration profiles. A user can share or customize an author profile by adding a simple TOML configuration file to the central `authors/` directory in their home configuration space.

## Proposed Changes

### System Configuration Layer

#### [NEW] [sinope.toml](file://../../authors/sinope.toml)
- Define the default `d. s. sinope` persona:
  ```toml
  name = "d. s. sinope"
  genre = "technical"
  persona = "A highly technical engineering leader who has cracked the human operating system..."
  tone_rules = [
      "Explicitly avoid rigid IT/software metaphors (e.g., malware, CPU cycles)",
      "Maintain a playful but ruthless, Machiavellian yet ethical perspective",
      "Translate war stories into archetypal lessons"
  ]
  tts_voice = "onyx"
  
  [formatting]
  sop_rule_count = 5
  require_bold_imperatives = true
  ```

#### [MODIFY] [config.go](file://../../internal/config/config.go)
- Add structs to load and serialize author profiles:
  ```go
  type AuthorProfile struct {
      Name        string            `toml:"name"`
      Genre       string            `toml:"genre"`
      Persona     string            `toml:"persona"`
      ToneRules   []string          `toml:"tone_rules"`
      TTSVoice    string            `toml:"tts_voice"`
      Formatting  AuthorFormatting  `toml:"formatting"`
  }

  type AuthorFormatting struct {
      SOPRuleCount          int  `toml:"sop_rule_count"`
      RequireBoldImperatives bool `toml:"require_bold_imperatives"`
  }
  ```

### Manifest Layer

#### [MODIFY] [manifest.go](file://../../internal/manifest/manifest.go)
- Update `BookProperties` to track the active author profile details:
  ```go
  type BookProperties struct {
      ...
      AuthorProfile AuthorProfile `json:"author_profile"` // Embedded active author profile
  }
  ```

### CLI Command Layer

#### [MODIFY] [initiate.go](file://../../cmd/pithos/initiate.go)
- Add `--author` flag to `initiate`:
  ```bash
  pithos initiate --output [dir] --author sinope
  ```
- On execution, search the global `authors/` directory, locate the target profile, load it, and save the profile structure directly into the `manifest.json`.

### Pipeline Core

#### [MODIFY] [brew.go](file://../../internal/pipeline/brew.go)
- Update LLM system instructions construction:
  - Rather than hardcoding the `sinope` prompt text, dynamically format the prompt:
    * Set prompt role: *"Act as my ghostwriter writing under the pen name [Profile.Name] (Persona: [Profile.Persona])"*
    * Append tone instructions from `Profile.ToneRules`.
    * Inject formatting constraints from `Profile.Formatting`.

---

## Verification Plan

### Automated Tests
- Run unit and integration tests:
  ```bash
  make test
  ```
- Add unit tests verifying:
  * Profile loader successfully parses author TOML configurations.
  * System prompt builder dynamically generates expected constraints based on loaded profiles.

### Manual Verification
1. Place a new profile `authors/new_author.toml` in your configuration folder.
2. Initialize a book specifying the new author:
   ```bash
   ./bin/pithos initiate --output custom-author-test --author new_author
   ```
3. Run `brew` to write a chapter and verify that the generated text aligns with the custom persona details defined in the new TOML file.
