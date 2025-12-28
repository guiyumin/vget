package extractor

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// MediaType represents the type of media being downloaded
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeImage MediaType = "image"
)

// Media is the interface for all extracted media types
type Media interface {
	GetID() string
	GetTitle() string
	GetUploader() string
	Type() MediaType
}

// Extractor defines the interface for media extractors
type Extractor interface {
	// Name returns the extractor name (e.g., "twitter", "direct")
	Name() string

	// Match returns true if this extractor can handle the URL
	// The URL is pre-parsed so extractors can reliably check the host/domain
	Match(u *url.URL) bool

	// Extract retrieves media information from the URL
	Extract(url string) (Media, error)
}

// VideoMedia represents video content with multiple format options
type VideoMedia struct {
	ID        string
	Title     string
	Uploader  string
	Duration  int // seconds
	Thumbnail string
	Formats   []VideoFormat
}

func (v *VideoMedia) GetID() string       { return v.ID }
func (v *VideoMedia) GetTitle() string    { return v.Title }
func (v *VideoMedia) GetUploader() string { return v.Uploader }
func (v *VideoMedia) Type() MediaType     { return MediaTypeVideo }

// VideoFormat represents a single video quality option
type VideoFormat struct {
	URL     string
	Quality string // "1080p", "720p", etc.
	Ext     string // "mp4", "m3u8", "ts"
	Width   int
	Height  int
	Bitrate int
	Headers map[string]string // Custom headers for download (e.g., Referer)
	AudioURL string // Separate audio stream URL (for adaptive formats that need merging)
}

// QualityLabel returns a human-readable quality label
func (f *VideoFormat) QualityLabel() string {
	if f.Quality != "" {
		return f.Quality
	}
	if f.Height > 0 {
		return fmt.Sprintf("%dp", f.Height)
	}
	return "unknown"
}

// AudioMedia represents audio content (podcasts, music)
type AudioMedia struct {
	ID       string
	Title    string
	Uploader string
	Duration int // seconds
	URL      string
	Ext      string // "mp3", "m4a", etc.
}

func (a *AudioMedia) GetID() string       { return a.ID }
func (a *AudioMedia) GetTitle() string    { return a.Title }
func (a *AudioMedia) GetUploader() string { return a.Uploader }
func (a *AudioMedia) Type() MediaType     { return MediaTypeAudio }

// ImageMedia represents one or more images from a single source
type ImageMedia struct {
	ID       string
	Title    string
	Uploader string
	Images   []Image
}

func (i *ImageMedia) GetID() string       { return i.ID }
func (i *ImageMedia) GetTitle() string    { return i.Title }
func (i *ImageMedia) GetUploader() string { return i.Uploader }
func (i *ImageMedia) Type() MediaType     { return MediaTypeImage }

// MultiVideoMedia represents multiple videos from a single source (e.g., Twitter multi-video tweets)
type MultiVideoMedia struct {
	ID       string
	Title    string
	Uploader string
	Videos   []*VideoMedia
}

func (m *MultiVideoMedia) GetID() string       { return m.ID }
func (m *MultiVideoMedia) GetTitle() string    { return m.Title }
func (m *MultiVideoMedia) GetUploader() string { return m.Uploader }
func (m *MultiVideoMedia) Type() MediaType     { return MediaTypeVideo }

// Image represents a single image to download
type Image struct {
	URL    string
	Ext    string // "jpg", "png", "webp"
	Width  int
	Height int
}

// SanitizeFilename removes or replaces characters that are invalid in filenames
func SanitizeFilename(name string) string {
	// Remove URLs first (before character replacement mangles them)
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	result := urlRegex.ReplaceAllString(name, "")

	// Replace characters that are problematic in filenames
	// Includes both ASCII and full-width (CJK) versions of reserved characters
	replacer := strings.NewReplacer(
		// ASCII versions
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
		"\n", " ",
		"\r", "",
		"\t", " ",
		// Full-width versions (common in Chinese/Japanese text)
		"：", "-", // U+FF1A Full-width colon
		"／", "-", // U+FF0F Full-width solidus
		"＼", "-", // U+FF3C Full-width reverse solidus
		"。", "-", // U+3002 CJK full stop
		"＊", "",  // U+FF0A Full-width asterisk
		"？", "",  // U+FF1F Full-width question mark
		"＂", "",  // U+FF02 Full-width quotation mark
		"＜", "",  // U+FF1C Full-width less-than
		"＞", "",  // U+FF1E Full-width greater-than
		"｜", "",  // U+FF5C Full-width vertical line
		// Additional problematic characters
		"「", "",  // CJK left corner bracket
		"」", "",  // CJK right corner bracket
		"【", "",  // CJK left black lenticular bracket
		"】", "",  // CJK right black lenticular bracket
	)
	result = replacer.Replace(result)

	// Remove control characters (0x00-0x1F, 0x7F) which are invalid on Windows
	result = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1 // -1 means delete the rune
		}
		return r
	}, result)

	// Trim spaces and dots from ends
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")

	// Collapse multiple spaces
	spaceRegex := regexp.MustCompile(`\s+`)
	result = spaceRegex.ReplaceAllString(result, " ")

	// Limit length to avoid "file name too long" errors
	// Most filesystems limit filenames to 255 bytes. For UTF-8 with CJK characters
	// (3-4 bytes each), 60 runes is safe (~180-240 bytes), leaving room for extension.
	const maxRunes = 60
	runes := []rune(result)
	if len(runes) > maxRunes {
		result = string(runes[:maxRunes])
	}

	// If result is empty after sanitization, return empty
	result = strings.TrimSpace(result)

	// Handle Windows reserved names (CON, PRN, AUX, NUL, COM1-9, LPT1-9)
	// These names (with or without extension) cannot be used as filenames on Windows
	reservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
	if slices.Contains(reservedNames, strings.ToUpper(result)) {
		result = "_" + result
	}

	return result
}
