//go:build !cgo

package transcriber

import (
	"fmt"

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
// This works without CGO by using exec.Command to run embedded whisper.cpp binary
// (Metal on macOS, CUDA on Windows).
func NewLocal(cfg config.LocalASRConfig) (*LocalTranscriber, error) {
	modelsDir := cfg.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = DefaultModelsDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get models directory: %w", err)
		}
	}

	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	runner, err := NewWhisperRunnerFromConfig(cfg, modelsDir)
	if err != nil {
		return nil, err
	}
	return &LocalTranscriber{
		Transcriber:   runner,
		whisperRunner: runner,
		modelName:     model,
	}, nil
}
