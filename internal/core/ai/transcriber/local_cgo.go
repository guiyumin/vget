//go:build cgo

package transcriber

import (
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
)

// LocalTranscriber wraps a Transcriber with additional local-specific methods.
type LocalTranscriber struct {
	Transcriber
	modelName string
}

// SetProgressReporter is a no-op for CGO builds (no TUI progress).
func (lt *LocalTranscriber) SetProgressReporter(reporter *ProgressReporter) {
	// No-op in CGO builds
}

// GetModelName returns the model name for display.
func (lt *LocalTranscriber) GetModelName() string {
	return lt.modelName
}

// NewLocal creates a local transcriber based on the configured model.
// Uses whisper.cpp for all models.
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

	t, err := NewWhisperTranscriberFromConfig(cfg, modelsDir)
	if err != nil {
		return nil, err
	}

	return &LocalTranscriber{
		Transcriber: t,
		modelName:   model,
	}, nil
}
