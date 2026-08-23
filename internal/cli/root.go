package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/guiyumin/vget/internal/core/config"
	"github.com/guiyumin/vget/internal/core/downloader"
	"github.com/guiyumin/vget/internal/core/extractor"
	"github.com/guiyumin/vget/internal/core/i18n"
	"github.com/guiyumin/vget/internal/core/version"
	"github.com/guiyumin/vget/internal/core/webdav"
	"github.com/spf13/cobra"
)

var (
	output    string
	quality   string
	info      bool
	inputFile string
)

var rootCmd = &cobra.Command{
	Use:     "vget [url]",
	Short:   "Versatile command-line toolkit for downloading audio, video, podcasts, and more",
	Version: version.Version,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Batch mode: read URLs from file
		if inputFile != "" {
			if err := runBatch(inputFile); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if len(args) == 0 {
			cmd.Help()
			return
		}
		if err := runDownload(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "output filename")
	rootCmd.Flags().StringVarP(&quality, "quality", "q", "", "preferred quality (e.g., 1080p, 720p)")
	rootCmd.Flags().BoolVar(&info, "info", false, "show video info without downloading")
	rootCmd.Flags().StringVarP(&inputFile, "file", "f", "", "read URLs from file (one per line)")
}

func Execute() error {
	return rootCmd.Execute()
}

func runDownload(url string) error {
	cfg := config.LoadOrDefault()
	t := i18n.T(cfg.Language)

	// Check for config file and warn if missing
	if !config.Exists() {
		fmt.Fprintf(os.Stderr, "\033[33m%s. Run 'vget init'.\033[0m\n", t.Errors.ConfigNotFound)
	}

	// Handle WebDAV URLs specially
	if webdav.IsWebDAVURL(url) {
		return runWebDAVDownload(url, cfg.Language)
	}

	// Find matching extractor
	ext := extractor.Match(url)
	if ext == nil {
		return fmt.Errorf("%s: %s", t.Errors.NoExtractor, url)
	}

	// Configure Twitter extractor with auth if available
	if twitterExt, ok := ext.(*extractor.TwitterExtractor); ok {
		if cfg.Twitter.AuthToken != "" {
			twitterExt.SetAuth(cfg.Twitter.AuthToken)
		}
	}

	// Extract media info with spinner
	media, err := runExtractWithSpinner(ext, url, cfg.Language)
	if err != nil {
		// Handle Twitter-specific errors with translated messages
		var twitterErr *extractor.TwitterError
		if errors.As(err, &twitterErr) {
			var msg string
			switch twitterErr.Code {
			case extractor.TwitterErrorNSFW:
				msg = t.Twitter.NsfwLoginRequired
			case extractor.TwitterErrorProtected:
				msg = t.Twitter.ProtectedTweet
			case extractor.TwitterErrorUnavailable:
				msg = t.Twitter.TweetUnavailable
			default:
				msg = twitterErr.Message
			}
			// Show auth hint if not authenticated
			if cfg.Twitter.AuthToken == "" {
				return fmt.Errorf("%s\n%s", msg, t.Twitter.AuthHint)
			}
			return fmt.Errorf("%s", msg)
		}
		return err
	}

	dl := downloader.New(cfg.Language)

	// Handle based on media type
	switch m := media.(type) {
	case *extractor.VideoMedia:
		return downloadVideo(m, dl, t, cfg.Language, cfg.OutputDir)
	case *extractor.AudioMedia:
		return downloadAudio(m, dl, cfg.OutputDir)
	case *extractor.ImageMedia:
		return downloadImages(m, dl, cfg.OutputDir)
	case *extractor.MultiVideoMedia:
		return downloadMultiVideo(m, dl, t, cfg.Language, cfg.OutputDir)
	default:
		return fmt.Errorf("unsupported media type")
	}
}

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

	fmt.Printf("  WebDAV: %s (%s)\n", fileInfo.Name, formatSize(fileInfo.Size))

	// Use multi-stream download for better performance
	fileURL := client.GetFileURL(filePath)
	authHeader := client.GetAuthHeader()
	msConfig := downloader.DefaultMultiStreamConfig()

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

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func downloadMultiVideo(m *extractor.MultiVideoMedia, dl *downloader.Downloader, t *i18n.Translations, lang string, outputDir string) error {
	// Info only mode
	if info {
		fmt.Printf("  Videos (%d):\n", len(m.Videos))
		for i, video := range m.Videos {
			fmt.Printf("    [%d] %s\n", i+1, video.Title)
			for j, f := range video.Formats {
				audioInfo := ""
				if f.AudioURL != "" {
					audioInfo = " [+audio]"
				}
				fmt.Printf("        [%d.%d] %s %dx%d (%s)%s\n", i+1, j, f.Quality, f.Width, f.Height, f.Ext, audioInfo)
			}
		}
		return nil
	}

	fmt.Printf("  Downloading %d video(s)...\n", len(m.Videos))

	for i, video := range m.Videos {
		fmt.Printf("\n  [%d/%d] %s\n", i+1, len(m.Videos), video.Title)
		// Pass index for multi-video to avoid filename collisions
		if err := downloadVideoWithIndex(video, dl, t, lang, outputDir, i+1, len(m.Videos)); err != nil {
			return fmt.Errorf("failed to download video %d: %w", i+1, err)
		}
	}
	return nil
}

func downloadVideo(m *extractor.VideoMedia, dl *downloader.Downloader, t *i18n.Translations, lang string, outputDir string) error {
	// Info only mode
	if info {
		for i, f := range m.Formats {
			audioInfo := ""
			if f.AudioURL != "" {
				audioInfo = " [+audio]"
			}
			fmt.Printf("  [%d] %s %dx%d (%s)%s\n", i, f.Quality, f.Width, f.Height, f.Ext, audioInfo)
		}
		return nil
	}

	// Select best format (or by quality flag)
	format := selectVideoFormat(m.Formats, quality)
	if format == nil {
		return fmt.Errorf("%s", t.Download.NoFormats)
	}

	fmt.Printf("  %s: %s (%s)\n", t.Download.SelectedFormat, format.Quality, format.Ext)

	// Determine output filename
	outputFile := output
	if outputFile == "" {
		title := extractor.SanitizeFilename(m.Title)
		// For m3u8, output as .ts (MPEG-TS container)
		ext := format.Ext
		if ext == "m3u8" {
			ext = "ts"
		}
		if title != "" {
			outputFile = fmt.Sprintf("%s.%s", title, ext)
		} else {
			outputFile = fmt.Sprintf("%s.%s", m.ID, ext)
		}
		// Prepend outputDir if configured
		if outputDir != "" {
			outputFile = filepath.Join(outputDir, outputFile)
		}
	}

	// Use HLS downloader for m3u8 streams
	if format.Ext == "m3u8" {
		// Create directory with title to keep things organized
		title := extractor.SanitizeFilename(m.Title)
		if title == "" {
			title = m.ID
		}
		// Use outputDir as base if configured
		baseDir := title
		if outputDir != "" {
			baseDir = filepath.Join(outputDir, title)
		}
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		// Put output file inside the directory
		outputFile = filepath.Join(baseDir, filepath.Base(outputFile))
		fmt.Printf("  Output directory: %s/\n", baseDir)
		return downloader.RunHLSDownloadWithHeadersTUI(format.URL, outputFile, m.ID, lang, format.Headers)
	}

	// Handle video+audio as separate downloads
	if format.AudioURL != "" {
		return downloadVideoAndAudio(format, outputFile, m.ID, dl)
	}

	// Use headers if provided by the extractor
	if len(format.Headers) > 0 {
		return dl.DownloadWithHeaders(format.URL, outputFile, m.ID, format.Headers)
	}
	return dl.Download(format.URL, outputFile, m.ID)
}

// downloadVideoWithIndex downloads a video with an index suffix in the filename (for multi-video posts)
func downloadVideoWithIndex(m *extractor.VideoMedia, dl *downloader.Downloader, t *i18n.Translations, lang string, outputDir string, index, total int) error {
	// Info only mode
	if info {
		for i, f := range m.Formats {
			audioInfo := ""
			if f.AudioURL != "" {
				audioInfo = " [+audio]"
			}
			fmt.Printf("  [%d] %s %dx%d (%s)%s\n", i, f.Quality, f.Width, f.Height, f.Ext, audioInfo)
		}
		return nil
	}

	// Select best format (or by quality flag)
	format := selectVideoFormat(m.Formats, quality)
	if format == nil {
		return fmt.Errorf("%s", t.Download.NoFormats)
	}

	fmt.Printf("  %s: %s (%s)\n", t.Download.SelectedFormat, format.Quality, format.Ext)

	// Determine output filename
	outputFile := output
	if outputFile == "" {
		title := extractor.SanitizeFilename(m.Title)
		ext := format.Ext
		if ext == "m3u8" {
			ext = "ts"
		}
		baseName := title
		if baseName == "" {
			baseName = m.ID
		}
		// Add index suffix for multi-video
		if total > 1 {
			outputFile = fmt.Sprintf("%s_%d.%s", baseName, index, ext)
		} else {
			outputFile = fmt.Sprintf("%s.%s", baseName, ext)
		}
		// Prepend outputDir if configured
		if outputDir != "" {
			outputFile = filepath.Join(outputDir, outputFile)
		}
	}

	// Use HLS downloader for m3u8 streams
	if format.Ext == "m3u8" {
		title := extractor.SanitizeFilename(m.Title)
		if title == "" {
			title = m.ID
		}
		baseDir := title
		if outputDir != "" {
			baseDir = filepath.Join(outputDir, title)
		}
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		outputFile = filepath.Join(baseDir, filepath.Base(outputFile))
		fmt.Printf("  Output directory: %s/\n", baseDir)
		return downloader.RunHLSDownloadWithHeadersTUI(format.URL, outputFile, m.ID, lang, format.Headers)
	}

	// Handle video+audio as separate downloads
	if format.AudioURL != "" {
		return downloadVideoAndAudio(format, outputFile, m.ID, dl)
	}

	// Use headers if provided by the extractor
	if len(format.Headers) > 0 {
		return dl.DownloadWithHeaders(format.URL, outputFile, m.ID, format.Headers)
	}
	return dl.Download(format.URL, outputFile, m.ID)
}

// downloadVideoAndAudio downloads video and audio as separate files, then merges them if ffmpeg is available
func downloadVideoAndAudio(format *extractor.VideoFormat, outputFile, videoID string, dl *downloader.Downloader) error {
	// Determine audio extension based on video format
	audioExt := "m4a"
	if format.Ext == "webm" {
		audioExt = "opus"
	}

	// Build temp filenames for video and audio
	ext := filepath.Ext(outputFile)
	baseName := strings.TrimSuffix(outputFile, ext)
	videoFile := baseName + "_video" + ext
	audioFile := baseName + "_audio." + audioExt

	// Download video with headers if provided
	fmt.Println("  Downloading video stream...")
	var err error
	if len(format.Headers) > 0 {
		err = dl.DownloadWithHeaders(format.URL, videoFile, videoID+"-video", format.Headers)
	} else {
		err = dl.Download(format.URL, videoFile, videoID+"-video")
	}
	if err != nil {
		return fmt.Errorf("failed to download video: %w", err)
	}

	// Download audio with headers if provided
	fmt.Println("  Downloading audio stream...")
	if len(format.Headers) > 0 {
		err = dl.DownloadWithHeaders(format.AudioURL, audioFile, videoID+"-audio", format.Headers)
	} else {
		err = dl.Download(format.AudioURL, audioFile, videoID+"-audio")
	}
	if err != nil {
		return fmt.Errorf("failed to download audio: %w", err)
	}

	// Try to merge with ffmpeg if available
	if downloader.FFmpegAvailable() {
		fmt.Println("  Merging video and audio...")
		// Merge to final output path and delete temp files on success
		err := downloader.MergeVideoAudio(videoFile, audioFile, outputFile, true)
		if err != nil {
			fmt.Printf("\n  Warning: ffmpeg merge failed: %v\n", err)
			fmt.Printf("\n  Downloaded separately:\n")
			fmt.Printf("    Video: %s\n", videoFile)
			fmt.Printf("    Audio: %s\n", audioFile)
			fmt.Printf("\n  To merge manually:\n")
			fmt.Printf("    ffmpeg -i \"%s\" -i \"%s\" -c copy \"%s\"\n", videoFile, audioFile, outputFile)
		} else {
			fmt.Printf("\n  Downloaded: %s\n", outputFile)
		}
	} else {
		// No ffmpeg, show manual command
		fmt.Printf("\n  Downloaded:\n")
		fmt.Printf("    Video: %s\n", videoFile)
		fmt.Printf("    Audio: %s\n", audioFile)
		fmt.Printf("\n  To merge with ffmpeg:\n")
		fmt.Printf("    ffmpeg -i \"%s\" -i \"%s\" -c copy \"%s\"\n", videoFile, audioFile, outputFile)
	}

	return nil
}

func downloadAudio(m *extractor.AudioMedia, dl *downloader.Downloader, outputDir string) error {
	// Info only mode
	if info {
		fmt.Printf("  Audio: %s (%s)\n", m.Title, m.Ext)
		return nil
	}

	// Determine output filename
	outputFile := output
	if outputFile == "" {
		title := extractor.SanitizeFilename(m.Title)
		if title != "" {
			outputFile = fmt.Sprintf("%s.%s", title, m.Ext)
		} else {
			outputFile = fmt.Sprintf("%s.%s", m.ID, m.Ext)
		}
		// Prepend outputDir if configured
		if outputDir != "" {
			outputFile = filepath.Join(outputDir, outputFile)
		}
	}

	return dl.Download(m.URL, outputFile, m.ID)
}

func downloadImages(m *extractor.ImageMedia, dl *downloader.Downloader, outputDir string) error {
	// Info only mode
	if info {
		fmt.Printf("  Images (%d):\n", len(m.Images))
		for i, img := range m.Images {
			fmt.Printf("    [%d] %dx%d (%s)\n", i+1, img.Width, img.Height, img.Ext)
		}
		return nil
	}

	fmt.Printf("  Downloading %d image(s)...\n", len(m.Images))

	for i, img := range m.Images {
		var outputFile string
		if output != "" {
			// If custom output specified, add suffix for multiple images
			if len(m.Images) > 1 {
				outputFile = fmt.Sprintf("%s_%d.%s", output, i+1, img.Ext)
			} else {
				outputFile = fmt.Sprintf("%s.%s", output, img.Ext)
			}
		} else {
			// Use sanitized title or ID with index suffix
			baseFilename := m.ID
			if title := extractor.SanitizeFilename(m.Title); title != "" {
				baseFilename = title
			}
			if len(m.Images) > 1 {
				outputFile = fmt.Sprintf("%s_%d.%s", baseFilename, i+1, img.Ext)
			} else {
				outputFile = fmt.Sprintf("%s.%s", baseFilename, img.Ext)
			}
			// Prepend outputDir if configured
			if outputDir != "" {
				outputFile = filepath.Join(outputDir, outputFile)
			}
		}

		if err := dl.Download(img.URL, outputFile, m.ID); err != nil {
			return fmt.Errorf("failed to download image %d: %w", i+1, err)
		}
	}
	return nil
}

func selectVideoFormat(formats []extractor.VideoFormat, preferred string) *extractor.VideoFormat {
	if len(formats) == 0 {
		return nil
	}

	// If quality specified, try to match
	if preferred != "" {
		for i := range formats {
			if formats[i].Quality == preferred {
				return &formats[i]
			}
		}
		// Also try partial match (e.g., "1080" matches "1080p60")
		for i := range formats {
			if strings.Contains(formats[i].Quality, preferred) {
				return &formats[i]
			}
		}
	}

	// Prefer highest quality adaptive format with audio (will download both files)
	var bestWithAudio *extractor.VideoFormat
	for i := range formats {
		f := &formats[i]
		if f.AudioURL != "" {
			if bestWithAudio == nil || f.Bitrate > bestWithAudio.Bitrate {
				bestWithAudio = f
			}
		}
	}
	if bestWithAudio != nil {
		return bestWithAudio
	}

	// Then prefer combined formats (has audio, no separate download)
	for i := range formats {
		if formats[i].AudioURL == "" && formats[i].Bitrate > 0 && formats[i].Ext != "m3u8" {
			return &formats[i]
		}
	}

	// Fall back to HLS if nothing else
	for i := range formats {
		if formats[i].Ext == "m3u8" {
			return &formats[i]
		}
	}

	// Fall back to highest bitrate (may need ffmpeg merge)
	best := &formats[0]
	for i := range formats {
		if formats[i].Bitrate > best.Bitrate {
			best = &formats[i]
		}
	}
	return best
}
