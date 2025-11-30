package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/i18n"
)

// Downloader handles file downloads with progress reporting
type Downloader struct {
	client *http.Client
	lang   string
}

// New creates a new Downloader
func New(lang string) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: 0, // No timeout for downloads
		},
		lang: lang,
	}
}

// Download downloads a file from URL to the specified path
func (d *Downloader) Download(url, output, videoID string) error {
	t := i18n.T(d.lang)
	fmt.Printf("%s: %s\n", t.Download.Downloading, videoID)

	// Create HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set common headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get content length
	contentLength := resp.ContentLength

	// Create output file
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Create progress reader
	progress := &ProgressReader{
		Reader:    resp.Body,
		Total:     contentLength,
		StartTime: time.Now(),
	}

	// Copy with progress
	written, err := io.Copy(file, progress)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Print final 100% progress
	progress.Current = progress.Total
	progress.printProgress()
	fmt.Println()

	fmt.Printf("%s: %s (%s)\n", t.Download.FileSaved, output, formatBytes(written))
	return nil
}

// ProgressReader wraps an io.Reader to report progress
type ProgressReader struct {
	Reader      io.Reader
	Total       int64
	Current     int64
	StartTime   time.Time
	lastPrint   time.Time
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)

	// Update progress at most 10 times per second
	if time.Since(pr.lastPrint) > 100*time.Millisecond {
		pr.printProgress()
		pr.lastPrint = time.Now()
	}

	return n, err
}

func (pr *ProgressReader) printProgress() {
	elapsed := time.Since(pr.StartTime).Seconds()
	speed := float64(pr.Current) / elapsed

	if pr.Total > 0 {
		percent := float64(pr.Current) / float64(pr.Total) * 100
		eta := time.Duration((float64(pr.Total-pr.Current) / speed)) * time.Second

		// Progress bar
		barWidth := 40
		filled := int(percent / 100 * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		fmt.Printf("\r[%s] %.1f%% | %s/%s | %s/s | ETA: %s    ",
			bar,
			percent,
			formatBytes(pr.Current),
			formatBytes(pr.Total),
			formatBytes(int64(speed)),
			formatDuration(eta),
		)
	} else {
		fmt.Printf("\r%s | %s/s    ",
			formatBytes(pr.Current),
			formatBytes(int64(speed)),
		)
	}
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
