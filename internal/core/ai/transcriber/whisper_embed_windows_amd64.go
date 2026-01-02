//go:build !cgo && windows && amd64

package transcriber

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed bin/whisper-windows-amd64.exe
var whisperBinary []byte

func extractWhisperBinary() (string, error) {
	// Check for NVIDIA GPU first
	if !hasNvidiaGPU() {
		return "", fmt.Errorf("AI features require NVIDIA GPU with CUDA support. No NVIDIA GPU detected")
	}

	if len(whisperBinary) == 0 {
		return "", fmt.Errorf("whisper binary not embedded - build with GitHub Actions")
	}

	// Extract to ~/AppData/Local/vget/bin/
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	binDir := filepath.Join(configDir, "vget", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", err
	}

	binaryPath := filepath.Join(binDir, "whisper-windows-amd64.exe")

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

// hasNvidiaGPU checks if an NVIDIA GPU is available by running nvidia-smi.
func hasNvidiaGPU() bool {
	cmd := exec.Command("nvidia-smi")
	err := cmd.Run()
	return err == nil
}
