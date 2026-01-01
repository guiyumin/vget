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

	"github.com/go-audio/wav"
	"github.com/guiyumin/vget/internal/core/config"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// SherpaTranscriber implements Transcriber using sherpa-onnx for Parakeet V3.
// For Whisper models, use WhisperTranscriber (whisper.cpp) instead.
type SherpaTranscriber struct {
	recognizer *sherpa.OfflineRecognizer
	modelPath  string
	language   string
}

// detectProvider returns "cuda" if NVIDIA GPU is available, otherwise "cpu".
func detectProvider() string {
	// Check NVIDIA_VISIBLE_DEVICES env var (set in Docker CUDA images)
	if nvDevices := os.Getenv("NVIDIA_VISIBLE_DEVICES"); nvDevices != "" && nvDevices != "void" {
		// Verify nvidia-smi works
		if err := exec.Command("nvidia-smi").Run(); err == nil {
			return "cuda"
		}
	}
	return "cpu"
}

// NewSherpaTranscriber creates a new sherpa-onnx transcriber for Parakeet V3.
func NewSherpaTranscriber(modelPath, language string) (*SherpaTranscriber, error) {
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("model directory not found: %s", modelPath)
	}

	config := sherpa.OfflineRecognizerConfig{}
	config.FeatConfig.SampleRate = 16000
	config.FeatConfig.FeatureDim = 80
	config.ModelConfig.NumThreads = 4
	config.ModelConfig.Provider = detectProvider()
	config.DecodingMethod = "greedy_search"

	// Parakeet V3 uses transducer model
	config.ModelConfig.Transducer.Encoder = filepath.Join(modelPath, "encoder.int8.onnx")
	config.ModelConfig.Transducer.Decoder = filepath.Join(modelPath, "decoder.int8.onnx")
	config.ModelConfig.Transducer.Joiner = filepath.Join(modelPath, "joiner.int8.onnx")
	config.ModelConfig.Tokens = filepath.Join(modelPath, "tokens.txt")

	recognizer := sherpa.NewOfflineRecognizer(&config)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create Parakeet recognizer")
	}

	return &SherpaTranscriber{
		recognizer: recognizer,
		modelPath:  modelPath,
		language:   language,
	}, nil
}

// NewSherpaTranscriberFromConfig creates a SherpaTranscriber from config.
func NewSherpaTranscriberFromConfig(cfg config.LocalASRConfig, modelsDir string) (*SherpaTranscriber, error) {
	model := cfg.Model
	if model == "" {
		model = "parakeet-v3"
	}

	// Use manager to get the correct model path
	manager := NewModelManager(modelsDir)
	modelPath := manager.ModelPath(model)

	language := cfg.Language
	if language == "" {
		language = "auto"
	}

	return NewSherpaTranscriber(modelPath, language)
}

// Name returns the provider name.
func (s *SherpaTranscriber) Name() string {
	return "parakeet"
}

// Transcribe converts an audio file to text using sherpa-onnx.
func (s *SherpaTranscriber) Transcribe(ctx context.Context, filePath string) (*Result, error) {
	// Convert audio to WAV format if needed
	wavPath, cleanup, err := s.ensureWAV(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare audio: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Read audio samples
	samples, sampleRate, err := s.readAudioSamples(wavPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Create stream and process
	stream := sherpa.NewOfflineStream(s.recognizer)
	if stream == nil {
		return nil, fmt.Errorf("failed to create offline stream")
	}
	defer sherpa.DeleteOfflineStream(stream)

	stream.AcceptWaveform(sampleRate, samples)
	s.recognizer.Decode(stream)

	result := stream.GetResult()
	if result == nil {
		return nil, fmt.Errorf("sherpa-onnx returned nil result")
	}

	// Build segments from timestamps if available
	var segments []Segment
	if len(result.Timestamps) > 0 && len(result.Tokens) > 0 {
		for i, token := range result.Tokens {
			if i < len(result.Timestamps) {
				start := time.Duration(result.Timestamps[i] * float32(time.Second))
				var end time.Duration
				if i < len(result.Timestamps)-1 {
					end = time.Duration(result.Timestamps[i+1] * float32(time.Second))
				} else {
					end = start + 500*time.Millisecond
				}
				segments = append(segments, Segment{
					Start: start,
					End:   end,
					Text:  token,
				})
			}
		}
	}

	// Calculate duration
	duration := time.Duration(float64(len(samples))/float64(sampleRate)) * time.Second

	// Get detected language
	detectedLang := result.Lang
	if detectedLang == "" {
		detectedLang = s.language
	}

	return &Result{
		RawText:  strings.TrimSpace(result.Text),
		Segments: segments,
		Language: detectedLang,
		Duration: duration,
	}, nil
}

// Close releases the recognizer resources.
func (s *SherpaTranscriber) Close() error {
	if s.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(s.recognizer)
		s.recognizer = nil
	}
	return nil
}

// ensureWAV converts audio to WAV format if needed.
func (s *SherpaTranscriber) ensureWAV(filePath string) (string, func(), error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// If already WAV, use as-is
	if ext == ".wav" {
		return filePath, nil, nil
	}

	// Convert to WAV using ffmpeg
	tmpFile, err := os.CreateTemp("", "sherpa-*.wav")
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
func (s *SherpaTranscriber) readAudioSamples(wavPath string) ([]float32, int, error) {
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

// SupportsLanguage checks if Parakeet V3 supports a language.
// Parakeet V3 supports 25 European languages.
func (s *SherpaTranscriber) SupportsLanguage(lang string) bool {
	parakeetLangs := map[string]bool{
		"bg": true, "hr": true, "cs": true, "da": true, "nl": true,
		"en": true, "et": true, "fi": true, "fr": true, "de": true,
		"el": true, "hu": true, "it": true, "lv": true, "lt": true,
		"mt": true, "pl": true, "pt": true, "ro": true, "sk": true,
		"sl": true, "es": true, "sv": true, "ru": true, "uk": true,
	}

	return parakeetLangs[lang]
}
