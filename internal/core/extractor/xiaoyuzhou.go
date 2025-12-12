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
	// Host matching is done by registry, check path pattern
	return strings.HasPrefix(u.Path, "/episode/") || strings.HasPrefix(u.Path, "/podcast/")
}

func (e *XiaoyuzhouExtractor) Extract(url string) (Media, error) {
	if strings.Contains(url, "/episode/") {
		return e.extractEpisode(url)
	}
	if strings.Contains(url, "/podcast/") {
		return e.extractPodcast(url)
	}
	return nil, fmt.Errorf("unsupported URL format")
}

// extractEpisode extracts a single episode
func (e *XiaoyuzhouExtractor) extractEpisode(url string) (*AudioMedia, error) {
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

	// Create filename: {podcast} - {title}
	filename := SanitizeFilename(fmt.Sprintf("%s - %s", episode.Podcast.Title, episode.Title))

	return &AudioMedia{
		ID:       episodeID,
		Title:    filename,
		Uploader: episode.Podcast.Title,
		Duration: episode.Duration,
		URL:      episode.Enclosure.URL,
		Ext:      ext,
	}, nil
}

// extractPodcast lists all episodes from a podcast
func (e *XiaoyuzhouExtractor) extractPodcast(_ string) (*AudioMedia, error) {
	// For now, return an error suggesting to use search
	// Full podcast download can be implemented later
	return nil, fmt.Errorf("podcast download not yet implemented. Use 'vget search --podcast <name>' to find specific episodes")
}


func init() {
	Register(&XiaoyuzhouExtractor{},
		"xiaoyuzhoufm.com",
	)
}
