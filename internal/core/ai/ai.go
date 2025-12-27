// Package ai provides AI-powered transcription and summarization for audio/video files.
package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/core/ai/output"
	"github.com/guiyumin/vget/internal/core/ai/summarizer"
	"github.com/guiyumin/vget/internal/core/ai/transcriber"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/crypto"
)

// Pipeline processes audio/video files through transcription and summarization.
type Pipeline struct {
	config      *config.Config
	transcriber transcriber.Transcriber
	summarizer  summarizer.Summarizer
	chunker     *Chunker
}

// Options configures the pipeline processing.
type Options struct {
	Transcribe bool
	Summarize  bool
}

// Result contains the output of pipeline processing.
type Result struct {
	TranscriptPath string
	SummaryPath    string
	Transcript     *transcriber.Result
	Summary        *summarizer.Result
}

// NewPipeline creates a new AI processing pipeline.
// The accountName specifies which AI account to use (empty for default).
// The pin is the 4-digit PIN used to decrypt the API keys.
func NewPipeline(cfg *config.Config, accountName string, pin string) (*Pipeline, error) {
	// Validate PIN format
	if err := crypto.ValidatePIN(pin); err != nil {
		return nil, err
	}

	// Get the specified account (or default)
	account := cfg.AI.GetAccount(accountName)
	if account == nil {
		if accountName == "" {
			return nil, fmt.Errorf("no AI accounts configured\nRun: vget ai config")
		}
		return nil, fmt.Errorf("AI account '%s' not found\nRun: vget ai config", accountName)
	}

	p := &Pipeline{
		config:  cfg,
		chunker: NewChunker(),
	}

	// Initialize transcriber if configured
	if account.Transcription.APIKeyEncrypted != "" {
		// Decrypt transcription API key
		apiKey, err := crypto.Decrypt(account.Transcription.APIKeyEncrypted, pin)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt transcription API key: %w\nHint: Check your PIN", err)
		}

		t, err := transcriber.New(account.Provider, account.Transcription, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create transcriber: %w", err)
		}
		p.transcriber = t
	}

	// Initialize summarizer if configured
	if account.Summarization.APIKeyEncrypted != "" {
		// Decrypt summarization API key
		apiKey, err := crypto.Decrypt(account.Summarization.APIKeyEncrypted, pin)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt summarization API key: %w\nHint: Check your PIN", err)
		}

		s, err := summarizer.New(account.Provider, account.Summarization, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create summarizer: %w", err)
		}
		p.summarizer = s
	}

	return p, nil
}

// Process runs the AI pipeline on the given file.
func (p *Pipeline) Process(ctx context.Context, filePath string, opts Options) (*Result, error) {
	result := &Result{}

	// Determine file type
	fileType := detectFileType(filePath)

	// Validate operations
	if opts.Transcribe && !isAudioVideo(fileType) {
		return nil, fmt.Errorf("--transcribe requires audio/video input, got %s file", fileType)
	}

	if opts.Summarize && !opts.Transcribe && !isText(fileType) {
		return nil, fmt.Errorf("--summarize requires text input or --transcribe flag\nHint: Add --transcribe first, or provide a text file")
	}

	if opts.Transcribe {
		if p.transcriber == nil {
			return nil, fmt.Errorf("transcription not configured\nRun: vget ai config")
		}

		fmt.Printf("Transcribing %s...\n", filepath.Base(filePath))

		// Transcribe the file
		transcript, err := p.transcribe(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("transcription failed: %w", err)
		}
		result.Transcript = transcript

		// Write transcript to file
		transcriptPath := getOutputPath(filePath, ".transcript.md")
		if err := output.WriteTranscript(transcriptPath, filePath, transcript); err != nil {
			return nil, fmt.Errorf("failed to write transcript: %w", err)
		}
		result.TranscriptPath = transcriptPath
		fmt.Printf("  Written: %s\n", transcriptPath)
	}

	if opts.Summarize {
		if p.summarizer == nil {
			return nil, fmt.Errorf("summarization not configured\nRun: vget ai config")
		}

		// Get text to summarize
		var text string
		var sourcePath string

		if result.Transcript != nil {
			// Use transcript from previous step
			text = result.Transcript.Text
			sourcePath = result.TranscriptPath
		} else {
			// Read from input file
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file: %w", err)
			}
			text = string(data)
			sourcePath = filePath
		}

		fmt.Printf("Summarizing...\n")

		// Summarize the text
		summary, err := p.summarizer.Summarize(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("summarization failed: %w", err)
		}
		result.Summary = summary

		// Write summary to file
		summaryPath := getOutputPath(filePath, ".summary.md")
		if err := output.WriteSummary(summaryPath, sourcePath, summary); err != nil {
			return nil, fmt.Errorf("failed to write summary: %w", err)
		}
		result.SummaryPath = summaryPath
		fmt.Printf("  Written: %s\n", summaryPath)
	}

	fmt.Println("\nComplete!")
	return result, nil
}

// transcribe handles the transcription process, including chunking for large files.
func (p *Pipeline) transcribe(ctx context.Context, filePath string) (*transcriber.Result, error) {
	// Check if file needs chunking
	needsChunking, err := p.chunker.NeedsChunking(filePath)
	if err != nil {
		return nil, err
	}

	if !needsChunking {
		// Direct transcription
		return p.transcriber.Transcribe(ctx, filePath)
	}

	// Check ffmpeg availability
	if !p.chunker.HasFFmpeg() {
		return nil, fmt.Errorf("large files require ffmpeg for chunking\nInstall: brew install ffmpeg (macOS) or apt install ffmpeg (Linux)")
	}

	// Split into chunks
	fmt.Println("  File exceeds size limit, splitting into chunks...")
	chunks, err := p.chunker.Split(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to split file: %w", err)
	}
	defer p.chunker.Cleanup(chunks)

	fmt.Printf("  Created %d chunks\n", len(chunks))

	// Transcribe each chunk
	var results []*transcriber.Result
	for i, chunk := range chunks {
		fmt.Printf("  [%d/%d] Transcribing chunk...\n", i+1, len(chunks))

		result, err := p.transcriber.Transcribe(ctx, chunk.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to transcribe chunk %d: %w", i+1, err)
		}
		results = append(results, result)
	}

	// Merge results
	fmt.Println("  Merging transcripts...")
	return p.chunker.MergeTranscripts(results, chunks)
}

// getOutputPath generates output file path with the given suffix.
func getOutputPath(inputPath, suffix string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)
	return base + suffix
}

// detectFileType returns the type of file based on extension.
func detectFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3", ".m4a", ".wav", ".ogg", ".flac", ".aac":
		return "audio"
	case ".mp4", ".mkv", ".webm", ".avi", ".mov":
		return "video"
	case ".md", ".txt", ".srt":
		return "text"
	default:
		return "unknown"
	}
}

func isAudioVideo(fileType string) bool {
	return fileType == "audio" || fileType == "video"
}

func isText(fileType string) bool {
	return fileType == "text"
}

// Segment represents a timestamped portion of transcript.
type Segment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}
