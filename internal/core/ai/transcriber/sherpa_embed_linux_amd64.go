//go:build !cgo && linux && amd64

package transcriber

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed bin/sherpa-linux-amd64
var sherpaBinary []byte

func extractSherpaBinary() (string, error) {
	// Check for NVIDIA GPU first
	if !hasNvidiaGPU() {
		return "", fmt.Errorf("local transcription requires NVIDIA GPU with CUDA support. No NVIDIA GPU detected. Use cloud transcription (OpenAI) instead")
	}

	if len(sherpaBinary) == 0 {
		return "", fmt.Errorf("sherpa-onnx binary not embedded - build with GitHub Actions")
	}

	// Extract to ~/.config/vget/bin/
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(configDir, "vget", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}

	binaryPath := filepath.Join(binDir, "sherpa-linux-amd64")

	// Check if already extracted and same size
	if info, err := os.Stat(binaryPath); err == nil {
		if info.Size() == int64(len(sherpaBinary)) {
			return binaryPath, nil
		}
	}

	// Extract binary
	if err := os.WriteFile(binaryPath, sherpaBinary, 0755); err != nil {
		return "", err
	}

	return binaryPath, nil
}
