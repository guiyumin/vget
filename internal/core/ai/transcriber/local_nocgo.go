//go:build !cgo

package transcriber

import (
	"fmt"

	"github.com/guiyumin/vget/internal/core/config"
)

// NewLocal returns an error when CGO is disabled.
// Local transcription requires CGO for sherpa-onnx and whisper.cpp bindings.
func NewLocal(_ config.LocalASRConfig) (Transcriber, error) {
	return nil, fmt.Errorf("local transcription requires CGO (build with CGO_ENABLED=1)")
}
