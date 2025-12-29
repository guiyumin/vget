package downloader

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DetectFileType reads the first few bytes of the file to determine its type
// Returns the suggested extension (without dot) if detected, or empty string if unknown
func DetectFileType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Read first 12 bytes
	// WEBP needs at least 12 bytes: "RIFF" + 4 bytes size + "WEBP"
	header := make([]byte, 12)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}
	if n < 4 {
		return "", nil // Too short
	}

	// Check magic bytes

	// WebP: RIFF....WEBP
	if n >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return "webp", nil
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if n >= 8 && bytes.Equal(header[0:8], []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "png", nil
	}

	// GIF: GIF87a or GIF89a
	if n >= 6 && (string(header[0:6]) == "GIF87a" || string(header[0:6]) == "GIF89a") {
		return "gif", nil
	}

	// JPEG: FF D8 FF
	if n >= 3 && bytes.Equal(header[0:3], []byte{0xFF, 0xD8, 0xFF}) {
		return "jpg", nil
	}

	return "", nil
}

// RenameByMagicBytes checks if the file's actual type differs from its extension
// and renames it if necessary. Returns the final path (renamed or original).
func RenameByMagicBytes(path string) string {
	detectedExt, err := DetectFileType(path)
	if err != nil || detectedExt == "" {
		return path
	}

	ext := filepath.Ext(path)
	currentExt := strings.TrimPrefix(ext, ".")
	if currentExt == "" || strings.EqualFold(currentExt, detectedExt) {
		return path
	}

	newPath := path[:len(path)-len(ext)] + "." + detectedExt
	if err := os.Rename(path, newPath); err != nil {
		return path
	}
	return newPath
}
