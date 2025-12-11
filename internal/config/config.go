package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigFileName = "config.yml"
	AppDirName     = "vget"
)

// ConfigDir returns the standard config directory for vget.
// Windows: %APPDATA%\vget\
// macOS/Linux: ~/.config/vget/
func ConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, AppDirName), nil
		}
	}

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

	// Default output directory
	OutputDir string `yaml:"output_dir,omitempty"`

	// Preferred format (e.g., "mp4", "webm", "best")
	Format string `yaml:"format,omitempty"`

	// Default quality preference (e.g., "1080p", "720p", "best")
	Quality string `yaml:"quality,omitempty"`

	// WebDAV servers configuration
	WebDAVServers map[string]WebDAVServer `yaml:"webdavServers,omitempty"`

	// Twitter/X configuration
	Twitter TwitterConfig `yaml:"twitter,omitempty"`

	// Server configuration for `vget serve`
	Server ServerConfig `yaml:"server,omitempty"`
}

// TwitterConfig holds Twitter/X authentication settings
type TwitterConfig struct {
	// AuthToken is the auth_token cookie value from browser (for NSFW content)
	AuthToken string `yaml:"auth_token,omitempty"`
}

// ServerConfig holds HTTP server settings for `vget serve`
type ServerConfig struct {
	// Port is the HTTP listen port (default: 8080)
	Port int `yaml:"port,omitempty"`

	// MaxConcurrent is the max number of concurrent downloads (default: 10)
	MaxConcurrent int `yaml:"max_concurrent,omitempty"`

	// APIKey for authentication (optional, if set all requests must include X-API-Key header)
	APIKey string `yaml:"api_key,omitempty"`
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

// DefaultDownloadDir returns the default download directory
// Windows: ~/Downloads/vget
// macOS: ~/Downloads/vget
// Linux: ~/downloads
func DefaultDownloadDir() string {
	// Docker: use the default container path (users mount their volume here)
	if isRunningInDocker() {
		return "/home/vget/downloads"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "./downloads"
	}

	switch runtime.GOOS {
	case "darwin", "windows":
		return filepath.Join(home, "Downloads", "vget")
	default:
		// Linux and others
		return filepath.Join(home, "downloads")
	}
}

// isRunningInDocker detects if we're running inside a Docker container
func isRunningInDocker() bool {
	// Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check cgroup
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
			return true
		}
	}
	// Check for kubernetes
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	return false
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Language:  "zh",
		OutputDir: DefaultDownloadDir(),
		Format:    "mp4",
		Quality:   "best",
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
		cfg = DefaultConfig()
	}
	return cfg
}

