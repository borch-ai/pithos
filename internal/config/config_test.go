package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any environment variables that might interfere
	_ = os.Unsetenv("PITHOS_API_GEMINI_KEY")
	_ = os.Unsetenv("PITHOS_API_OPENAI_KEY")
	_ = os.Unsetenv("PITHOS_MCP_IMAGEGEN_PATH")
	_ = os.Unsetenv("PITHOS_MCP_KDP_MATH_PATH")
	_ = os.Unsetenv("PITHOS_MCP_SEO_PATH")
	_ = os.Unsetenv("PITHOS_MCP_VIRAL_PATH")
	_ = os.Unsetenv("PITHOS_MCP_TYPST_PATH")
	_ = os.Unsetenv("PITHOS_MCP_CLOUD_PATH")
	_ = os.Unsetenv("PITHOS_MCP_IMAGEGEN_FORCE_CREF")
	_ = os.Unsetenv("PITHOS_MCP_IMAGEGEN_FORCE_SREF")

	// Load with empty string config path to trigger fallback/warning and defaults
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error with empty config path, got: %v", err)
	}

	if cfg.MCP.ImageGenForceCref {
		t.Errorf("expected default ImageGenForceCref to be false, got true")
	}
	if cfg.MCP.ImageGenForceSref {
		t.Errorf("expected default ImageGenForceSref to be false, got true")
	}

	if cfg.MCP.ImageGenPath != "pw-mcp-imagegen" {
		t.Errorf("expected default ImageGenPath 'pw-mcp-imagegen', got: '%s'", cfg.MCP.ImageGenPath)
	}
	if cfg.MCP.KDPMathPath != "pw-mcp-kdp-math" {
		t.Errorf("expected default KDPMathPath 'pw-mcp-kdp-math', got: '%s'", cfg.MCP.KDPMathPath)
	}
	if cfg.MCP.SEOPath != "pw-mcp-seo" {
		t.Errorf("expected default SEOPath 'pw-mcp-seo', got: '%s'", cfg.MCP.SEOPath)
	}
	if cfg.MCP.ViralPath != "pw-mcp-viral" {
		t.Errorf("expected default ViralPath 'pw-mcp-viral', got: '%s'", cfg.MCP.ViralPath)
	}
	if cfg.MCP.TypstPath != "pw-mcp-typst" {
		t.Errorf("expected default TypstPath 'pw-mcp-typst', got: '%s'", cfg.MCP.TypstPath)
	}
	if cfg.MCP.CloudPath != "pw-mcp-cloud" {
		t.Errorf("expected default CloudPath 'pw-mcp-cloud', got: '%s'", cfg.MCP.CloudPath)
	}
}

func TestLoadConfig_MissingFileExplicit(t *testing.T) {
	// If a config file is explicitly passed and missing, it should error
	_, err := LoadConfig("non_existent_file.toml")
	if err == nil {
		t.Fatal("expected error with explicit non-existent file, got nil")
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create dummy files for MCP paths to pass validation
	dummyImagegen := filepath.Join(tmpDir, "dummy-imagegen")
	if wErr := os.WriteFile(dummyImagegen, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write dummy file: %v", wErr)
	}

	tomlContent := `
[mcp]
imagegen_path = "` + dummyImagegen + `"
kdp_math_path = ""
seo_path = ""
viral_path = ""

[api]
gemini_key = "test-gemini-key"
openai_key = "test-openai-key"

[telemetry]
lamplighter_enabled = true
firebase_credentials_path = "/path/to/firebase.json"
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(tomlContent), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.MCP.ImageGenPath != dummyImagegen {
		t.Errorf("expected ImageGenPath '%s', got '%s'", dummyImagegen, cfg.MCP.ImageGenPath)
	}
	if cfg.MCP.KDPMathPath != "pw-mcp-kdp-math" {
		t.Errorf("expected fallback 'pw-mcp-kdp-math', got '%s'", cfg.MCP.KDPMathPath)
	}
	if cfg.API.GeminiKey != "test-gemini-key" {
		t.Errorf("expected gemini_key 'test-gemini-key', got '%s'", cfg.API.GeminiKey)
	}
	if !cfg.Telemetry.LamplighterEnabled {
		t.Errorf("expected lamplighter_enabled true")
	}
	if cfg.Telemetry.FirebaseCredentialsPath != "/path/to/firebase.json" {
		t.Errorf("expected firebase_credentials_path '/path/to/firebase.json', got '%s'", cfg.Telemetry.FirebaseCredentialsPath)
	}
}

func TestLoadConfig_InvalidPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tomlContent := `
[mcp]
imagegen_path = "/nonexistent/path/to/imagegen"
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(tomlContent), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Fatal("expected error due to nonexistent MCP path, got nil")
	}
}

