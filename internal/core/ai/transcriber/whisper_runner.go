//go:build !cgo

package transcriber

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"codeberg.org/gruf/go-ffmpreg/ffmpreg"
	"codeberg.org/gruf/go-ffmpreg/wasm"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/guiyumin/vget/internal/core/config"
	"github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"
	"github.com/tetratelabs/wazero"
)

// WhisperRunner transcribes audio using whisper.cpp CLI binary.
// This is used when CGO is disabled (CGO_ENABLED=0).
// The whisper.cpp binary is downloaded on first use from GitHub releases.
type WhisperRunner struct {
	binaryPath string
	modelPath  string
	language   string
}

// NewWhisperRunner creates a new whisper runner.
// Uses embedded GPU-enabled binary (Metal on macOS, CUDA on Windows).
// Falls back to download if binary not embedded.
func NewWhisperRunner(modelPath, language string) (*WhisperRunner, error) {
	// Validate model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found: %s", modelPath)
	}

	// Try embedded binary first (GPU-enabled: Metal/CUDA)
	binaryPath, err := extractWhisperBinary()
	if err != nil {
		// Fallback to download if not embedded
		fmt.Printf("  Embedded binary not available, downloading...\n")
		binDir, dirErr := DefaultBinDir()
		if dirErr != nil {
			return nil, fmt.Errorf("failed to get bin directory: %w", dirErr)
		}

		rtManager := NewRuntimeManager(binDir)
		binaryPath, err = rtManager.EnsureWhisper()
		if err != nil {
			return nil, fmt.Errorf("failed to get whisper binary: %w", err)
		}
	}

	return &WhisperRunner{
		binaryPath: binaryPath,
		modelPath:  modelPath,
		language:   language,
	}, nil
}

// NewWhisperRunnerFromConfig creates a WhisperRunner from config.
func NewWhisperRunnerFromConfig(cfg config.LocalASRConfig, modelsDir string) (*WhisperRunner, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = DefaultModel
	}

	// Look up model in registry to get the correct filename
	model := GetModel(modelName)
	var modelFile string
	if model != nil {
		modelFile = model.DirName
	} else {
		modelFile = modelName
		if !strings.HasSuffix(modelFile, ".bin") {
			modelFile = modelFile + ".bin"
		}
	}

	modelPath := filepath.Join(modelsDir, modelFile)

	language := cfg.Language
	if language == "" {
		language = "auto"
	}

	return NewWhisperRunner(modelPath, language)
}

// Name returns the provider name.
func (w *WhisperRunner) Name() string {
	return "whisper.cpp"
}

// Transcribe converts an audio file to text using whisper.cpp CLI.
func (w *WhisperRunner) Transcribe(ctx context.Context, filePath string) (*Result, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Convert audio to WAV if needed
	wavPath, cleanup, err := w.ensureWAV(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare audio: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	// Create temp file for output
	tmpDir, err := os.MkdirTemp("", "whisper-output-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outputBase := filepath.Join(tmpDir, "output")

	// Build command
	args := []string{
		"-m", w.modelPath,
		"-f", wavPath,
		"-otxt",
		"-of", outputBase,
		"-pp", // print progress
	}

	if w.language != "" && w.language != "auto" {
		args = append(args, "-l", w.language)
	}

	// Use available CPU threads
	numThreads := runtime.NumCPU()
	if numThreads > 8 {
		numThreads = 8
	}
	args = append(args, "-t", fmt.Sprintf("%d", numThreads))

	fmt.Printf("  Running whisper.cpp...\n")
	fmt.Printf("  Model: %s\n", filepath.Base(w.modelPath))
	fmt.Printf("  Threads: %d\n", numThreads)

	// Run whisper.cpp
	cmd := exec.CommandContext(ctx, w.binaryPath, args...)

	// Capture stderr for progress
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start whisper: %w", err)
	}

	// Read and print progress
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "progress") || strings.Contains(line, "%") {
				fmt.Printf("  %s\n", line)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("whisper failed: %w", err)
	}

	// Read output file
	outputPath := outputBase + ".txt"
	content, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	text := strings.TrimSpace(string(content))

	// Parse segments from output (whisper.cpp txt format has timestamps)
	segments := parseWhisperOutput(text)

	// Get audio duration
	duration, _ := getAudioDuration(wavPath)

	return &Result{
		RawText:  cleanTranscriptText(text),
		Segments: segments,
		Language: w.language,
		Duration: duration,
	}, nil
}

