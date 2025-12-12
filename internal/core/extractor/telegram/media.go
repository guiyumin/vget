package telegram

import (
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
)

// ExtractedMedia contains extracted media information
type ExtractedMedia struct {
	Title    string
	Filename string
	Ext      string
	Size     int64
	Width    int
	Height   int
	IsVideo  bool
	IsAudio  bool
	IsPhoto  bool
}

// ExtractDocumentInfo extracts metadata from a Telegram document
func ExtractDocumentInfo(doc *tg.Document, messageText string, msgID int) *ExtractedMedia {
	var filename string
	var isVideo, isAudio bool
	var width, height int

	for _, attr := range doc.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeFilename:
			filename = a.FileName
		case *tg.DocumentAttributeVideo:
			isVideo = true
			width = a.W
			height = a.H
		case *tg.DocumentAttributeAudio:
			isAudio = true
		}
	}

	ext := ExtFromMime(doc.MimeType)
	if filename != "" {
		if idx := strings.LastIndex(filename, "."); idx > 0 {
			ext = filename[idx+1:]
		}
	}

	title := truncateText(messageText, 100)
	if title == "" {
		title = fmt.Sprintf("telegram_%d", msgID)
	}

	return &ExtractedMedia{
		Title:    title,
		Filename: filename,
		Ext:      ext,
		Size:     doc.Size,
		Width:    width,
		Height:   height,
		IsVideo:  isVideo,
		IsAudio:  isAudio,
	}
}

// FindLargestPhotoSize finds the largest photo size from available sizes
func FindLargestPhotoSize(sizes []tg.PhotoSizeClass) *tg.PhotoSize {
	var largest *tg.PhotoSize
	var largestArea int

	for _, size := range sizes {
		if ps, ok := size.(*tg.PhotoSize); ok {
			area := ps.W * ps.H
			if area > largestArea {
				largest = ps
				largestArea = area
			}
		}
	}

	return largest
}

// ExtFromMime returns file extension from MIME type
func ExtFromMime(mime string) string {
	switch mime {
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "video/quicktime":
		return "mov"
	case "audio/mpeg":
		return "mp3"
	case "audio/ogg":
		return "ogg"
	case "audio/mp4":
		return "m4a"
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "application/pdf":
		return "pdf"
	default:
		return "bin"
	}
}

func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
