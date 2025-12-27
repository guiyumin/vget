// Package output provides formatters for AI output.
package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/core/ai/summarizer"
	"github.com/guiyumin/vget/internal/core/ai/transcriber"
)

// WriteTranscript writes a transcription result to a markdown file.
func WriteTranscript(outputPath, sourcePath string, result *transcriber.Result) error {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Transcript: %s\n\n", filepath.Base(sourcePath)))

	// Metadata
	if result.Duration > 0 {
		b.WriteString(fmt.Sprintf("**Duration:** %s\n", formatDuration(result.Duration)))
	}
	if result.Language != "" {
		b.WriteString(fmt.Sprintf("**Language:** %s\n", result.Language))
	}
	b.WriteString(fmt.Sprintf("**Transcribed:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("\n---\n\n")

	// Content with timestamps
	if len(result.Segments) > 0 {
		for _, seg := range result.Segments {
			timestamp := formatTimestamp(seg.Start)
			text := strings.TrimSpace(seg.Text)
			if text != "" {
				b.WriteString(fmt.Sprintf("[%s] %s\n\n", timestamp, text))
			}
		}
	} else {
		// No segments, just output the text
		b.WriteString(result.Text)
		b.WriteString("\n")
	}

	return os.WriteFile(outputPath, []byte(b.String()), 0644)
}

// WriteSummary writes a summarization result to a markdown file.
func WriteSummary(outputPath, sourcePath string, result *summarizer.Result) error {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Summary: %s\n\n", filepath.Base(sourcePath)))

	// Metadata
	b.WriteString(fmt.Sprintf("**Source:** %s\n", sourcePath))
	b.WriteString(fmt.Sprintf("**Summarized:** %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("\n---\n\n")

	// Key points (if available)
	if len(result.KeyPoints) > 0 {
		b.WriteString("## Key Points\n\n")
		for _, point := range result.KeyPoints {
			b.WriteString(fmt.Sprintf("- %s\n", point))
		}
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString(result.Summary)
	b.WriteString("\n")

	return os.WriteFile(outputPath, []byte(b.String()), 0644)
}

// formatTimestamp formats a duration as HH:MM:SS.
func formatTimestamp(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
