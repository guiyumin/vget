package summarizer

import (
	"context"
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	// QwenDefaultBaseURL is the OpenAI-compatible endpoint for Qwen
	QwenDefaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
)

// Qwen implements Summarizer using Alibaba Qwen via OpenAI-compatible API (official SDK).
type Qwen struct {
	client openai.Client
	model  string
}

// NewQwen creates a new Qwen summarizer.
// The apiKey parameter is the decrypted API key (DashScope API key).
func NewQwen(cfg config.AIServiceConfig, apiKey string) (*Qwen, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Qwen API key not provided")
	}

	// Use OpenAI-compatible endpoint
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = QwenDefaultBaseURL
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	}

	client := openai.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = "qwen-plus" // Default to qwen-plus (good balance of cost/performance)
	}

	return &Qwen{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name.
func (q *Qwen) Name() string {
	return "qwen"
}

// Summarize generates a summary from the given text using Qwen.
func (q *Qwen) Summarize(ctx context.Context, text string) (*Result, error) {
	// Truncate text if too long
	maxChars := 100000
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n[Text truncated due to length...]"
	}

	// Create chat completion request (OpenAI-compatible)
	resp, err := q.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModel(q.model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(summarizationPrompt + text),
		},
		MaxTokens:   openai.Int(2000),
		Temperature: openai.Float(0.3),
	})
	if err != nil {
		return nil, fmt.Errorf("summarization API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	content := resp.Choices[0].Message.Content

	// Parse response
	return parseResponse(content), nil
}
