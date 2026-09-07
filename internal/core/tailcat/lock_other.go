//go:build !unix

package tailcat

// lockFile is a no-op where flock is unavailable; a second server with the
// same identity is then only reported by the relay dropping one of them.
func lockFile(path string) (release func(), err error) {
	return func() {}, nil
}
