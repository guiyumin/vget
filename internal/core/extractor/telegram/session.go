package telegram

import (
	"os"
	"path/filepath"
)

// SessionPath returns the path where Telegram session is stored
// Uses ~/.config/vget/telegram/ to match vget's standard config directory
func SessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", "vget", "telegram")
}

// SessionFile returns the full path to the desktop session file
func SessionFile() string {
	return filepath.Join(SessionPath(), "desktop-session.json")
}

// SessionExists checks if a Telegram session exists
func SessionExists() bool {
	_, err := os.Stat(SessionFile())
	return err == nil
}
