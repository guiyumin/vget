package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const SitesFileName = "sites.yml"

// Site represents a site configuration for browser-based extraction
type Site struct {
	// Match is a substring to match against the URL (e.g., "kanav.ad")
	Match string `yaml:"match"`

	// Type is the media type to extract (e.g., "m3u8", "mp4")
	Type string `yaml:"type"`
}

// SitesConfig holds the sites configuration
type SitesConfig struct {
	Sites []Site `yaml:"sites"`
}

// LoadSites reads sites.yml from the current directory
func LoadSites() (*SitesConfig, error) {
	data, err := os.ReadFile(SitesFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No sites.yml, that's fine
		}
		return nil, fmt.Errorf("failed to read %s: %w", SitesFileName, err)
	}

	cfg := &SitesConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", SitesFileName, err)
	}

	return cfg, nil
}

// SaveSites writes sites.yml to the current directory
func SaveSites(cfg *SitesConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize sites config: %w", err)
	}

	header := "# vget sites configuration\n# Sites that require browser-based extraction\n# Run 'vget config sites' to manage\n\n"
	content := header + string(data)

	return os.WriteFile(SitesFileName, []byte(content), 0644)
}

// MatchSite finds a matching site for the given URL
func (c *SitesConfig) MatchSite(url string) *Site {
	if c == nil {
		return nil
	}
	for i := range c.Sites {
		if strings.Contains(url, c.Sites[i].Match) {
			return &c.Sites[i]
		}
	}
	return nil
}

// AddSite adds a new site configuration
func (c *SitesConfig) AddSite(match, mediaType string) {
	c.Sites = append(c.Sites, Site{
		Match: match,
		Type:  mediaType,
	})
}

// RemoveSite removes a site by match string
func (c *SitesConfig) RemoveSite(match string) bool {
	for i := range c.Sites {
		if c.Sites[i].Match == match {
			c.Sites = append(c.Sites[:i], c.Sites[i+1:]...)
			return true
		}
	}
	return false
}

// SitesExist checks if sites.yml exists in current directory
func SitesExist() bool {
	_, err := os.Stat(SitesFileName)
	return err == nil
}
