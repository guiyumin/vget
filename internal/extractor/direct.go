package extractor

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// DirectExtractor handles direct file URLs (mp4, mp3, jpg, etc.)
// This is a fallback extractor that matches any URL not handled by others
type DirectExtractor struct {
	client *http.Client
}

// Name returns the extractor name
func (d *DirectExtractor) Name() string {
	return "direct"
}

// Match always returns true - this is the fallback extractor
func (d *DirectExtractor) Match(u *url.URL) bool {
	// Only match http/https URLs
	return u.Scheme == "http" || u.Scheme == "https"
}

// Extract retrieves media information from a direct URL
func (d *DirectExtractor) Extract(urlStr string) (Media, error) {
	if d.client == nil {
		d.client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Follow redirects but limit to 10
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
	}

	// HEAD request to get Content-Type and filename
	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	finalURL := resp.Request.URL.String() // URL after redirects

	// Determine media type and extension
	mediaType, ext := detectMediaType(contentType, finalURL)

	// Extract filename from URL path
	parsedURL, _ := url.Parse(finalURL)
	filename := path.Base(parsedURL.Path)
	if filename == "" || filename == "/" || filename == "." {
		filename = "download"
	}

	// Remove extension from filename for title
	title := strings.TrimSuffix(filename, "."+ext)
	if title == "" {
		title = filename
	}

	// Generate ID from URL
	id := generateID(finalURL)

	switch mediaType {
	case MediaTypeVideo:
		return &VideoMedia{
			ID:    id,
			Title: title,
			Formats: []VideoFormat{
				{
					URL: finalURL,
					Ext: ext,
				},
			},
		}, nil

	case MediaTypeAudio:
		return &AudioMedia{
			ID:    id,
			Title: title,
			URL:   finalURL,
			Ext:   ext,
		}, nil

	case MediaTypeImage:
		return &ImageMedia{
			ID:    id,
			Title: title,
			Images: []Image{
				{
					URL: finalURL,
					Ext: ext,
				},
			},
		}, nil

	default:
		// Treat unknown as video (generic file download)
		return &VideoMedia{
			ID:    id,
			Title: title,
			Formats: []VideoFormat{
				{
					URL: finalURL,
					Ext: ext,
				},
			},
		}, nil
	}
}

// detectMediaType determines the media type from Content-Type header or URL extension
func detectMediaType(contentType, urlStr string) (MediaType, string) {
	// First try Content-Type header
	contentType = strings.ToLower(strings.Split(contentType, ";")[0])

	switch {
	// Video types
	case strings.HasPrefix(contentType, "video/"):
		ext := strings.TrimPrefix(contentType, "video/")
		if ext == "mp4" || ext == "webm" || ext == "quicktime" {
			if ext == "quicktime" {
				ext = "mov"
			}
			return MediaTypeVideo, ext
		}
		return MediaTypeVideo, "mp4"

	case contentType == "application/vnd.apple.mpegurl",
		contentType == "application/x-mpegurl":
		return MediaTypeVideo, "m3u8"

	// Audio types
	case strings.HasPrefix(contentType, "audio/"):
		ext := strings.TrimPrefix(contentType, "audio/")
		switch ext {
		case "mpeg":
			ext = "mp3"
		case "mp4", "x-m4a":
			ext = "m4a"
		}
		return MediaTypeAudio, ext

	// Image types
	case strings.HasPrefix(contentType, "image/"):
		ext := strings.TrimPrefix(contentType, "image/")
		if ext == "jpeg" {
			ext = "jpg"
		}
		return MediaTypeImage, ext
	}

	// Fallback to URL extension
	parsedURL, err := url.Parse(urlStr)
	if err == nil {
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(parsedURL.Path), "."))
		switch ext {
		case "mp4", "webm", "mov", "avi", "mkv", "flv", "m3u8", "ts":
			return MediaTypeVideo, ext
		case "mp3", "m4a", "aac", "ogg", "wav", "flac":
			return MediaTypeAudio, ext
		case "jpg", "jpeg", "png", "gif", "webp", "bmp":
			if ext == "jpeg" {
				ext = "jpg"
			}
			return MediaTypeImage, ext
		case "":
			// No extension, default to binary download
			return MediaTypeVideo, "bin"
		default:
			// Unknown extension, use it as-is
			return MediaTypeVideo, ext
		}
	}

	return MediaTypeVideo, "bin"
}

// generateID creates a short ID from URL
func generateID(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "direct"
	}

	// Use last path segment as ID
	base := path.Base(parsedURL.Path)
	if base == "" || base == "/" || base == "." {
		return parsedURL.Host
	}

	// Remove extension
	if idx := strings.LastIndex(base, "."); idx > 0 {
		base = base[:idx]
	}

	// Limit length
	if len(base) > 32 {
		base = base[:32]
	}

	return base
}

func init() {
	RegisterFallback(&DirectExtractor{})
}
