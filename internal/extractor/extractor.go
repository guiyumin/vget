package extractor

import (
	"fmt"
	"net/url"
)

// MediaType represents the type of media being downloaded
type MediaType string

const (
	MediaTypeVideo   MediaType = "video"
	MediaTypeAudio   MediaType = "audio"
	MediaTypePDF     MediaType = "pdf"
	MediaTypeEPUB    MediaType = "epub"
	MediaTypeMOBI    MediaType = "mobi"
	MediaTypeAZW     MediaType = "azw"
	MediaTypeUnknown MediaType = "unknown" // fallback, treated as video
)

// Extractor defines the interface for video extractors
type Extractor interface {
	// Name returns the extractor name (e.g., "twitter", "direct")
	Name() string

	// Match returns true if this extractor can handle the URL
	// The URL is pre-parsed so extractors can reliably check the host/domain
	Match(u *url.URL) bool

	// Extract retrieves video information from the URL
	Extract(url string) (*VideoInfo, error)
}

// VideoInfo contains extracted video metadata
type VideoInfo struct {
	ID          string
	Title       string
	Description string
	Duration    int // seconds
	Thumbnail   string
	Formats     []Format
	Uploader    string
	UploadDate  string
	MediaType   MediaType // video, audio, document, etc.
}

// Format represents a single video format/quality option
type Format struct {
	URL       string
	Quality   string // "1080p", "720p", etc.
	Ext       string // "mp4", "m3u8", "ts"
	Width     int
	Height    int
	Bitrate   int
	FileSize  int64
	VideoOnly bool
	AudioOnly bool
}

// QualityLabel returns a human-readable quality label
func (f *Format) QualityLabel() string {
	if f.Quality != "" {
		return f.Quality
	}
	if f.Height > 0 {
		return fmt.Sprintf("%dp", f.Height)
	}
	return "unknown"
}
