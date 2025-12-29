package transcriber

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"github.com/go-audio/wav"
	"github.com/guiyumin/vget/internal/core/config"
)

// WhisperLocal implements Transcriber using whisper.cpp via CGO.
type WhisperLocal struct {
	model     whisper.Model
	modelPath string
	language  string // "auto" for auto-detection, or ISO code like "en", "zh"
}

// NewWhisperLocal creates a new local Whisper transcriber.
func NewWhisperLocal(modelPath string, language string) (*WhisperLocal, error) {
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("model file not found: %s", modelPath)
	}

	model, err := whisper.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load whisper model: %w", err)
	}

	if language == "" {
		language = "auto"
	}

	return &WhisperLocal{
		model:     model,
		modelPath: modelPath,
		language:  language,
	}, nil
}

// NewWhisperLocalFromConfig creates a WhisperLocal from config.
func NewWhisperLocalFromConfig(cfg config.AIServiceConfig, modelsDir string) (*WhisperLocal, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = "ggml-small.bin"
	}

	// If model name doesn't end with .bin, assume it's a short name
	if !strings.HasSuffix(modelName, ".bin") {
		modelName = "ggml-" + modelName + ".bin"
	}

	modelPath := filepath.Join(modelsDir, modelName)
	return NewWhisperLocal(modelPath, "auto")
}

// Name returns the provider name.
func (w *WhisperLocal) Name() string {
	return "whisper-local"
}

// Transcribe converts an audio file to text using whisper.cpp.
func (w *WhisperLocal) Transcribe(ctx context.Context, filePath string) (*Result, error) {
	// Convert audio to WAV format if needed
	wavPath, cleanup, err := w.ensureWAV(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare audio: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Read audio samples
	samples, err := w.readAudioSamples(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Create context for transcription
	whisperCtx, err := w.model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("failed to create whisper context: %w", err)
	}

	// Configure context
	if w.language != "auto" {
		if err := whisperCtx.SetLanguage(w.language); err != nil {
			return nil, fmt.Errorf("failed to set language: %w", err)
		}
	}

	// Process audio
	if err := whisperCtx.Process(samples, nil, nil, nil); err != nil {
		return nil, fmt.Errorf("transcription failed: %w", err)
	}

	// Collect segments
	var segments []Segment
	var textParts []string

	for {
		segment, err := whisperCtx.NextSegment()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get segment: %w", err)
		}

		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}

		segments = append(segments, Segment{
			Start: segment.Start,
			End:   segment.End,
			Text:  text,
		})
		textParts = append(textParts, text)
	}

	// Calculate duration from last segment
	var duration time.Duration
	if len(segments) > 0 {
		duration = segments[len(segments)-1].End
	}

	// Get detected language
	detectedLang := whisperCtx.Language()

	return &Result{
		RawText:  strings.Join(textParts, " "),
		Segments: segments,
		Language: detectedLang,
		Duration: duration,
	}, nil
}

// Close releases the model resources.
func (w *WhisperLocal) Close() error {
	if w.model != nil {
		return w.model.Close()
	}
	return nil
}

// ensureWAV converts audio to WAV format if needed.
// Returns the path to the WAV file and a cleanup function.
func (w *WhisperLocal) ensureWAV(filePath string) (string, func(), error) {
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
		"-ar", "16000", // 16kHz sample rate (required by Whisper)
		"-ac", "1", // mono
		"-c:a", "pcm_s16le", // 16-bit PCM
		"-y", // overwrite
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
func (w *WhisperLocal) readAudioSamples(wavPath string) ([]float32, error) {
	file, err := os.Open(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	// Read all samples
	buf, err := decoder.FullPCMBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to decode WAV: %w", err)
	}

	// Convert to float32 (normalize 16-bit samples)
	// We use 16-bit PCM as specified in ffmpeg conversion (-c:a pcm_s16le)
	const maxInt16 = 32768.0
	samples := make([]float32, len(buf.Data))
	for i, sample := range buf.Data {
		samples[i] = float32(sample) / maxInt16
	}

	return samples, nil
}

// IsMultilingual returns whether the loaded model supports multiple languages.
func (w *WhisperLocal) IsMultilingual() bool {
	return w.model.IsMultilingual()
}

// Languages returns the list of supported languages.
func (w *WhisperLocal) Languages() []string {
	return w.model.Languages()
}
