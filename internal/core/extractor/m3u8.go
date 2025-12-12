package extractor

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// M3U8Extractor handles direct m3u8 playlist URLs
type M3U8Extractor struct {
	client *http.Client
}

// Name returns the extractor name
func (m *M3U8Extractor) Name() string {
	return "m3u8"
}

// Match checks if the URL is an m3u8 playlist
func (m *M3U8Extractor) Match(u *url.URL) bool {
	// Only match http/https URLs
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Check file extension
	ext := strings.ToLower(path.Ext(u.Path))
	return ext == ".m3u8" || ext == ".m3u"
}

// Extract retrieves media information from an m3u8 URL
func (m *M3U8Extractor) Extract(urlStr string) (Media, error) {
	if m.client == nil {
		m.client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}
	}

	// Parse URL to extract filename
	parsedURL, _ := url.Parse(urlStr)
	filename := path.Base(parsedURL.Path)
	title := strings.TrimSuffix(filename, path.Ext(filename))
	if title == "" {
		title = "stream"
	}

	// Generate ID from URL
	id := generateM3U8ID(urlStr)

	return &VideoMedia{
		ID:    id,
		Title: title,
		Formats: []VideoFormat{
			{
				URL: urlStr,
				Ext: "m3u8",
			},
		},
	}, nil
}

// generateM3U8ID creates a short ID from URL
func generateM3U8ID(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "m3u8"
	}

	base := path.Base(parsedURL.Path)
	if base == "" || base == "/" || base == "." {
		return parsedURL.Host
	}

	// Remove extension
	if idx := strings.LastIndex(base, "."); idx > 0 {
		base = base[:idx]
	}

	if len(base) > 32 {
		base = base[:32]
	}

	return base
}

