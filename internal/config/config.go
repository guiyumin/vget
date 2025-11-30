package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFileYml  = ".vget.yml"
	ConfigFileYaml = ".vget.yaml"
)

type Config struct {
	// Default output directory
	OutputDir string `yaml:"output_dir,omitempty"`

	// Default quality preference (e.g., "1080p", "720p", "best")
	Quality string `yaml:"quality,omitempty"`

	// Default output filename template
	FilenameTemplate string `yaml:"filename_template,omitempty"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		OutputDir:        ".",
		Quality:          "best",
		FilenameTemplate: "{{.ID}}.{{.Ext}}",
	}
}

// ConfigPath returns the path to the config file that should be used.
// Priority: .vget.yml > .vget.yaml
func ConfigPath() (string, bool) {
	// .yml takes priority
	if _, err := os.Stat(ConfigFileYml); err == nil {
		return ConfigFileYml, true
	}
	if _, err := os.Stat(ConfigFileYaml); err == nil {
		return ConfigFileYaml, true
	}
	return "", false
}

// Exists checks if any config file exists in the current directory
func Exists() bool {
	_, found := ConfigPath()
	return found
}

// Load reads the config from .vget.yml or .vget.yaml
func Load() (*Config, error) {
	path, found := ConfigPath()
	if !found {
		return nil, fmt.Errorf("config file not found")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return cfg, nil
}

// Save writes the config to .vget.yml
func Save(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// Add a header comment
	header := "# vget configuration file\n# Run 'vget init' to regenerate with defaults\n\n"
	content := header + string(data)

	return os.WriteFile(ConfigFileYml, []byte(content), 0644)
}

// Init creates a new .vget.yml with default values
func Init() error {
	if Exists() {
		path, _ := ConfigPath()
		return fmt.Errorf("%s already exists", path)
	}
	return Save(DefaultConfig())
}

// LoadOrDefault loads config if it exists, otherwise returns defaults
func LoadOrDefault() *Config {
	cfg, err := Load()
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}

// FindConfigFile looks for .vget.yml or .vget.yaml in current dir and parent dirs
// Priority: .vget.yml > .vget.yaml
func FindConfigFile() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		// Check .yml first (higher priority)
		ymlPath := filepath.Join(dir, ConfigFileYml)
		if _, err := os.Stat(ymlPath); err == nil {
			return ymlPath, true
		}
		// Then check .yaml
		yamlPath := filepath.Join(dir, ConfigFileYaml)
		if _, err := os.Stat(yamlPath); err == nil {
			return yamlPath, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", false
}
