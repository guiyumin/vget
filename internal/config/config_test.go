package config

import (
	"os"
	"testing"
)

// envBackup stores environment variable values for restoration
type envBackup map[string]string

// backupAndClearEnvVars backs up and clears the specified environment variables
func backupAndClearEnvVars(keys []string) envBackup {
	backup := make(envBackup)
	for _, key := range keys {
		backup[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	return backup
}

// restore restores the backed up environment variables
func (b envBackup) restore() {
	for key, value := range b {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

// proxyEnvVars is the list of proxy-related environment variables
var proxyEnvVars = []string{
	"HTTPS_PROXY", "https_proxy",
	"HTTP_PROXY", "http_proxy",
	"ALL_PROXY", "all_proxy",
}

func TestLoadEnvProxy_SchemeLess(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	os.Setenv("HTTP_PROXY", "proxy.example:8080")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	if cfg.Proxy != "proxy.example:8080" {
		t.Errorf("expected cfg.Proxy to be 'proxy.example:8080', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_Precedence(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	os.Setenv("HTTPS_PROXY", "https://secure:8443")
	os.Setenv("HTTP_PROXY", "http://other:8080")
	os.Setenv("ALL_PROXY", "socks5://fallback:1080")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	if cfg.Proxy != "https://secure:8443" {
		t.Errorf("expected cfg.Proxy to be 'https://secure:8443', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_LowercaseAndUppercase(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		expected string
	}{
		{
			name:     "uppercase HTTP_PROXY",
			envKey:   "HTTP_PROXY",
			envValue: "http://upper.example:8080",
			expected: "http://upper.example:8080",
		},
		{
			name:     "lowercase http_proxy",
			envKey:   "http_proxy",
			envValue: "http://lower.example:8080",
			expected: "http://lower.example:8080",
		},
		{
			name:     "uppercase HTTPS_PROXY",
			envKey:   "HTTPS_PROXY",
			envValue: "https://upper.example:8443",
			expected: "https://upper.example:8443",
		},
		{
			name:     "lowercase https_proxy",
			envKey:   "https_proxy",
			envValue: "https://lower.example:8443",
			expected: "https://lower.example:8443",
		},
		{
			name:     "uppercase ALL_PROXY",
			envKey:   "ALL_PROXY",
			envValue: "socks5://upper.example:1080",
			expected: "socks5://upper.example:1080",
		},
		{
			name:     "lowercase all_proxy",
			envKey:   "all_proxy",
			envValue: "socks5://lower.example:1080",
			expected: "socks5://lower.example:1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := backupAndClearEnvVars(proxyEnvVars)
			defer backup.restore()

			os.Setenv(tt.envKey, tt.envValue)

			cfg := DefaultConfig()
			loadEnvProxy(cfg)

			if cfg.Proxy != tt.expected {
				t.Errorf("expected cfg.Proxy to be '%s', got '%s'", tt.expected, cfg.Proxy)
			}
		})
	}
}

func TestLoadEnvProxy_InvalidIgnored(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	os.Setenv("HTTP_PROXY", "::not-a-proxy")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	if cfg.Proxy != "" {
		t.Errorf("expected cfg.Proxy to be '', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_NilConfig(t *testing.T) {
	// Should not panic with nil config
	loadEnvProxy(nil)
}

func TestLoadEnvProxy_WhitespaceHandling(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	os.Setenv("HTTP_PROXY", "  http://proxy.example:8080  ")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	// Should trim whitespace but preserve the original (trimmed) value
	if cfg.Proxy != "http://proxy.example:8080" {
		t.Errorf("expected cfg.Proxy to be 'http://proxy.example:8080', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_UnsupportedScheme(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	os.Setenv("HTTP_PROXY", "ftp://ftp.example:21")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	// ftp scheme should be rejected
	if cfg.Proxy != "" {
		t.Errorf("expected cfg.Proxy to be '', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_PreservesOriginalValue(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	// Value with credentials
	os.Setenv("HTTP_PROXY", "http://user:password@proxy.example:8080")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	if cfg.Proxy != "http://user:password@proxy.example:8080" {
		t.Errorf("expected cfg.Proxy to preserve original value 'http://user:password@proxy.example:8080', got '%s'", cfg.Proxy)
	}
}

func TestLoadEnvProxy_LowercasePrecedence(t *testing.T) {
	backup := backupAndClearEnvVars(proxyEnvVars)
	defer backup.restore()

	// Set lowercase https_proxy but not uppercase HTTPS_PROXY
	os.Setenv("https_proxy", "https://lowercase.example:8443")
	os.Setenv("HTTP_PROXY", "http://http.example:8080")

	cfg := DefaultConfig()
	loadEnvProxy(cfg)

	// https_proxy should be preferred over HTTP_PROXY
	if cfg.Proxy != "https://lowercase.example:8443" {
		t.Errorf("expected cfg.Proxy to be 'https://lowercase.example:8443', got '%s'", cfg.Proxy)
	}
}
