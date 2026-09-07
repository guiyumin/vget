package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/downloader"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tailcat"
	"github.com/guiyumin/vget/internal/core/webdav"
)

var (
	pushUser string
	pushPass string
)

var pushCmd = &cobra.Command{
	Use:   "push <file>... <tc-address | remote:/dir/>",
	Short: "Push files to a one-time inbox or into a library",
	Long: `Upload local files to the other side.

The target is either a one-time address from "vget recv", or a configured
library path. A library only accepts uploads if it is served with --rw or
into its inbox (--inbox, exposed at /inbox).

Examples:
  vget push reply.zip <tc-address>
  vget push movie.mkv nas:/inbox/
  vget push a.pdf b.pdf nas:/papers/        # directories are created as needed
  vget push report.pdf nas:/docs/final.pdf  # a single file can be renamed`,
	Args: cobra.MinimumNArgs(2),
	RunE: runPush,
}

func init() {
	pushCmd.Flags().StringVar(&pushUser, "user", "", "account name (for a tc-address that requires one)")
	pushCmd.Flags().StringVar(&pushPass, "pass", "", "account password")
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	target := args[len(args)-1]
	files := args[:len(args)-1]
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)
	ctx := context.Background()

	var (
		client  *webdav.Client
		dir     string // remote directory receiving the files
		rename  string // explicit remote file name (single file only)
		display string // how the destination is shown
		err     error
	)
	switch {
	case tailcat.IsAddr(target):
		client, err = webdav.NewTailcatClient(target, pushUser, pushPass)
		if err != nil {
			return err
		}
		dir = "/"
		display = tailcat.Mask(target) + ":"
	case webdav.IsRemotePath(target):
		name, p, perr := webdav.ParseRemotePath(target)
		if perr != nil {
			return perr
		}
		server := cfg.GetWebDAVServer(name)
		if server == nil {
			return fmt.Errorf("WebDAV server '%s' not found. Add it with 'vget config webdav add %s'", name, name)
		}
		client, err = webdav.NewClientFromConfig(server)
		if err != nil {
			return err
		}
		switch {
		case strings.HasSuffix(target, "/") || p == "/":
			dir = p
		case len(files) == 1:
			dir, rename = path.Dir(p), path.Base(p)
		default:
			dir = p
		}
		display = name + ":"
	default:
		if strings.HasPrefix(target, "tc") {
			return tailcat.Validate(target)
		}
		return fmt.Errorf("target must be a tailcat address or remote:/dir/ (got %q)", target)
	}
	defer closeTunnels()

	// Reach the other side once before pushing, so connectivity and
	// credential problems show up as one clear error.
	if _, err := client.Stat(ctx, "/"); err != nil {
		return fmt.Errorf("cannot reach %s: %w", display, err)
	}
	if dir != "/" {
		if err := client.MkdirAll(ctx, dir); err != nil {
			if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "405") {
				return errors.New(t.Tailcat.PushForbidden)
			}
			return err
		}
	}

	for _, local := range files {
		info, err := os.Stat(local)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory; push files one by one", local)
		}
		name := filepath.Base(local)
		if rename != "" {
			name = rename
		}
		remotePath := path.Join(dir, name)
		if err := uploadFile(client, local, remotePath, info.Size(), display, cfg.Language, t); err != nil {
			return err
		}
	}
	return nil
}

// uploadFile PUTs local to remotePath with a progress view.
func uploadFile(client *webdav.Client, local, remotePath string, size int64, display, lang string, t *i18n.Translations) error {
	return downloader.RunUploadTUI(filepath.Base(local), display+remotePath, lang, size, func(ctx context.Context, progress func(int64)) error {
		f, err := os.Open(local)
		if err != nil {
			return err
		}
		defer f.Close()

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, client.GetFileURL(remotePath), &progressReader{r: f, fn: progress})
		if err != nil {
			return err
		}
		req.ContentLength = size
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("User-Agent", downloader.DefaultUserAgent)
		if auth := client.GetAuthHeader(); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := client.HTTPClient().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusNoContent:
			return nil
		case http.StatusUnauthorized:
			return errors.New("unauthorized (401): check the user name and password")
		case http.StatusForbidden, http.StatusMethodNotAllowed:
			return errors.New(t.Tailcat.PushForbidden)
		case http.StatusConflict:
			return fmt.Errorf("the remote directory %s does not exist", path.Dir(remotePath))
		default:
			return fmt.Errorf("upload failed: %s", resp.Status)
		}
	})
}

// progressReader reports bytes read to fn.
type progressReader struct {
	r  io.Reader
	fn func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 && p.fn != nil {
		p.fn(int64(n))
	}
	return n, err
}

// closeTunnels shuts down any tailcat tunnels this process opened, giving
// their last packets a moment to reach the other side.
func closeTunnels() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tailcat.Shutdown(ctx)
}
