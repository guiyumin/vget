package downloader

import (
	"fmt"
	"io"
	"time"
)

// DefaultUserAgent is the default User-Agent header used for downloads
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Downloader handles file downloads with progress reporting
type Downloader struct {
	lang string
}

// New creates a new Downloader
func New(lang string) *Downloader {
	return &Downloader{
		lang: lang,
	}
}

// Download downloads a file from URL to the specified path using TUI
func (d *Downloader) Download(url, output, videoID string) error {
	return RunDownloadTUI(url, output, videoID, d.lang, nil)
}

// DownloadWithHeaders downloads a file from URL with custom headers
func (d *Downloader) DownloadWithHeaders(url, output, videoID string, headers map[string]string) error {
	return RunDownloadTUI(url, output, videoID, d.lang, headers)
}

// DownloadFromReader downloads from an io.ReadCloser to the specified path using TUI
// This is useful for WebDAV and other sources that provide a reader instead of URL
func (d *Downloader) DownloadFromReader(reader io.ReadCloser, size int64, output, displayID string) error {
	return RunDownloadFromReaderTUI(reader, size, output, displayID, d.lang)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "??:??"
	}
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	if m > 60 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
