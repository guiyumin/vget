package extractor

import (
	"net/url"
	"path"
	"strings"
)

// extractorsByHost maps hostnames to their extractors
var extractorsByHost = map[string]Extractor{}

// fallbackExtractor handles direct file URLs and unknown hosts
var fallbackExtractor Extractor

// m3u8Extractor handles m3u8 URLs specifically (no HEAD request validation)
var m3u8Extractor = &M3U8Extractor{}

// directDownloadExtensions are file extensions that bypass host-based extractors
var directDownloadExtensions = map[string]bool{
	// Video
	".mp4": true, ".webm": true, ".mov": true, ".avi": true, ".mkv": true,
	".flv": true, ".m3u8": true, ".ts": true, ".m4v": true, ".wmv": true,
	// Audio
	".mp3": true, ".m4a": true, ".aac": true, ".ogg": true, ".wav": true,
	".flac": true, ".wma": true,
	// Image
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".bmp": true, ".svg": true, ".ico": true, ".tiff": true,
	// Documents
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".csv": true, ".txt": true, ".rtf": true,
	// Ebooks
	".epub": true, ".mobi": true, ".azw": true, ".azw3": true,
	// Archives
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".rar": true, ".7z": true, ".dmg": true, ".iso": true,
}

// Register adds an extractor for the given hostnames
func Register(e Extractor, hosts ...string) {
	for _, host := range hosts {
		extractorsByHost[host] = e
	}
}

// RegisterFallback sets the fallback extractor for direct files and unknown hosts
func RegisterFallback(e Extractor) {
	fallbackExtractor = e
}

// Match finds the extractor for a URL using O(1) hostname lookup
// Returns nil for unknown hosts (caller should check sites.yml)
func Match(rawURL string) Extractor {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	// Check if it's a direct file URL first (skip host-based extractors)
	ext := strings.ToLower(path.Ext(u.Path))
	if directDownloadExtensions[ext] {
		// Use specialized m3u8 extractor for HLS streams (no HEAD validation needed)
		if ext == ".m3u8" || ext == ".m3u" {
			return m3u8Extractor
		}
		return fallbackExtractor
	}

	// Lookup by hostname
	host := strings.ToLower(u.Hostname())

	// Try exact match
	if e, ok := extractorsByHost[host]; ok {
		// Also check path pattern via Match() (e.g., /status/ for Twitter)
		if e.Match(u) {
			return e
		}
	}

	// Try without www. prefix
	if strings.HasPrefix(host, "www.") {
		if e, ok := extractorsByHost[host[4:]]; ok {
			if e.Match(u) {
				return e
			}
		}
	}

	// Unknown host - return nil so caller can check sites.yml
	return nil
}

// List returns all unique registered extractors
func List() []Extractor {
	seen := make(map[string]bool)
	var result []Extractor
	for _, e := range extractorsByHost {
		if !seen[e.Name()] {
			seen[e.Name()] = true
			result = append(result, e)
		}
	}
	return result
}
