//go:build cgo

package transcriber

import (
	"fmt"
	"strings"

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
// Uses whisper.cpp for whisper-* models, sherpa-onnx for parakeet-* models.
func NewLocal(cfg config.LocalASRConfig) (*LocalTranscriber, error) {
	modelsDir := cfg.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = DefaultModelsDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get models directory: %w", err)
		}
	}

	// Determine engine from model name or explicit engine config
	engine := cfg.Engine
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	fmt.Printf("=== LOCAL ASR CONFIG ===\n")
	fmt.Printf("  Configured Model: %q\n", cfg.Model)
	fmt.Printf("  Using Model: %q\n", model)
	fmt.Printf("  Configured Engine: %q\n", cfg.Engine)

	if engine == "" {
		// Infer from model name
		if strings.HasPrefix(model, "whisper") {
			engine = "whisper"
		} else {
			engine = "parakeet"
		}
	}

	fmt.Printf("  Using Engine: %q\n", engine)
	fmt.Printf("  Models Dir: %s\n", modelsDir)
	fmt.Printf("========================\n")

	// Create appropriate transcriber
	var t Transcriber
	var err error
	switch engine {
	case "whisper":
		t, err = NewWhisperTranscriberFromConfig(cfg, modelsDir)
	default:
		t, err = NewSherpaTranscriberFromConfig(cfg, modelsDir)
	}

	if err != nil {
		return nil, err
	}

	return &LocalTranscriber{
		Transcriber: t,
		modelName:   model,
	}, nil
}
