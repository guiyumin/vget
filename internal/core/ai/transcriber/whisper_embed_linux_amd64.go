//go:build !cgo && linux && amd64

package transcriber

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed bin/whisper-linux-amd64
var whisperBinary []byte

func extractWhisperBinary() (string, error) {
	// Check for NVIDIA GPU first
	if !hasNvidiaGPU() {
		return "", fmt.Errorf("local transcription requires NVIDIA GPU with CUDA support. No NVIDIA GPU detected. Use cloud transcription (OpenAI) instead")
	}

	if len(whisperBinary) == 0 {
		return "", fmt.Errorf("whisper binary not embedded - build with GitHub Actions")
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

	binaryPath := filepath.Join(binDir, "whisper-linux-amd64")

	// Check if already extracted and same size
	if info, err := os.Stat(binaryPath); err == nil {
		if info.Size() == int64(len(whisperBinary)) {
			return binaryPath, nil
		}
	}

	// Extract binary
	if err := os.WriteFile(binaryPath, whisperBinary, 0755); err != nil {
		return "", err
	}

	return binaryPath, nil
}
