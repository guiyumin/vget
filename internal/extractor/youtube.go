package extractor

import (
	"bufio"
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// YouTubeDockerRequiredError indicates YouTube extraction needs Docker
type YouTubeDockerRequiredError struct {
	URL string
}

func (e *YouTubeDockerRequiredError) Error() string {
	return "YouTube extraction requires Docker"
}

// YouTubeDirectDownload indicates yt-dlp should handle the download directly
type YouTubeDirectDownload struct {
	URL       string
	OutputDir string
}

// Implement Media interface for YouTubeDirectDownload
func (y *YouTubeDirectDownload) GetID() string       { return y.URL }
func (y *YouTubeDirectDownload) GetTitle() string    { return "YouTube Video" }
func (y *YouTubeDirectDownload) GetUploader() string { return "" }
func (y *YouTubeDirectDownload) Type() MediaType     { return MediaTypeVideo }

// ytdlpExtractor uses yt-dlp/youtube-dl for YouTube extraction (Docker only)
type ytdlpExtractor struct{}

func (e *ytdlpExtractor) Name() string {
	return "YouTube (yt-dlp)"
}

func (e *ytdlpExtractor) Match(u *url.URL) bool {
	host := strings.ToLower(u.Host)
	return host == "youtube.com" ||
		host == "www.youtube.com" ||
		host == "youtu.be" ||
		host == "m.youtube.com" ||
		host == "music.youtube.com"
}

func (e *ytdlpExtractor) Extract(urlStr string) (Media, error) {
	if !isRunningInDocker() {
		return nil, &YouTubeDockerRequiredError{URL: urlStr}
	}

	// For YouTube, we return a special marker that tells the CLI
	// to use yt-dlp for direct download instead of vget's downloader
	// OutputDir will be set by CLI from config
	return &YouTubeDirectDownload{
		URL: urlStr,
	}, nil
}

// DownloadWithYtdlp downloads a YouTube video using yt-dlp directly
func DownloadWithYtdlp(url, outputDir string) error {
	return DownloadWithYtdlpProgress(context.Background(), url, outputDir, nil)
}

// DownloadWithYtdlpProgress downloads a YouTube video using yt-dlp with progress callback
func DownloadWithYtdlpProgress(ctx context.Context, url, outputDir string, progressFn func(downloaded, total int64)) error {
	outputTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"-f", "bv*+ba/b", // best video + best audio, or best combined
		"--merge-output-format", "mp4",
		"--no-playlist",
		"--newline",                         // Output progress on new lines for parsing
		"--remote-components", "ejs:github", // download JS challenge solver
		"-o", outputTemplate,
		url,
	)

	// If no progress callback, just run normally
	if progressFn == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err == nil {
			return nil
		}
		// Fallback to youtube-dl
		return downloadWithYoutubeDL(ctx, url, outputDir)
	}

	// Parse progress from stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return downloadWithYoutubeDL(ctx, url, outputDir)
	}

	// Parse yt-dlp progress output
	// Format: [download]  45.2% of  150.00MiB at  5.00MiB/s ETA 00:15
	progressRe := regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~?(\d+\.?\d*)(Ki|Mi|Gi)?B`)

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		matches := progressRe.FindStringSubmatch(line)
		if len(matches) >= 3 {
			percent, _ := strconv.ParseFloat(matches[1], 64)
			size, _ := strconv.ParseFloat(matches[2], 64)

			// Convert size to bytes
			multiplier := int64(1)
			if len(matches) >= 4 {
				switch matches[3] {
				case "Ki":
					multiplier = 1024
				case "Mi":
					multiplier = 1024 * 1024
				case "Gi":
					multiplier = 1024 * 1024 * 1024
				}
			}

			totalBytes := int64(size * float64(multiplier))
			downloadedBytes := int64(float64(totalBytes) * percent / 100)
			progressFn(downloadedBytes, totalBytes)
		}
	}

	err = cmd.Wait()
	if err == nil {
		return nil
	}

	// Fallback to youtube-dl
	return downloadWithYoutubeDL(ctx, url, outputDir)
}

func downloadWithYoutubeDL(ctx context.Context, url, outputDir string) error {
	outputTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")
	cmd := exec.CommandContext(ctx, "youtube-dl",
		"-f", "bestvideo+bestaudio/best",
		"--merge-output-format", "mp4",
		"--no-playlist",
		"-o", outputTemplate,
		url,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isRunningInDocker detects if we're running inside a Docker container
func isRunningInDocker() bool {
	// Method 1: Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check cgroup (Linux)
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
			return true
		}
	}

	// Method 3: Check for kubernetes
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	return false
}

func init() {
	Register(&ytdlpExtractor{},
		"youtube.com",
		"www.youtube.com",
		"youtu.be",
		"m.youtube.com",
		"music.youtube.com",
	)
}
