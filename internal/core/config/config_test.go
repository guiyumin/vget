package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "Absolute path",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "Relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			name:     "Home directory only",
			input:    "~",
			expected: home,
		},
		{
			name:     "Home directory with forward slash",
			input:    "~/Downloads",
			expected: filepath.Join(home, "Downloads"),
		},
		{
			name:     "Home directory with backslash (simulated)",
			input:    `~\Downloads`,
			expected: filepath.Join(home, "Downloads"),
		},
		{
			name:     "Invalid tilde use (middle)",
			input:    "/path/~/test",
			expected: "/path/~/test",
		},
		{
			name:     "Invalid tilde use (no separator)",
			input:    "~user",
			expected: "~user", // We don't support ~user expansion currently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.input)
			if got != tt.expected {
				t.Errorf("expandPath(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
