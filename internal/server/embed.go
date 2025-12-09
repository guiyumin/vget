package server

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// GetDistFS returns the embedded dist filesystem
// Returns nil if dist directory doesn't exist (dev mode)
func GetDistFS() fs.FS {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return subFS
}
