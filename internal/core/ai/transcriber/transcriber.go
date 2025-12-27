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
	Text     string
	Segments []Segment
	Language string
	Duration time.Duration
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
func New(provider string, cfg config.AIServiceConfig, apiKey string) (Transcriber, error) {
	switch provider {
	case "openai":
		return NewOpenAI(cfg, apiKey)
	default:
		return nil, fmt.Errorf("unsupported transcription provider: %s", provider)
	}
}
