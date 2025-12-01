package extractor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// AppleExtractor handles Apple Podcasts downloads
type AppleExtractor struct{}

func (e *AppleExtractor) Name() string {
	return "apple"
}

// Match URLs like:
// https://podcasts.apple.com/podcast/id173001861
// https://podcasts.apple.com/us/podcast/dan-carlins-hardcore-history/id173001861
// https://podcasts.apple.com/us/podcast/dan-carlins-hardcore-history/id173001861?i=1000682587885
var applePodcastRegex = regexp.MustCompile(`podcasts\.apple\.com.*?/(?:podcast/)?(?:[^/]+/)?id(\d+)(?:\?i=(\d+))?`)

func (e *AppleExtractor) Match(url string) bool {
	return strings.Contains(url, "podcasts.apple.com")
}

func (e *AppleExtractor) Extract(url string) (*VideoInfo, error) {
	matches := applePodcastRegex.FindStringSubmatch(url)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract podcast ID from URL")
	}

	podcastID := matches[1]
	var episodeID string
	if len(matches) >= 3 && matches[2] != "" {
		episodeID = matches[2]
	}

	// If episode ID provided, fetch that specific episode
	if episodeID != "" {
		return e.extractEpisode(podcastID, episodeID)
	}

	// Otherwise list episodes from the podcast
	return e.listEpisodes(podcastID)
}

func (e *AppleExtractor) extractEpisode(podcastID, episodeID string) (*VideoInfo, error) {
	// Lookup episode by ID
	url := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&entity=podcastEpisode", podcastID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result iTunesLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Find the specific episode
	for _, item := range result.Results {
		if item.WrapperType == "podcastEpisode" && fmt.Sprintf("%d", item.TrackID) == episodeID {
			ext := item.EpisodeFileExtension
			if ext == "" {
				ext = "mp3"
			}

			// Create filename: {podcast} - {episode}.{ext}
			filename := sanitizeFilename(fmt.Sprintf("%s - %s", item.CollectionName, item.TrackName))

			return &VideoInfo{
				ID:        episodeID,
				Title:     filename,
				Duration:  item.TrackTimeMillis / 1000,
				Uploader:  item.ArtistName,
				MediaType: MediaTypeAudio,
				Formats: []Format{
					{
						URL:     item.EpisodeURL,
						Quality: "audio",
						Ext:     ext,
					},
				},
			}, nil
		}
	}

	return nil, fmt.Errorf("episode not found")
}

func (e *AppleExtractor) listEpisodes(podcastID string) (*VideoInfo, error) {
	// Return error suggesting to use specific episode URL
	// We could list episodes here but for now keep it simple
	return nil, fmt.Errorf("please provide a specific episode URL. Use 'vget search --podcast <name>' to find episodes, or visit the podcast page and select an episode")
}

// iTunes API response structures
type iTunesLookupResponse struct {
	ResultCount int                  `json:"resultCount"`
	Results     []iTunesLookupResult `json:"results"`
}

type iTunesLookupResult struct {
	WrapperType          string `json:"wrapperType"`
	Kind                 string `json:"kind"`
	TrackID              int    `json:"trackId"`
	ArtistName           string `json:"artistName"`
	CollectionName       string `json:"collectionName"`
	TrackName            string `json:"trackName"`
	TrackTimeMillis      int    `json:"trackTimeMillis"`
	EpisodeURL           string `json:"episodeUrl"`
	EpisodeFileExtension string `json:"episodeFileExtension"`
	ReleaseDate          string `json:"releaseDate"`
}

func init() {
	Register(&AppleExtractor{})
}