// Close is a no-op for the runner.
func (w *WhisperRunner) Close() error {
	return nil
}

// SupportsLanguage returns true - Whisper supports 99+ languages.
func (w *WhisperRunner) SupportsLanguage(lang string) bool {
	return true
}

// MaxFileSize returns 0 - local whisper has no file size limit.
func (w *WhisperRunner) MaxFileSize() int64 {
	return 0
}

// ensureWAV converts audio to 16kHz mono WAV if needed.
func (w *WhisperRunner) ensureWAV(filePath string) (string, func(), error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// If already WAV, check sample rate
	if ext == ".wav" {
		return filePath, nil, nil
	}

	// Convert to WAV
	tmpFile, err := os.CreateTemp("", "whisper-*.wav")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	cleanup := func() {
		os.Remove(tmpPath)
	}

	// Try pure Go decoders first
	var samples []float32
	var sampleRate int

	switch ext {
	case ".mp3":
		samples, sampleRate, err = readMP3Samples(filePath)
	case ".flac":
		samples, sampleRate, err = readFLACSamples(filePath)
	default:
		// Use embedded ffmpeg WASM for other formats
		err = convertWithFFmpeg(filePath, tmpPath)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		return tmpPath, cleanup, nil
	}

	if err != nil {
		cleanup()
		return "", nil, err
	}

	// Resample to 16kHz if needed
	if sampleRate != 16000 {
		samples = resampleTo16kHz(samples, sampleRate)
	}

	// Write WAV file
	if err := writeWAV(tmpPath, samples, 16000); err != nil {
		cleanup()
		return "", nil, err
	}

	return tmpPath, cleanup, nil
}

// readMP3Samples reads MP3 and returns float32 samples.
func readMP3Samples(filePath string) ([]float32, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	decoder, err := mp3.NewDecoder(file)
	if err != nil {
		return nil, 0, err
	}

	sampleRate := decoder.SampleRate()
	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, 0, err
	}

	// Convert stereo 16-bit PCM to mono float32
	numSamples := len(data) / 4
	samples := make([]float32, numSamples)

	const maxInt16 = 32768.0
	for i := 0; i < numSamples; i++ {
		left := int16(data[i*4]) | int16(data[i*4+1])<<8
		right := int16(data[i*4+2]) | int16(data[i*4+3])<<8
		mono := (int32(left) + int32(right)) / 2
		samples[i] = float32(mono) / maxInt16
	}

	return samples, sampleRate, nil
}

// readFLACSamples reads FLAC and returns float32 samples.
func readFLACSamples(filePath string) ([]float32, int, error) {
	stream, err := flac.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer stream.Close()

	sampleRate := int(stream.Info.SampleRate)
	nChannels := int(stream.Info.NChannels)
	bitsPerSample := int(stream.Info.BitsPerSample)
	maxVal := float32(int64(1) << (bitsPerSample - 1))

	var samples []float32
	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}

		nSamples := len(frame.Subframes[0].Samples)
		for i := 0; i < nSamples; i++ {
			var mono int64
			for ch := 0; ch < nChannels; ch++ {
				mono += int64(frame.Subframes[ch].Samples[i])
			}
			mono /= int64(nChannels)
			samples = append(samples, float32(mono)/maxVal)
		}
	}

	return samples, sampleRate, nil
}

