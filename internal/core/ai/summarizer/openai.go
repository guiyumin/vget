package summarizer

import (
	"context"
	"fmt"
	"strings"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAI implements Summarizer using OpenAI GPT (official SDK).
type OpenAI struct {
	client openai.Client
	model  openai.ChatModel
}

// NewOpenAI creates a new OpenAI summarizer.
// The apiKey parameter is the decrypted API key.
func NewOpenAI(cfg config.AIServiceConfig, apiKey string) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	client := openai.NewClient(opts...)

	model := openai.ChatModel(cfg.Model)
	if cfg.Model == "" {
		model = openai.ChatModelGPT4o
	}

	return &OpenAI{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name.
func (o *OpenAI) Name() string {
	return "openai"
}

// Summarize generates a summary from the given text using OpenAI GPT.
func (o *OpenAI) Summarize(ctx context.Context, text string) (*Result, error) {
	// Truncate text if too long (GPT-4o has 128k context but we want to be efficient)
	maxChars := 100000
	if len(text) > maxChars {
		text = text[:maxChars] + "\n\n[Text truncated due to length...]"
	}

	// Create chat completion request
	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: o.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(SummarizationPrompt + text),
		},
		MaxTokens:   openai.Int(8000),
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

// parseResponse extracts summary and key points from the response.
func parseResponse(content string) *Result {
	result := &Result{
		Summary: content,
	}

	// Try to extract key points
	lines := strings.Split(content, "\n")
	var keyPoints []string
	var summaryLines []string
	inKeyPoints := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "## Key Points") || strings.HasPrefix(line, "**Key Points") {
			inKeyPoints = true
			continue
		}

		if strings.HasPrefix(line, "## Summary") || strings.HasPrefix(line, "**Summary") {
			inKeyPoints = false
			continue
		}

		if inKeyPoints {
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
				point := strings.TrimPrefix(line, "-")
				point = strings.TrimPrefix(point, "*")
				point = strings.TrimSpace(point)
				if point != "" {
					keyPoints = append(keyPoints, point)
				}
			}
		} else if !strings.HasPrefix(line, "##") && line != "" {
			summaryLines = append(summaryLines, line)
		}
	}

	if len(keyPoints) > 0 {
		result.KeyPoints = keyPoints
	}

	if len(summaryLines) > 0 {
		result.Summary = strings.Join(summaryLines, "\n")
	}

	return result
}
