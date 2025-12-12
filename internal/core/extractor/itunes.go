package extractor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

// iTunesExtractor handles Apple Podcasts downloads via iTunes API
type iTunesExtractor struct{}

func (e *iTunesExtractor) Name() string {
	return "itunes"
}

// Match URLs like:
// https://podcasts.apple.com/podcast/id173001861
// https://podcasts.apple.com/us/podcast/dan-carlins-hardcore-history/id173001861
// https://podcasts.apple.com/us/podcast/dan-carlins-hardcore-history/id173001861?i=1000682587885
var applePodcastRegex = regexp.MustCompile(`/(?:podcast/)?(?:[^/]+/)?id(\d+)`)

func (e *iTunesExtractor) Match(u *url.URL) bool {
	// Host matching is done by registry
	return true
}

func (e *iTunesExtractor) Extract(rawURL string) (Media, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	matches := applePodcastRegex.FindStringSubmatch(u.Path)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract podcast ID from URL")
	}

	podcastID := matches[1]
	episodeID := u.Query().Get("i")

	// If episode ID provided, fetch that specific episode
	if episodeID != "" {
		return e.extractEpisode(podcastID, episodeID)
	}

	// Otherwise list episodes from the podcast
	return e.listEpisodes()
}

func (e *iTunesExtractor) extractEpisode(podcastID, episodeID string) (*AudioMedia, error) {
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

			// Create filename: {podcast} - {episode}
			filename := SanitizeFilename(fmt.Sprintf("%s - %s", item.CollectionName, item.TrackName))

			return &AudioMedia{
				ID:       episodeID,
				Title:    filename,
				Uploader: item.ArtistName,
				Duration: item.TrackTimeMillis / 1000,
				URL:      item.EpisodeURL,
				Ext:      ext,
			}, nil
		}
	}

	return nil, fmt.Errorf("episode not found")
}

func (e *iTunesExtractor) listEpisodes() (*AudioMedia, error) {
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
	Register(&iTunesExtractor{},
		"podcasts.apple.com",
	)
}
