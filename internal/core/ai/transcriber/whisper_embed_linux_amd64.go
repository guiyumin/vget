//go:build !cgo && linux && amd64

package transcriber

import "fmt"

// AI features are not available on Linux.
func extractWhisperBinary() (string, error) {
	return "", fmt.Errorf("AI features are not available on Linux")
}
