// Package transcriber provides speech-to-text transcription.
package transcriber

import (
	"context"
	"fmt"
	"time"

	"github.com/guiyumin/vget/internal/core/config"
)

// Segment represents a timestamped portion of transcript.
type Segment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// Result contains the transcription output.
type Result struct {
	RawText  string        // Original transcript from speech-to-text
	Segments []Segment     // Timestamped segments (from raw transcription)
	Language string        // Detected language
	Duration time.Duration // Audio duration
}

// FormattedText returns the transcript with timestamps in format [HH:MM:SS] Text
func (r *Result) FormattedText() string {
	if len(r.Segments) == 0 {
		return r.RawText
	}

	var result string
	for _, seg := range r.Segments {
		timestamp := formatTimestamp(seg.Start)
		result += fmt.Sprintf("[%s] %s\n", timestamp, seg.Text)
	}
	return result
}

// formatTimestamp converts duration to HH:MM:SS format
func formatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// Transcriber converts audio to text.
type Transcriber interface {
	// Transcribe converts an audio file to text.
	Transcribe(ctx context.Context, filePath string) (*Result, error)

	// Name returns the provider name.
	Name() string
}

// New creates a new Transcriber based on configuration.
// The apiKey parameter is the decrypted API key (decryption happens at runtime with user PIN).
// For local transcription, apiKey is not required.
func New(provider string, cfg config.AIServiceConfig, apiKey string) (Transcriber, error) {
	switch provider {
	case "openai":
		return NewOpenAI(cfg, apiKey)
	default:
		return nil, fmt.Errorf("unsupported transcription provider: %s", provider)
	}
}
