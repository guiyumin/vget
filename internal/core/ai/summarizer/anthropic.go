package summarizer

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/guiyumin/vget/internal/core/config"
)

// Anthropic implements Summarizer using Anthropic Claude.
type Anthropic struct {
	client *anthropic.Client
	model  string
}

// NewAnthropic creates a new Anthropic summarizer.
// The apiKey parameter is the decrypted API key.
func NewAnthropic(cfg config.AIServiceConfig, apiKey string) (*Anthropic, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key not provided")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	return &Anthropic{
		client: &client,
		model:  model,
	}, nil
}

// Name returns the provider name.
func (a *Anthropic) Name() string {
	return "anthropic"
}

// Summarize generates a summary from the given text using Anthropic Claude.
func (a *Anthropic) Summarize(ctx context.Context, text string) (*Result, error) {
	// Truncate text if too long (Claude has 200k context but we want to be efficient)
	maxChars := 150000
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n[Text truncated due to length...]"
	}

	// Create message request
	message, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: 8000,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(SummarizationPrompt + text)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("summarization API error: %w", err)
	}

	// Extract text from response
	var content string
	for _, block := range message.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	if content == "" {
		return nil, fmt.Errorf("no response from API")
	}

	// Parse response
	return parseResponse(content), nil
}