func TestLoadConfig_DirectoryPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_test_dir")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// We pass the temp directory itself as the imagegen_path
	tomlContent := `
[mcp]
imagegen_path = "` + tmpDir + `"
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(tomlContent), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Fatal("expected error since imagegen_path is a directory, got nil")
	}
}

func TestLoadConfig_EnvVars(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	_ = os.Setenv("PITHOS_API_GEMINI_KEY", "env-gemini-key")
	defer func() {
		_ = os.Unsetenv("PITHOS_API_GEMINI_KEY")
	}()

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.API.GeminiKey != "env-gemini-key" {
		t.Errorf("expected API gemini key to be overridden by env var, got '%s'", cfg.API.GeminiKey)
	}
}

func TestLoadConfig_EnvFileLoading(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_env_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	_ = os.Unsetenv("PITHOS_API_GEMINI_KEY")
	_ = os.Unsetenv("PITHOS_API_OPENAI_KEY")

	envContent := `
# Comment line
PITHOS_API_GEMINI_KEY="file-gemini-key"
PITHOS_API_OPENAI_KEY='file-openai-key'
INVALID_LINE_NO_EQUALS
`
	envFile := filepath.Join(tmpDir, ".env")
	if wErr := os.WriteFile(envFile, []byte(envContent), 0600); wErr != nil {
		t.Fatalf("failed to write .env file: %v", wErr)
	}

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Fatalf("expected no load error, got: %v", loadErr)
	}

	if cfg.API.GeminiKey != "file-gemini-key" {
		t.Errorf("expected API GeminiKey 'file-gemini-key' loaded from .env, got '%s'", cfg.API.GeminiKey)
	}
	if cfg.API.OpenAIKey != "file-openai-key" {
		t.Errorf("expected API OpenAIKey 'file-openai-key' loaded from .env, got '%s'", cfg.API.OpenAIKey)
	}
}

func TestLoadConfig_EnvPrecedence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_precedence_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	_ = os.Setenv("PITHOS_API_GEMINI_KEY", "env-takes-precedence")
	defer func() {
		_ = os.Unsetenv("PITHOS_API_GEMINI_KEY")
	}()

	envContent := `PITHOS_API_GEMINI_KEY="dotenv-value"`
	envFile := filepath.Join(tmpDir, ".env")
	if wErr := os.WriteFile(envFile, []byte(envContent), 0600); wErr != nil {
		t.Fatalf("failed to write .env: %v", wErr)
	}

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no load error, got: %v", err)
	}

	if cfg.API.GeminiKey != "env-takes-precedence" {
		t.Errorf("expected API GeminiKey 'env-takes-precedence' to take precedence over .env file 'dotenv-value', got '%s'", cfg.API.GeminiKey)
	}
}

func TestLoadConfig_FallbackConfigSuccess(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "mock_home")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpHome)
	}()

	// Mock HOME directory
	t.Setenv("HOME", tmpHome)

	// Create ~/.config/pithos/config.toml
	confDir := filepath.Join(tmpHome, ".config", "pithos")
	if mkdirErr := os.MkdirAll(confDir, 0700); mkdirErr != nil {
		t.Fatalf("failed to create conf dir: %v", mkdirErr)
	}

	tomlContent := `
