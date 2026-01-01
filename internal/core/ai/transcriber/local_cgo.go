//go:build cgo

package transcriber

import (
	"fmt"
	"strings"

	"github.com/guiyumin/vget/internal/core/config"
)

// NewLocal creates a local transcriber based on the configured model.
// Uses whisper.cpp for whisper-* models, sherpa-onnx for parakeet-* models.
func NewLocal(cfg config.LocalASRConfig) (Transcriber, error) {
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
	if engine == "" {
		// Infer from model name
		model := cfg.Model
		if model == "" {
			model = DefaultModel
		}
		if strings.HasPrefix(model, "whisper") {
			engine = "whisper"
		} else {
			engine = "parakeet"
		}
	}

	// Create appropriate transcriber
	switch engine {
	case "whisper":
		return NewWhisperTranscriberFromConfig(cfg, modelsDir)
	default:
		return NewSherpaTranscriberFromConfig(cfg, modelsDir)
	}
}
