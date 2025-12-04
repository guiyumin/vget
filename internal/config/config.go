package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFileName = "config.yml"
	AppDirName     = "vget"
)

// ConfigDir returns the standard config directory for vget.
// All platforms: ~/.config/vget/
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", AppDirName), nil
}

// ConfigPath returns the path to the config file.
// e.g., ~/.config/vget/config.yml
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

type Config struct {
	// Language for metadata (e.g., "en", "zh", "ja")
	Language string `yaml:"language,omitempty"`

	// Proxy URL (e.g., "http://127.0.0.1:7890", "socks5://127.0.0.1:1080")
	Proxy string `yaml:"proxy,omitempty"`

	// Default output directory
	OutputDir string `yaml:"output_dir,omitempty"`

	// Preferred format (e.g., "mp4", "webm", "best")
	Format string `yaml:"format,omitempty"`

	// Default quality preference (e.g., "1080p", "720p", "best")
	Quality string `yaml:"quality,omitempty"`

	// Default output filename template
	FilenameTemplate string `yaml:"filename_template,omitempty"`

	// WebDAV servers configuration
	WebDAVServers map[string]WebDAVServer `yaml:"webdavServers,omitempty"`

	// Twitter/X configuration
	Twitter TwitterConfig `yaml:"twitter,omitempty"`
}

// TwitterConfig holds Twitter/X authentication settings
type TwitterConfig struct {
	// AuthToken is the auth_token cookie value from browser (for NSFW content)
	AuthToken string `yaml:"auth_token,omitempty"`
}

// WebDAVServer represents a WebDAV server configuration
type WebDAVServer struct {
	// URL is the WebDAV server URL (e.g., "https://pikpak.com/dav")
	URL string `yaml:"url"`

	// Username for authentication
	Username string `yaml:"username,omitempty"`

	// Password for authentication
	Password string `yaml:"password,omitempty"`
}

// GetWebDAVServer returns a WebDAV server by name, or nil if not found
func (c *Config) GetWebDAVServer(name string) *WebDAVServer {
	if c.WebDAVServers == nil {
		return nil
	}
	if s, ok := c.WebDAVServers[name]; ok {
		return &s
	}
	return nil
}

// SetWebDAVServer adds or updates a WebDAV server
func (c *Config) SetWebDAVServer(name string, server WebDAVServer) {
	if c.WebDAVServers == nil {
		c.WebDAVServers = make(map[string]WebDAVServer)
	}
	c.WebDAVServers[name] = server
}

// DeleteWebDAVServer removes a WebDAV server by name
func (c *Config) DeleteWebDAVServer(name string) {
	if c.WebDAVServers != nil {
		delete(c.WebDAVServers, name)
	}
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Language:         "en",
		Proxy:            "",
		OutputDir:        ".",
		Format:           "mp4",
		Quality:          "best",
		FilenameTemplate: "{{.ID}}.{{.Ext}}",
	}
}

// Exists checks if config file exists
func Exists() bool {
	path, err := ConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Load reads the config from ~/.config/vget/config.yml
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return cfg, nil
}

// Save writes the config to ~/.config/vget/config.yml
func Save(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	configPath, err := ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Add a header comment
	header := "# vget configuration file\n# Run 'vget init' to regenerate with defaults\n\n"
	content := header + string(data)

	return os.WriteFile(configPath, []byte(content), 0644)
}

// SavePath returns the path where config will be saved
func SavePath() string {
	if path, err := ConfigPath(); err == nil {
		return path
	}
	return "config.yml"
}

// Init creates a new config.yml with default values
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

// loadEnvProxy checks environment variables for proxy settings and applies them to cfg.
// It checks in order: HTTPS_PROXY, https_proxy, HTTP_PROXY, http_proxy, ALL_PROXY, all_proxy.
// The first valid proxy URL found is used.
// Supported schemes: http, https, socks5. Scheme-less values are assumed to be http.
func loadEnvProxy(cfg *Config) {
	if cfg == nil {
		return
	}

	// Precedence: HTTPS_PROXY > https_proxy > HTTP_PROXY > http_proxy > ALL_PROXY > all_proxy
	envKeys := []string{
		"HTTPS_PROXY", "https_proxy",
		"HTTP_PROXY", "http_proxy",
		"ALL_PROXY", "all_proxy",
	}

	for _, key := range envKeys {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}

		// Try to parse the URL
		u, err := url.Parse(value)
		if err != nil || u.Host == "" {
			// If parsing failed or no host, try adding http:// prefix
			u, err = url.Parse("http://" + value)
			if err != nil || u.Host == "" {
				continue
			}
		}

		// Check for valid schemes
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "http", "https", "socks5":
			// Valid scheme, use the original value
			cfg.Proxy = value
			return
		default:
			// Invalid or unsupported scheme, try next env var
			continue
		}
	}
}
