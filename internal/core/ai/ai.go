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

// ChunkOptions configures audio chunking parameters.
type ChunkOptions struct {
	ChunkDuration time.Duration
	Overlap       time.Duration
}

// Result contains the output of pipeline processing.
type Result struct {
	ExtractedAudioPath string // Path to extracted audio (for video files)
	ChunksDir          string // Path to chunks directory (for large files)
	TranscriptPath     string
	SummaryPath        string
	Transcript         *transcriber.Result
	Summary            *summarizer.Result
}

// NewPipeline creates a new AI processing pipeline.
// The accountName specifies which AI account to use (empty for default).
// The pin is the 4-digit PIN used to decrypt the API keys.
func NewPipeline(cfg *config.Config, accountName string, pin string) (*Pipeline, error) {
	// Get the specified account (or default)
	account := cfg.AI.GetAccount(accountName)
	if account == nil {
		if accountName == "" {
			return nil, fmt.Errorf("no AI accounts configured\nRun: vget ai config")
		}
		return nil, fmt.Errorf("AI account '%s' not found\nRun: vget ai config", accountName)
	}

	return NewPipelineWithAccount(account, "", pin)
}

// NewPipelineWithAccount creates a new AI processing pipeline from an account.
// The model parameter optionally overrides the default model.
// The pin is used to decrypt the API key (empty if account uses plain text keys).
func NewPipelineWithAccount(account *config.AIAccount, model string, pin string) (*Pipeline, error) {
	if account == nil {
		return nil, fmt.Errorf("no AI account provided")
	}

	// Decrypt API key
	var apiKey string
	if strings.HasPrefix(account.APIKey, "plain:") {
		// Plain text key - no decryption needed
		apiKey = strings.TrimPrefix(account.APIKey, "plain:")
	} else {
		// Encrypted key - validate PIN and decrypt
		if err := crypto.ValidatePIN(pin); err != nil {
			return nil, fmt.Errorf("PIN required to decrypt API key: %w", err)
		}
		var err error
		apiKey, err = crypto.Decrypt(account.APIKey, pin)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt API key: %w\nHint: Check your PIN", err)
		}
	}

	// Create service config for transcriber/summarizer
	svcCfg := config.AIServiceConfig{
		Model: model,
	}

	p := &Pipeline{
		chunker: NewChunker(),
	}

	// Initialize transcriber (OpenAI is the only provider that supports transcription)
	if account.Provider == "openai" {
		t, err := transcriber.New(account.Provider, svcCfg, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create transcriber: %w", err)
		}
		p.transcriber = t
	}

	// Initialize summarizer (all providers support summarization)
	s, err := summarizer.New(account.Provider, svcCfg, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarizer: %w", err)
	}
	p.summarizer = s

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
		transcript, chunksDir, err := p.transcribe(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("transcription failed: %w", err)
		}
		result.Transcript = transcript
		result.ChunksDir = chunksDir

		// Check for extracted audio path
		if chunksDir != "" {
			manifest, _ := LoadManifest(chunksDir)
			if manifest != nil && manifest.ExtractedAudioPath != "" {
				result.ExtractedAudioPath = manifest.ExtractedAudioPath
			}
		}

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
func (p *Pipeline) transcribe(ctx context.Context, filePath string) (*transcriber.Result, string, error) {
	// Check if file needs chunking
	needsChunking, err := p.chunker.NeedsChunking(filePath)
	if err != nil {
		return nil, "", err
	}

	if !needsChunking {
		// Direct transcription
		result, err := p.transcriber.Transcribe(ctx, filePath)
		return result, "", err
	}

	// Check ffmpeg availability
	if !p.chunker.HasFFmpeg() {
		return nil, "", fmt.Errorf("large files require ffmpeg for chunking\nInstall: brew install ffmpeg (macOS) or apt install ffmpeg (Linux)")
	}

	// Split into chunks with manifest (preserves all intermediate files)
	fmt.Println("  File exceeds size limit, splitting into chunks...")
	chunks, manifest, err := p.chunker.SplitWithManifest(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to split file: %w", err)
	}

	fmt.Printf("  Created %d chunks in: %s\n", len(chunks), manifest.ChunksDir)

	// Transcribe each chunk
	var results []*transcriber.Result
	for i, chunk := range chunks {
		fmt.Printf("  [%d/%d] Transcribing chunk...\n", i+1, len(chunks))

		result, err := p.transcriber.Transcribe(ctx, chunk.FilePath)
		if err != nil {
			return nil, manifest.ChunksDir, fmt.Errorf("failed to transcribe chunk %d: %w", i+1, err)
		}
		results = append(results, result)

		// Update chunk status in manifest
		manifest.Chunks[i].Status = "transcribed"
		manifestPath := filepath.Join(manifest.ChunksDir, "manifest.json")
		p.chunker.writeManifest(manifest, manifestPath)
	}

	// Merge results
	fmt.Println("  Merging transcripts...")
	merged, err := p.chunker.MergeTranscripts(results, chunks)
	return merged, manifest.ChunksDir, err
}

// getOutputPath generates output file path with the given suffix.
// Handles special cases like .transcript.md -> .summary.md
func getOutputPath(inputPath, suffix string) string {
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(inputPath, ext)

	// Handle .transcript.md -> .summary.md case
	if before, ok :=strings.CutSuffix(base, ".transcript"); ok  {
		base = before
	}

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

// SliceOnly performs audio/video slicing without requiring API keys.
// This is a standalone operation useful as the first step before transcription.
func SliceOnly(filePath string, opts ChunkOptions) error {
	// Validate file type
	fileType := detectFileType(filePath)
	if !isAudioVideo(fileType) {
		return fmt.Errorf("--slice requires audio/video input, got %s file", fileType)
	}

	chunker := NewChunkerWithOptions(opts)

	// Check ffmpeg availability
	if !chunker.HasFFmpeg() {
		return fmt.Errorf("--slice requires ffmpeg\nInstall: brew install ffmpeg (macOS) or apt install ffmpeg (Linux)")
	}

	fmt.Printf("Slicing %s...\n", filepath.Base(filePath))
	fmt.Printf("  Chunk duration: %s, Overlap: %s\n", chunker.chunkDuration, chunker.overlapDuration)

	// Get file info for display
	needsChunking, err := chunker.NeedsChunking(filePath)
	if err != nil {
		return fmt.Errorf("failed to check file: %w", err)
	}

	if !needsChunking {
		fmt.Printf("  File is small enough for direct transcription (<%dMB)\n", MaxFileSize/(1024*1024))
		fmt.Println("  Slicing anyway for preparation...")
	}

	// Perform slicing and generate manifest
	chunks, manifest, err := chunker.SplitWithManifest(filePath)
	if err != nil {
		return fmt.Errorf("failed to slice file: %w", err)
	}

	fmt.Printf("  Created %d chunks in: %s\n", len(chunks), manifest.ChunksDir)
	fmt.Printf("  Manifest: %s/manifest.json\n", manifest.ChunksDir)
	fmt.Println("\nChunks:")
	for _, chunk := range chunks {
		fmt.Printf("  [%d] %s (%.0fs - %.0fs)\n",
			chunk.Index,
			filepath.Base(chunk.FilePath),
			chunk.Start.Seconds(),
			chunk.End.Seconds(),
		)
	}

	fmt.Println("\nComplete!")
	fmt.Printf("  Chunks directory: %s\n", manifest.ChunksDir)
	fmt.Println("  Ready for transcription with: vget ai <file> --transcribe")

	return nil
}
