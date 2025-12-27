// Package summarizer provides text summarization.
package summarizer

import (
	"context"
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
)

// Result contains the summarization output.
type Result struct {
	Summary   string
	KeyPoints []string
}

// Summarizer generates summaries from text.
type Summarizer interface {
	// Summarize generates a summary from the given text.
	Summarize(ctx context.Context, text string) (*Result, error)

	// Name returns the provider name.
	Name() string
}

// New creates a new Summarizer based on configuration.
// The apiKey parameter is the decrypted API key (decryption happens at runtime with user PIN).
func New(provider string, cfg config.AIServiceConfig, apiKey string) (Summarizer, error) {
	switch provider {
	case "openai":
		return NewOpenAI(cfg, apiKey)
	case "anthropic":
		return NewAnthropic(cfg, apiKey)
	case "qwen":
		return NewQwen(cfg, apiKey)
	default:
		return nil, fmt.Errorf("unsupported summarization provider: %s", provider)
	}
}
