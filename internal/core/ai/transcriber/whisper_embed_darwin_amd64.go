//go:build !cgo && darwin && amd64

package transcriber

import "fmt"

// AI features are not available on Intel Macs.
// GPU acceleration (Metal) is only available on Apple Silicon (M1/M2/M3/M4).
func extractWhisperBinary() (string, error) {
	return "", fmt.Errorf("AI features are not available on Intel Macs. Please use a Mac with Apple Silicon (M1/M2/M3/M4)")
}
