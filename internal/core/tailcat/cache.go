package tailcat

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CacheDir returns vget's cache directory: ~/.cache/vget, or
// %LOCALAPPDATA%\vget\cache on Windows.
func CacheDir() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "vget", "cache"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "vget"), nil
}

// diskDERPMapCache stores the public DERP map in ~/.cache/vget/derpmap.json,
// implementing tailcat.DERPMapCache. The file's mtime is the stored-at time;
// the server's ETag lives in a sibling .etag file. Only the public map is
// ever fetched (a self-hosted relay is embedded in the identity and address),
// so the cache ignores the URL key.
type diskDERPMapCache struct{}

func (diskDERPMapCache) paths() (dataPath, etagPath string, err error) {
	dir, err := CacheDir()
	if err != nil {
		return "", "", err
	}
	base := filepath.Join(dir, "derpmap")
	return base + ".json", base + ".etag", nil
}

func (c diskDERPMapCache) Get(url string) (data []byte, etag string, storedAt time.Time, ok bool) {
	dataPath, etagPath, err := c.paths()
	if err != nil {
		return
	}
	fi, err := os.Stat(dataPath)
	if err != nil {
		return
	}
	data, err = os.ReadFile(dataPath)
	if err != nil {
		return
	}
	if v, err := os.ReadFile(etagPath); err == nil {
		etag = strings.TrimSpace(string(v))
	}
	return data, etag, fi.ModTime(), true
}

func (c diskDERPMapCache) Put(url string, data []byte, etag string) error {
	dataPath, etagPath, err := c.paths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o700); err != nil {
		return err
	}
	// Rewriting identical contents still bumps the mtime, restarting the
	// freshness window after a 304 revalidation.
	if err := os.WriteFile(dataPath, data, 0o644); err != nil {
		return err
	}
	if etag == "" {
		os.Remove(etagPath)
		return nil
	}
	return os.WriteFile(etagPath, []byte(etag), 0o644)
}
