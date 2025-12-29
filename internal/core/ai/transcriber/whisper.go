//go:build cgo

package transcriber

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/go-audio/wav"
	"github.com/guiyumin/vget/internal/core/config"
)

// WhisperTranscriber implements Transcriber using whisper.cpp.
type WhisperTranscriber struct {
	model     whisper.Model
	modelPath string
	language  string
}

// NewWhisperTranscriber creates a new whisper.cpp transcriber.
func NewWhisperTranscriber(modelPath, language string) (*WhisperTranscriber, error) {
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found: %s", modelPath)
	}

	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %w", err)
	}

	return &WhisperTranscriber{
		model:     model,
		modelPath: modelPath,
		language:  language,
	}, nil
}

// NewWhisperTranscriberFromConfig creates a WhisperTranscriber from config.
func NewWhisperTranscriberFromConfig(cfg config.LocalASRConfig, modelsDir string) (*WhisperTranscriber, error) {
	model := cfg.Model
	if model == "" {
		model = "whisper-small"
	}

	// Map model name to ggml file
	var modelFile string
	switch model {
	case "whisper-small":
		modelFile = "ggml-small.bin"
	case "whisper-medium":
		modelFile = "ggml-medium.bin"
	case "whisper-large-turbo":
		modelFile = "ggml-large-v3-turbo.bin"
	default:
		// Assume it's a direct path or filename
		modelFile = model
	}

	modelPath := filepath.Join(modelsDir, modelFile)

	language := cfg.Language
	if language == "" {
		language = "auto"
	}

	return NewWhisperTranscriber(modelPath, language)
}

// Name returns the provider name.
func (w *WhisperTranscriber) Name() string {
	return "whisper.cpp"
}

// Transcribe converts an audio file to text using whisper.cpp.
func (w *WhisperTranscriber) Transcribe(ctx context.Context, filePath string) (*Result, error) {
	// Convert audio to WAV format if needed
	wavPath, cleanup, err := w.ensureWAV(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare audio: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Read audio samples
	samples, sampleRate, err := w.readAudioSamples(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Create context
	wctx, err := w.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("failed to create whisper context: %w", err)
	}

	// Set language
	if w.language != "" && w.language != "auto" {
		if err := wctx.SetLanguage(w.language); err != nil {
			return nil, fmt.Errorf("failed to set language: %w", err)
		}
	}

	// Process audio (4 callbacks: encoder begin, segment, progress, abort)
	if err := wctx.Process(samples, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("failed to process audio: %w", err)
	}

	// Collect segments
	var segments []Segment
	var fullText strings.Builder

	for {
		segment, err := wctx.NextSegment()
		if err != nil {
			break
		}

		segments = append(segments, Segment{
			Start: segment.Start,
			End:   segment.End,
			Text:  segment.Text,
		})

		fullText.WriteString(segment.Text)
		fullText.WriteString(" ")
	}

	// Calculate duration
	duration := time.Duration(float64(len(samples))/float64(sampleRate)) * time.Second

	// Detect language (whisper reports this)
	detectedLang := w.language
	if w.model.IsMultilingual() && w.language == "auto" {
		// Try to get detected language from context if available
		// For now, default to "auto"
		detectedLang = "auto"
	}

	return &Result{
		RawText:  strings.TrimSpace(fullText.String()),
		Segments: segments,
		Language: detectedLang,
		Duration: duration,
	}, nil
}

// Close releases the model resources.
func (w *WhisperTranscriber) Close() error {
	if w.model != nil {
		return w.model.Close()
	}
	return nil
}

// ensureWAV converts audio to WAV format if needed.
func (w *WhisperTranscriber) ensureWAV(filePath string) (string, func(), error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// If already WAV, use as-is
	if ext == ".wav" {
		return filePath, nil, nil
	}

	// Convert to WAV using ffmpeg
	tmpFile, err := os.CreateTemp("", "whisper-*.wav")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// ffmpeg command to convert to 16kHz mono WAV
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-y",
		tmpPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("ffmpeg conversion failed: %w\n%s", err, string(output))
	}

	cleanup := func() {
		os.Remove(tmpPath)
	}

	return tmpPath, cleanup, nil
}

// readAudioSamples reads a WAV file and returns float32 samples.
func (w *WhisperTranscriber) readAudioSamples(wavPath string) ([]float32, int, error) {
	file, err := os.Open(wavPath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, 0, fmt.Errorf("invalid WAV file")
	}

	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to decode WAV: %w", err)
	}

	// Convert to float32 (normalize 16-bit samples)
	const maxInt16 = 32768.0
	samples := make([]float32, len(buf.Data))
	for i, sample := range buf.Data {
		samples[i] = float32(sample) / maxInt16
	}

	sampleRate := int(decoder.SampleRate)
	return samples, sampleRate, nil
}

// SupportsLanguage returns true - Whisper supports 99+ languages.
func (w *WhisperTranscriber) SupportsLanguage(lang string) bool {
	return true
}
