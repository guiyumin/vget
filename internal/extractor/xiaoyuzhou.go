package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// XiaoyuzhouExtractor handles xiaoyuzhoufm.com podcast downloads
type XiaoyuzhouExtractor struct{}

func (e *XiaoyuzhouExtractor) Name() string {
	return "xiaoyuzhou"
}

func (e *XiaoyuzhouExtractor) Match(u *url.URL) bool {
	host := u.Hostname()
	if host != "xiaoyuzhoufm.com" && host != "www.xiaoyuzhoufm.com" {
		return false
	}
	return strings.HasPrefix(u.Path, "/episode/") || strings.HasPrefix(u.Path, "/podcast/")
}

func (e *XiaoyuzhouExtractor) Extract(url string) (*VideoInfo, error) {
	if strings.Contains(url, "/episode/") {
		return e.extractEpisode(url)
	}
	if strings.Contains(url, "/podcast/") {
		return e.extractPodcast(url)
	}
	return nil, fmt.Errorf("unsupported URL format")
}

// extractEpisode extracts a single episode
func (e *XiaoyuzhouExtractor) extractEpisode(url string) (*VideoInfo, error) {
	// Extract episode ID from URL
	re := regexp.MustCompile(`/episode/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract episode ID from URL")
	}
	episodeID := matches[1]

	// Fetch the episode page to get JSON data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the embedded JSON data from the page
	// Look for the script tag with __NEXT_DATA__
	content := string(body)
	jsonStart := strings.Index(content, `<script id="__NEXT_DATA__" type="application/json">`)
	if jsonStart == -1 {
		return nil, fmt.Errorf("could not find episode data in page")
	}

	jsonStart = strings.Index(content[jsonStart:], ">") + jsonStart + 1
	jsonEnd := strings.Index(content[jsonStart:], "</script>") + jsonStart

	if jsonEnd <= jsonStart {
		return nil, fmt.Errorf("could not parse episode data")
	}

	jsonData := content[jsonStart:jsonEnd]

	// Parse the JSON
	var pageData struct {
		Props struct {
			PageProps struct {
				Episode struct {
					Eid       string `json:"eid"`
					Title     string `json:"title"`
					Duration  int    `json:"duration"`
					Enclosure struct {
						URL string `json:"url"`
					} `json:"enclosure"`
					Podcast struct {
						Title string `json:"title"`
					} `json:"podcast"`
				} `json:"episode"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(jsonData), &pageData); err != nil {
		return nil, fmt.Errorf("failed to parse episode JSON: %v", err)
	}

	episode := pageData.Props.PageProps.Episode
	if episode.Enclosure.URL == "" {
		return nil, fmt.Errorf("no audio URL found for episode")
	}

	// Determine file extension
	ext := "m4a"
	if strings.Contains(episode.Enclosure.URL, ".mp3") {
		ext = "mp3"
	}

	// Create filename: {podcast} - {title}.{ext}
	filename := sanitizeFilename(fmt.Sprintf("%s - %s", episode.Podcast.Title, episode.Title))

	return &VideoInfo{
		ID:        episodeID,
		Title:     filename,
		Duration:  episode.Duration,
		Uploader:  episode.Podcast.Title,
		MediaType: MediaTypeAudio,
		Formats: []Format{
			{
				URL:     episode.Enclosure.URL,
				Quality: "audio",
				Ext:     ext,
			},
		},
	}, nil
}

// extractPodcast lists all episodes from a podcast
func (e *XiaoyuzhouExtractor) extractPodcast(url string) (*VideoInfo, error) {
	// For now, return an error suggesting to use search
	// Full podcast download can be implemented later
	return nil, fmt.Errorf("podcast download not yet implemented. Use 'vget search --podcast <name>' to find specific episodes")
}

// sanitizeFilename removes or replaces characters that are invalid in filenames
func sanitizeFilename(name string) string {
	// Replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
		"\n", " ",
		"\r", "",
	)
	result := replacer.Replace(name)

	// Trim spaces and dots from ends
	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")

	// Limit length
	if len(result) > 200 {
		result = result[:200]
	}

	return result
}

func init() {
	Register(&XiaoyuzhouExtractor{})
}
