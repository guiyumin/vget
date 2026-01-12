//go:build !noai

package server

// IsLocalAISupported returns true when built with AI support
func IsLocalAISupported() bool {
	return true
}