// convertWithFFmpeg uses embedded ffmpeg WASM to convert audio.
func convertWithFFmpeg(inputPath, outputPath string) error {
	fmt.Printf("  Converting audio using embedded ffmpeg...\n")

	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return err
	}
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}

	inputDir := filepath.Dir(absInput)
	outputDir := filepath.Dir(absOutput)

	ctx := context.Background()
	args := wasm.Args{
		Stderr: io.Discard,
		Stdout: io.Discard,
		Args: []string{
			"-i", absInput,
			"-ar", "16000",
			"-ac", "1",
			"-c:a", "pcm_s16le",
			"-y",
			absOutput,
		},
		Config: func(cfg wazero.ModuleConfig) wazero.ModuleConfig {
			return cfg.WithFSConfig(wazero.NewFSConfig().
				WithDirMount(inputDir, inputDir).
				WithDirMount(outputDir, outputDir))
		},
	}

	rc, err := ffmpreg.Ffmpeg(ctx, args)
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}
	if rc != 0 {
		return fmt.Errorf("ffmpeg exited with code %d", rc)
	}

	return nil
}

// writeWAV writes float32 samples to a WAV file.
func writeWAV(path string, samples []float32, sampleRate int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := wav.NewEncoder(file, sampleRate, 16, 1, 1)
	defer encoder.Close()

	// Convert float32 to int
	intBuf := &audio.IntBuffer{
		Data:           make([]int, len(samples)),
		Format:         &audio.Format{SampleRate: sampleRate, NumChannels: 1},
		SourceBitDepth: 16,
	}

	for i, s := range samples {
		// Clamp and convert
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		intBuf.Data[i] = int(s * 32767)
	}

	return encoder.Write(intBuf)
}

// resampleTo16kHz resamples audio using linear interpolation.
func resampleTo16kHz(samples []float32, srcRate int) []float32 {
	if srcRate == 16000 {
		return samples
	}

	ratio := float64(srcRate) / 16000.0
	newLen := int(float64(len(samples)) / ratio)
	resampled := make([]float32, newLen)

	for i := 0; i < newLen; i++ {
		srcPos := float64(i) * ratio
		srcIdx := int(srcPos)
		frac := float32(srcPos - float64(srcIdx))

		if srcIdx+1 < len(samples) {
			resampled[i] = samples[srcIdx]*(1-frac) + samples[srcIdx+1]*frac
		} else if srcIdx < len(samples) {
			resampled[i] = samples[srcIdx]
		}
	}

	return resampled
}

// parseWhisperOutput parses whisper.cpp text output into segments.
func parseWhisperOutput(text string) []Segment {
	// whisper.cpp txt format: [00:00:00.000 --> 00:00:05.000] Text here
	var segments []Segment
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse timestamp format
		if strings.HasPrefix(line, "[") {
			endBracket := strings.Index(line, "]")
			if endBracket > 0 {
				timestamp := line[1:endBracket]
				parts := strings.Split(timestamp, " --> ")
				if len(parts) == 2 {
					start := parseTimestamp(parts[0])
					end := parseTimestamp(parts[1])
					text := strings.TrimSpace(line[endBracket+1:])
					if text != "" {
						segments = append(segments, Segment{
							Start: start,
							End:   end,
							Text:  text,
						})
					}
					continue
				}
			}
		}

		// Plain text without timestamps
		if len(segments) == 0 {
			segments = append(segments, Segment{Text: line})
		} else {
			segments[len(segments)-1].Text += " " + line
		}
	}

	return segments
}

// parseTimestamp parses HH:MM:SS.mmm format.
func parseTimestamp(s string) time.Duration {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}

	var hours, minutes int
	var seconds float64
	fmt.Sscanf(parts[0], "%d", &hours)
	fmt.Sscanf(parts[1], "%d", &minutes)
	fmt.Sscanf(parts[2], "%f", &seconds)

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds*float64(time.Second))
}

// cleanTranscriptText removes timestamp markers from text.
func cleanTranscriptText(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove timestamp prefix if present
		if strings.HasPrefix(line, "[") {
			endBracket := strings.Index(line, "]")
			if endBracket > 0 {
				line = strings.TrimSpace(line[endBracket+1:])
			}
		}

		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, " ")
}

// getAudioDuration gets the duration of a WAV file.
func getAudioDuration(wavPath string) (time.Duration, error) {
	file, err := os.Open(wavPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	decoder := wav.NewDecoder(file)
	if !decoder.IsValidFile() {
		return 0, fmt.Errorf("invalid WAV file")
	}

	dur, err := decoder.Duration()
	if err != nil {
		return 0, err
	}

	return dur, nil
}
