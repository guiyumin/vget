package extractor

import (
	"net/url"

	"github.com/guiyumin/vget/internal/extractor/telegram"
)

// Re-export telegram package functions for external use
var (
	TelegramDownload      = telegram.Download
	TelegramSessionPath   = telegram.SessionPath
	TelegramSessionExists = telegram.SessionExists
)

// Re-export constants
const (
	TelegramDesktopAppID   = telegram.DesktopAppID
	TelegramDesktopAppHash = telegram.DesktopAppHash
)

// Re-export types
type TelegramDownloadResult = telegram.DownloadResult

// TelegramExtractor wraps the telegram.Extractor for registration
type TelegramExtractor struct {
	ext *telegram.Extractor
}

func (t *TelegramExtractor) Name() string {
	return t.ext.Name()
}

func (t *TelegramExtractor) Match(u *url.URL) bool {
	return t.ext.Match(u)
}

func (t *TelegramExtractor) Extract(urlStr string) (Media, error) {
	info, err := t.ext.Extract(urlStr)
	if err != nil {
		return nil, err
	}

	// Convert to extractor.Media interface
	if info.IsPhoto {
		return &ImageMedia{
			ID:       info.ID,
			Title:    info.Title,
			Uploader: info.Uploader,
			Images: []Image{
				{
					URL:    info.URL,
					Ext:    info.Ext,
					Width:  info.Width,
					Height: info.Height,
				},
			},
		}, nil
	}

	if info.IsAudio {
		return &AudioMedia{
			ID:       info.ID,
			Title:    info.Title,
			Uploader: info.Uploader,
			URL:      info.URL,
			Ext:      info.Ext,
		}, nil
	}

	// Default to video
	return &VideoMedia{
		ID:       info.ID,
		Title:    info.Title,
		Uploader: info.Uploader,
		Formats: []VideoFormat{
			{
				URL:    info.URL,
				Ext:    info.Ext,
				Width:  info.Width,
				Height: info.Height,
			},
		},
	}, nil
}

func init() {
	Register(&TelegramExtractor{ext: &telegram.Extractor{}},
		"t.me",
		"telegram.me",
	)
}
