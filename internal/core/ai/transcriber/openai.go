package transcriber

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/guiyumin/vget/internal/core/config"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAI implements Transcriber using OpenAI Whisper API.
type OpenAI struct {
	client *openai.Client
	model  string
}

// NewOpenAI creates a new OpenAI transcriber.
// The apiKey parameter is the decrypted API key.
func NewOpenAI(cfg config.AIServiceConfig, apiKey string) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided")
	}

	clientConfig := openai.DefaultConfig(apiKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	model := cfg.Model
	if model == "" {
		model = "whisper-1"
	}

	return &OpenAI{
		client: openai.NewClientWithConfig(clientConfig),
		model:  model,
	}, nil
}

// Name returns the provider name.
func (o *OpenAI) Name() string {
	return "openai"
}

// Transcribe converts an audio file to text using OpenAI Whisper.
func (o *OpenAI) Transcribe(ctx context.Context, filePath string) (*Result, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create transcription request with verbose output for timestamps
	req := openai.AudioRequest{
		Model:    o.model,
		FilePath: filePath,
		Format:   openai.AudioResponseFormatVerboseJSON,
	}

	// Call OpenAI API
	resp, err := o.client.CreateTranscription(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("transcription API error: %w", err)
	}

	// Convert response to Result
	result := &Result{
		Text:     resp.Text,
		Language: resp.Language,
		Duration: time.Duration(resp.Duration * float64(time.Second)),
	}

	// Convert segments
	for _, seg := range resp.Segments {
		result.Segments = append(result.Segments, Segment{
			Start: time.Duration(seg.Start * float64(time.Second)),
			End:   time.Duration(seg.End * float64(time.Second)),
			Text:  seg.Text,
		})
	}

	return result, nil
}
