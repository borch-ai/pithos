package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Version represents the application version string.
// Injected at compilation time using ldflags in the release pipeline.
var Version = "dev"

// Config represents the schema of the .pithos.toml file.
type Config struct {
	MCP       MCPConfig       `mapstructure:"mcp"`
	API       APIConfig       `mapstructure:"api"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

// MCPConfig holds paths to local Powerword MCP server binaries.
type MCPConfig struct {
	ImageGenPath string `mapstructure:"imagegen_path"`
	KDPMathPath  string `mapstructure:"kdp_math_path"`
	SEOPath      string `mapstructure:"seo_path"`
	VideoPath    string `mapstructure:"video_path"`
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
	// Load environment variables from .env if present
	if cfgFile != "" {
		loadEnvFile(filepath.Dir(cfgFile))
	} else {
		loadEnvFile(".")
	}

	v := viper.New()

	// Set default values so environment variables can bind even if keys are missing from the config file.
	v.SetDefault("mcp.imagegen_path", "")
	v.SetDefault("mcp.kdp_math_path", "")
	v.SetDefault("mcp.seo_path", "")
	v.SetDefault("mcp.video_path", "")
	v.SetDefault("api.gemini_key", "")
	v.SetDefault("api.openai_key", "")
	v.SetDefault("telemetry.lamplighter_enabled", false)
	v.SetDefault("telemetry.firebase_credentials_path", "")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// Default config locations:
		// 1. Current working directory: ./ .pithos.toml
		v.AddConfigPath(".")
		v.SetConfigName(".pithos")
		v.SetConfigType("toml")

		// 2. User config directory: ~/.config/pithos/config.toml
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "pithos"))
			// Also support searching under home dir directly
			v.AddConfigPath(home)
		}
	}

	// Environment variable configuration
	v.SetEnvPrefix("PITHOS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Fprintln(os.Stderr, "Warning: .pithos.toml not found. Running with environment variables or default settings.")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var rawConfig Config
	if err := v.Unmarshal(&rawConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
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

	rawConfig.MCP.VideoPath, err = validateOrFallbackPath(rawConfig.MCP.VideoPath, "pw-mcp-video")
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
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("configured path for %s does not exist at %s: %w", defaultName, path, err)
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

		_ = os.Setenv(key, val)
	}
}
