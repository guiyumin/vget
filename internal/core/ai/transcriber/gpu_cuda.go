//go:build !cgo && ((linux && amd64) || (windows && amd64))

package transcriber

import "os/exec"

// hasNvidiaGPU checks if an NVIDIA GPU is available by running nvidia-smi.
func hasNvidiaGPU() bool {
	cmd := exec.Command("nvidia-smi")
	err := cmd.Run()
	return err == nil
}
