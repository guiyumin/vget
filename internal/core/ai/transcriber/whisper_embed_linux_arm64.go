//go:build !cgo && linux && arm64

package transcriber

import "fmt"

// AI features require GPU acceleration (CUDA or Metal).
// Linux ARM64 has no GPU support.
func extractWhisperBinary() (string, error) {
	return "", fmt.Errorf("AI features are not available on Linux ARM64 (no GPU acceleration)")
}
