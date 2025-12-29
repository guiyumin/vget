//go:build cgo

package transcriber

import (
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
)

// NewLocal creates a local transcriber using sherpa-onnx.
func NewLocal(cfg config.LocalASRConfig) (Transcriber, error) {
	modelsDir := cfg.ModelsDir
	if modelsDir == "" {
		var err error
		modelsDir, err = DefaultModelsDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get models directory: %w", err)
		}
	}
	return NewSherpaTranscriberFromConfig(cfg, modelsDir)
}
