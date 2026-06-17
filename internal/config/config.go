package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/borch-ai/powerword/pkg/telemetry"
	"github.com/spf13/viper"
)

// Version represents the application version string.
// Injected at compilation time using ldflags in the release pipeline.
var Version = "dev"

// Config represents the schema of the .pithos.toml file.
type Config struct {
	Concurrency int                               `mapstructure:"concurrency"`
	MCP         MCPConfig                         `mapstructure:"mcp"`
	API         APIConfig                         `mapstructure:"api"`
	Telemetry   TelemetryConfig                   `mapstructure:"telemetry"`
	Pricing     map[string]telemetry.ModelPricing `mapstructure:"pricing"`
}

// MCPConfig holds paths to local Powerword MCP server binaries.
type MCPConfig struct {
	ImageGenPath string `mapstructure:"imagegen_path"`
	KDPMathPath  string `mapstructure:"kdp_math_path"`
	SEOPath      string `mapstructure:"seo_path"`
	ViralPath    string `mapstructure:"viral_path"`
	TypstPath    string `mapstructure:"typst_path"`
	CloudPath    string `mapstructure:"cloud_path"`
}

// APIConfig holds API keys for LLM and content services.
type APIConfig struct {
	GeminiKey string `mapstructure:"gemini_key"`
	OpenAIKey string `mapstructure:"openai_key"`
}

// TelemetryConfig holds WebRTC/Firebase parameters for mobile notifications.
type TelemetryConfig struct {
	LamplighterEnabled      bool   `mapstructure:"lamplighter_enabled"`
	FirebaseCredentialsPath string `mapstructure:"firebase_credentials_path"`
}

// Cfg is the global configuration instance.
var Cfg *Config

// LoadConfig initializes Viper and loads configuration from the specified file,
// fallback locations, or environment variables.
func LoadConfig(cfgFile string) (*Config, error) {
	v := viper.New()

	// Set default values so environment variables can bind even if keys are missing from the config file.
	v.SetDefault("concurrency", 1)
	v.SetDefault("mcp.imagegen_path", "")
	v.SetDefault("mcp.kdp_math_path", "")
	v.SetDefault("mcp.seo_path", "")
	v.SetDefault("mcp.viral_path", "")
	v.SetDefault("mcp.typst_path", "")
	v.SetDefault("mcp.cloud_path", "")
	v.SetDefault("api.gemini_key", "")
	v.SetDefault("api.openai_key", "")
	v.SetDefault("telemetry.lamplighter_enabled", false)
	v.SetDefault("telemetry.firebase_credentials_path", "")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		loadEnvFile(filepath.Dir(cfgFile))
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		return finalizeLoad(v)
	}

	// Default config locations:
	// 1. Current working directory: ./ .pithos.toml
	v.AddConfigPath(".")
	v.SetConfigName(".pithos")
	v.SetConfigType("toml")

	// 2. User config directory: ~/.config/pithos/config.toml
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "pithos"))
		v.AddConfigPath(home)
	}

	err := v.ReadInConfig()
	if err == nil {
		// A config file was successfully loaded. Load sibling .env file.
		envDir := "."
		if used := v.ConfigFileUsed(); used != "" {
			envDir = filepath.Dir(used)
		}
		loadEnvFile(envDir)
		return finalizeLoad(v)
	}

	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		fmt.Fprintln(os.Stderr, "Warning: .pithos.toml not found. Running with environment variables or default settings.")
		loadEnvFile(".")
		return finalizeLoad(v)
	}

	return nil, fmt.Errorf("failed to read config file: %w", err)
}

// finalizeLoad unmarshals configuration and validates paths.
func finalizeLoad(v *viper.Viper) (*Config, error) {
	// Environment variable configuration
	v.SetEnvPrefix("PITHOS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var rawConfig Config
	if err := v.Unmarshal(&rawConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if len(rawConfig.Pricing) == 0 {
		rawConfig.Pricing = map[string]telemetry.ModelPricing{
			"gemini-2.5-flash": {Input: 0.075, Output: 0.30, Cached: 0.01875},
			"gpt-4o":           {Input: 5.00, Output: 15.00},
			"imagegen":         {Input: 40000.00},
		}
	}

	// Validate and assign fallback paths for MCP binaries
	var errs []string
	var err error

	rawConfig.MCP.ImageGenPath, err = validateOrFallbackPath(rawConfig.MCP.ImageGenPath, "pw-mcp-imagegen")
	if err != nil {
		errs = append(errs, err.Error())
	}

	rawConfig.MCP.KDPMathPath, err = validateOrFallbackPath(rawConfig.MCP.KDPMathPath, "pw-mcp-kdp-math")
	if err != nil {
		errs = append(errs, err.Error())
	}

	rawConfig.MCP.SEOPath, err = validateOrFallbackPath(rawConfig.MCP.SEOPath, "pw-mcp-seo")
	if err != nil {
		errs = append(errs, err.Error())
	}

	rawConfig.MCP.ViralPath, err = validateOrFallbackPath(rawConfig.MCP.ViralPath, "pw-mcp-viral")
	if err != nil {
		errs = append(errs, err.Error())
	}

	rawConfig.MCP.TypstPath, err = validateOrFallbackPath(rawConfig.MCP.TypstPath, "pw-mcp-typst")
	if err != nil {
		errs = append(errs, err.Error())
	}

	rawConfig.MCP.CloudPath, err = validateOrFallbackPath(rawConfig.MCP.CloudPath, "pw-mcp-cloud")
	if err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	Cfg = &rawConfig
	return Cfg, nil
}

// validateOrFallbackPath checks if path is empty (returning fallback) or validates its existence.
func validateOrFallbackPath(path string, defaultName string) (string, error) {
	if path == "" {
		return defaultName, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("configured path for %s does not exist at %s: %w", defaultName, path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("configured path for %s at %s is a directory, expected executable file", defaultName, path)
	}
	return path, nil
}

// loadEnvFile reads a .env file from the specified directory and loads its keys into system environment variables.
func loadEnvFile(dir string) {
	envPath := filepath.Join(dir, ".env")
	//nolint:gosec // ReadFile path is constructed using config directory inputs in local CLI environment
	data, err := os.ReadFile(envPath)
	if err != nil {
		return // Ignore if .env is missing
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip optional single or double quotes
		val = strings.Trim(val, `"'`)

		// Shell/CI environment variables take precedence over .env file
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
