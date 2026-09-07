package cli

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/downloader"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/tailcat"
	"github.com/guiyumin/vget/internal/core/webdav"
)

// runWebDAVDownload handles "vget remote:/path" and webdav:// URLs.
func runWebDAVDownload(rawURL, lang string) error {
	ctx := context.Background()
	cfg := config.LoadOrDefault()

	var client *webdav.Client
	var filePath string
	var serverName string
	var err error

	// Check if it's a remote path (e.g., "pikpak:/path/to/file")
	if webdav.IsRemotePath(rawURL) {
		serverName, filePath, err = webdav.ParseRemotePath(rawURL)
		if err != nil {
			return err
		}

		server := cfg.GetWebDAVServer(serverName)
		if server == nil {
			return fmt.Errorf("WebDAV server '%s' not found. Add it with 'vget config webdav add %s'", serverName, serverName)
		}

		client, err = webdav.NewClientFromConfig(server)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}
	} else {
		// Create WebDAV client from URL
		client, err = webdav.NewClient(rawURL)
		if err != nil {
			return fmt.Errorf("failed to create WebDAV client: %w", err)
		}

		// Parse the file path from URL
		filePath, err = webdav.ParseURL(rawURL)
		if err != nil {
			return fmt.Errorf("invalid WebDAV URL: %w", err)
		}
		serverName = "webdav" // Default name for direct URLs
	}
	defer closeTunnels()

	// Get file info
	fileInfo, err := client.Stat(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// If it's a directory, open the TUI browser
	if fileInfo.IsDir {
		result, err := RunBrowseTUI(client, serverName, filePath)
		if err != nil {
			return fmt.Errorf("browse failed: %w", err)
		}
		if result.Cancelled {
			return nil // User cancelled, no error
		}
		// User selected a file, update filePath and continue with download
		filePath = result.SelectedFile
		// Re-fetch file info for the selected file
		fileInfo, err = client.Stat(ctx, filePath)
		if err != nil {
			return fmt.Errorf("failed to get file info: %w", err)
		}
	}

	// Determine output filename
	outputFile := output
	if outputFile == "" {
		outputFile = webdav.ExtractFilename(filePath)
		// Prepend outputDir if configured
		if cfg.OutputDir != "" {
			outputFile = filepath.Join(cfg.OutputDir, outputFile)
		}
	}

	return downloadRemoteFile(client, filePath, fileInfo, outputFile, lang)
}

// downloadRemoteFile fetches one remote file with the multi-stream engine,
// through the client's tunnel when it has one.
func downloadRemoteFile(client *webdav.Client, filePath string, fileInfo *webdav.FileInfo, outputFile, lang string) error {
	fmt.Printf("  WebDAV: %s (%s)\n", fileInfo.Name, formatSize(fileInfo.Size))

	if dir := filepath.Dir(outputFile); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	fileURL := client.GetFileURL(filePath)
	authHeader := client.GetAuthHeader()
	msConfig := downloader.DefaultMultiStreamConfig()
	msConfig.DialContext = client.DialContext()

	return downloader.RunMultiStreamDownloadWithAuthTUI(
		fileURL,
		authHeader,
		outputFile,
		fileInfo.Name,
		lang,
		fileInfo.Size,
		msConfig,
	)
}

// runTailcatDownload handles "vget <tc-address>": fetch what a one-time
// "vget send" offers. A single file is downloaded right away; several are
// picked in a list, or all taken with --all.
func runTailcatDownload(addr string, cfg *config.Config) error {
	t := i18n.T(cfg.Language)
	ctx := context.Background()

	client, err := webdav.NewTailcatClient(addr, remoteUser, remotePass)
	if err != nil {
		return err
	}
	defer closeTunnels()

	entries, err := client.List(ctx, "/")
	if err != nil {
		return fmt.Errorf("cannot reach the other side: %w (is 'vget send' still running there?)", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("the other side offers no files")
	}

	var picked []webdav.FileInfo
	switch {
	case downloadAll || (len(entries) == 1 && !entries[0].IsDir):
		picked = entries
	default:
		picked, err = RunSelectTUI(tailcat.Mask(addr), entries, cfg.Language)
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			fmt.Println("  " + t.Tailcat.NothingSelected)
			return nil
		}
	}

	// Expand directories into their files, keeping relative paths.
	type job struct {
		info *webdav.FileInfo
		rel  string
	}
	var jobs []job
	var walk func(dir string, rel string) error
	walk = func(dir string, rel string) error {
		items, err := client.List(ctx, dir)
		if err != nil {
			return err
		}
		for i := range items {
			it := items[i]
			p := path.Join(dir, it.Name)
			r := filepath.Join(rel, it.Name)
			if it.IsDir {
				if err := walk(p, r); err != nil {
					return err
				}
				continue
			}
			jobs = append(jobs, job{info: &webdav.FileInfo{Name: it.Name, Path: p, Size: it.Size}, rel: r})
		}
		return nil
	}
	for i := range picked {
		it := picked[i]
		p := path.Join("/", it.Name)
		if it.IsDir {
			if err := walk(p, it.Name); err != nil {
				return err
			}
			continue
		}
		jobs = append(jobs, job{info: &webdav.FileInfo{Name: it.Name, Path: p, Size: it.Size}, rel: it.Name})
	}
	if len(jobs) == 0 {
		return fmt.Errorf("nothing to download")
	}
	if len(jobs) > 1 {
		fmt.Printf("  %s\n", fmt.Sprintf(t.Tailcat.DownloadingN, len(jobs), tailcat.Mask(addr)))
	}

	for i, j := range jobs {
		outputFile := filepath.Join(cfg.OutputDir, j.rel)
		if output != "" && len(jobs) == 1 {
			outputFile = output
		}
		if len(jobs) > 1 {
			fmt.Printf("\n  [%d/%d] %s\n", i+1, len(jobs), j.rel)
		}
		if err := downloadRemoteFile(client, j.info.Path, j.info, outputFile, cfg.Language); err != nil {
			return err
		}
	}
	return nil
}
