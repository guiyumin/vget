//go:build cgo

package transcriber

// TUIProgressReporter is a stub for CGO builds.
// The actual implementation is in progress.go (non-CGO builds).
type TUIProgressReporter struct {
	*ProgressReporter
}

// NewTUIProgressReporter creates a stub TUI progress reporter for CGO builds.
func NewTUIProgressReporter() *TUIProgressReporter {
	return &TUIProgressReporter{
		ProgressReporter: &ProgressReporter{},
	}
}

// RunTranscribeTUI is a stub for CGO builds - does nothing and returns nil.
func RunTranscribeTUI(filename, model string, reporter *TUIProgressReporter) error {
	// No TUI in CGO builds - progress is printed directly
	return nil
}