[mcp]
imagegen_path = ""
`
	if writeErr := os.WriteFile(filepath.Join(confDir, ".pithos.toml"), []byte(tomlContent), 0600); writeErr != nil {
		t.Fatalf("failed to write config: %v", writeErr)
	}

	// Create sibling .env in the same dir
	envContent := `PITHOS_API_GEMINI_KEY="fallback-env-key"`
	if writeErr := os.WriteFile(filepath.Join(confDir, ".env"), []byte(envContent), 0600); writeErr != nil {
		t.Fatalf("failed to write env: %v", writeErr)
	}

	_ = os.Unsetenv("PITHOS_API_GEMINI_KEY")

	// Load with empty string (triggers fallback paths)
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.API.GeminiKey != "fallback-env-key" {
		t.Errorf("expected GeminiKey 'fallback-env-key' loaded from fallback config's sibling .env, got '%s'", cfg.API.GeminiKey)
	}
}

func TestLoadConfig_InvalidToml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte("invalid toml structure {[[["), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	_, err = LoadConfig(configFile)
	if err == nil {
		t.Fatal("expected error loading invalid TOML, got nil")
	}
}

func TestLoadConfig_CloudConfig_Toml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_cloud_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Clear environment variables first to avoid pollution
	_ = os.Unsetenv("PITHOS_CLOUD_PROVIDER")
	_ = os.Unsetenv("PITHOS_CLOUD_BUCKET")
	_ = os.Unsetenv("PITHOS_CLOUD_CREDENTIALS_PATH")
	_ = os.Unsetenv("PITHOS_CLOUD_PROJECT_ID")
	_ = os.Unsetenv("POWERWORD_CLOUD_PROVIDER")
	_ = os.Unsetenv("POWERWORD_CLOUD_BUCKET")
	_ = os.Unsetenv("POWERWORD_CLOUD_CREDENTIALS_PATH")
	_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
	_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")

	defer func() {
		_ = os.Unsetenv("PITHOS_CLOUD_PROVIDER")
		_ = os.Unsetenv("PITHOS_CLOUD_BUCKET")
		_ = os.Unsetenv("PITHOS_CLOUD_CREDENTIALS_PATH")
		_ = os.Unsetenv("PITHOS_CLOUD_PROJECT_ID")
		_ = os.Unsetenv("POWERWORD_CLOUD_PROVIDER")
		_ = os.Unsetenv("POWERWORD_CLOUD_BUCKET")
		_ = os.Unsetenv("POWERWORD_CLOUD_CREDENTIALS_PATH")
		_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	}()

	tomlContent := `
