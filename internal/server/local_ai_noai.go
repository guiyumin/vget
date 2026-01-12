//go:build noai

package server

// IsLocalAISupported returns false when built without AI support
func IsLocalAISupported() bool {
	return false
}
