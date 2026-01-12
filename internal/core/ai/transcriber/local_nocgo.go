//go:build !cgo

package transcriber

import (
	"fmt"
	"strings"

	"github.com/guiyumin/vget/internal/core/config"
)

// LocalTranscriber wraps a Transcriber with additional local-specific methods.
type LocalTranscriber struct {
	Transcriber
	whisperRunner *WhisperRunner
	modelName     string
}

// SetProgressReporter sets the progress reporter for TUI updates.
func (lt *LocalTranscriber) SetProgressReporter(reporter *ProgressReporter) {
	if lt.whisperRunner != nil {
		lt.whisperRunner.SetProgressReporter(reporter)
	}
}

// GetModelName returns the model name for display.
func (lt *LocalTranscriber) GetModelName() string {
	return lt.modelName
}

// NewLocal creates a local transcriber using embedded binaries.
// This works without CGO by using exec.Command to run embedded binaries:
// - whisper-* models → whisper.cpp (Metal on macOS, CUDA on Windows)
// - parakeet-* models → sherpa-onnx (CoreML on macOS, CUDA on Windows)
func NewLocal(cfg config.LocalASRConfig) (*LocalTranscriber, error) {
	modelsDir := cfg.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = DefaultModelsDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get models directory: %w", err)
		}
	}

	// Determine engine from model name
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	// Route to appropriate engine based on model name
	if strings.HasPrefix(model, "whisper") {
		runner, err := NewWhisperRunnerFromConfig(cfg, modelsDir)
		if err != nil {
			return nil, err
		}
		return &LocalTranscriber{
			Transcriber:   runner,
			whisperRunner: runner,
			modelName:     model,
		}, nil
	} else if strings.HasPrefix(model, "parakeet") {
		runner, err := NewSherpaRunnerFromConfig(cfg, modelsDir)
		if err != nil {
			return nil, err
		}
		return &LocalTranscriber{
			Transcriber: runner,
			modelName:   model,
		}, nil
	}

	return nil, fmt.Errorf("unsupported model: %q (supported: whisper-*, parakeet-*)", model)
}