[cloud]
provider = "gcs"
bucket = "my-test-bucket"
credentials_path = "/my/creds/path.json"
project_id = "my-gcp-project"
`
	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(tomlContent), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no load error, got: %v", err)
	}

	// Verify unmarshaling from TOML
	if cfg.Cloud.Provider != "gcs" {
		t.Errorf("expected cloud.provider 'gcs', got '%s'", cfg.Cloud.Provider)
	}
	if cfg.Cloud.Bucket != "my-test-bucket" {
		t.Errorf("expected cloud.bucket 'my-test-bucket', got '%s'", cfg.Cloud.Bucket)
	}
	if cfg.Cloud.CredentialsPath != "/my/creds/path.json" {
		t.Errorf("expected cloud.credentials_path '/my/creds/path.json', got '%s'", cfg.Cloud.CredentialsPath)
	}
	if cfg.Cloud.ProjectID != "my-gcp-project" {
		t.Errorf("expected cloud.project_id 'my-gcp-project', got '%s'", cfg.Cloud.ProjectID)
	}

	// Verify propagation to POWERWORD_ and GOOGLE_ env variables
	if os.Getenv("POWERWORD_CLOUD_PROVIDER") != "gcs" {
		t.Errorf("expected POWERWORD_CLOUD_PROVIDER 'gcs', got '%s'", os.Getenv("POWERWORD_CLOUD_PROVIDER"))
	}
	if os.Getenv("POWERWORD_CLOUD_BUCKET") != "my-test-bucket" {
		t.Errorf("expected POWERWORD_CLOUD_BUCKET 'my-test-bucket', got '%s'", os.Getenv("POWERWORD_CLOUD_BUCKET"))
	}
	if os.Getenv("POWERWORD_CLOUD_CREDENTIALS_PATH") != "/my/creds/path.json" {
		t.Errorf("expected POWERWORD_CLOUD_CREDENTIALS_PATH '/my/creds/path.json', got '%s'", os.Getenv("POWERWORD_CLOUD_CREDENTIALS_PATH"))
	}
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "/my/creds/path.json" {
		t.Errorf("expected GOOGLE_APPLICATION_CREDENTIALS '/my/creds/path.json', got '%s'", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "my-gcp-project" {
		t.Errorf("expected GOOGLE_CLOUD_PROJECT 'my-gcp-project', got '%s'", os.Getenv("GOOGLE_CLOUD_PROJECT"))
	}
}

func TestLoadConfig_CloudConfig_EnvOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_cloud_env_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	_ = os.Setenv("PITHOS_CLOUD_PROVIDER", "gcp")
	_ = os.Setenv("PITHOS_CLOUD_BUCKET", "env-bucket")
	_ = os.Setenv("PITHOS_CLOUD_CREDENTIALS_PATH", "/env/creds.json")
	_ = os.Setenv("PITHOS_CLOUD_PROJECT_ID", "env-project")

	defer func() {
		_ = os.Unsetenv("PITHOS_CLOUD_PROVIDER")
		_ = os.Unsetenv("PITHOS_CLOUD_BUCKET")
		_ = os.Unsetenv("PITHOS_CLOUD_CREDENTIALS_PATH")
		_ = os.Unsetenv("PITHOS_CLOUD_PROJECT_ID")
		_ = os.Unsetenv("POWERWORD_CLOUD_PROVIDER")
		_ = os.Unsetenv("POWERWORD_CLOUD_BUCKET")
		_ = os.Unsetenv("POWERWORD_CLOUD_CREDENTIALS_PATH")
		_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		_ = os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	}()

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no load error, got: %v", err)
	}

	if cfg.Cloud.Provider != "gcp" {
		t.Errorf("expected overridden cloud.provider 'gcp', got '%s'", cfg.Cloud.Provider)
	}
	if cfg.Cloud.Bucket != "env-bucket" {
		t.Errorf("expected overridden cloud.bucket 'env-bucket', got '%s'", cfg.Cloud.Bucket)
	}
	if cfg.Cloud.CredentialsPath != "/env/creds.json" {
		t.Errorf("expected overridden cloud.credentials_path '/env/creds.json', got '%s'", cfg.Cloud.CredentialsPath)
	}
	if cfg.Cloud.ProjectID != "env-project" {
		t.Errorf("expected overridden cloud.project_id 'env-project', got '%s'", cfg.Cloud.ProjectID)
	}
}

func TestLoadConfig_ImageGenForceOverrides(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pithos_overrides_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	_ = os.Setenv("PITHOS_MCP_IMAGEGEN_FORCE_CREF", "true")
	_ = os.Setenv("PITHOS_MCP_IMAGEGEN_FORCE_SREF", "true")
	defer func() {
		_ = os.Unsetenv("PITHOS_MCP_IMAGEGEN_FORCE_CREF")
		_ = os.Unsetenv("PITHOS_MCP_IMAGEGEN_FORCE_SREF")
	}()

	configFile := filepath.Join(tmpDir, "config.toml")
	if wErr := os.WriteFile(configFile, []byte(""), 0600); wErr != nil {
		t.Fatalf("failed to write config file: %v", wErr)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("expected no load error, got: %v", err)
	}

	if !cfg.MCP.ImageGenForceCref {
		t.Errorf("expected MCP.ImageGenForceCref to be true, got false")
	}
	if !cfg.MCP.ImageGenForceSref {
		t.Errorf("expected MCP.ImageGenForceSref to be true, got false")
	}
}

func TestConfig_ExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping test because home directory is not resolvable")
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"tilde only", "~", home},
		{"tilde prefix slash", "~/abc", filepath.Join(home, "abc")},
		{"absolute path", "/abs/path", "/abs/path"},
		{"relative path", "rel/path", "rel/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := expandTilde(tt.input)
			if actual != tt.expected {
				t.Errorf("expandTilde(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestBudgetConfig(t *testing.T) {
	cfg := &Config{
		Budget: BudgetConfig{
			MaxCostUSD: 5.00,
		},
	}

	if val := cfg.GetMaxCostUSD(); val != 5.00 {
		t.Errorf("expected 5.00, got %f", val)
	}

	cfg.SetMaxCostUSD(10.00)
	if val := cfg.GetMaxCostUSD(); val != 10.00 {
		t.Errorf("expected 10.00, got %f", val)
	}

	var nilCfg *Config
	if val := nilCfg.GetMaxCostUSD(); val != 0.0 {
		t.Errorf("expected 0.0, got %f", val)
	}

	nilCfg.SetMaxCostUSD(5.0) // should not panic
}
