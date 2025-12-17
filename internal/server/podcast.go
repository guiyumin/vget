package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"unicode"

	"github.com/gin-gonic/gin"
)

// Podcast search types

type PodcastSearchRequest struct {
	Query string `json:"query" binding:"required"`
	Lang  string `json:"lang"` // language code, defaults to config language
}

type PodcastSearchResult struct {
	Source   string           `json:"source"`   // "xiaoyuzhou" or "itunes"
	Podcasts []PodcastChannel `json:"podcasts"` // podcast channels
	Episodes []PodcastEpisode `json:"episodes"` // individual episodes
}

type PodcastChannel struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	Description  string `json:"description"`
	EpisodeCount int    `json:"episode_count"`
	FeedURL      string `json:"feed_url,omitempty"` // iTunes only
	Source       string `json:"source"`             // "xiaoyuzhou" or "itunes"
}

type PodcastEpisode struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PodcastName string `json:"podcast_name"`
	Duration    int    `json:"duration"` // seconds
	PubDate     string `json:"pub_date,omitempty"`
	DownloadURL string `json:"download_url"`
	Source      string `json:"source"` // "xiaoyuzhou" or "itunes"
}

type PodcastEpisodesRequest struct {
	PodcastID string `json:"podcast_id" binding:"required"`
	Source    string `json:"source" binding:"required"` // "xiaoyuzhou" or "itunes"
}

// containsChinese checks if string contains Chinese characters
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// handlePodcastSearch handles POST /api/podcast/search
func (s *Server) handlePodcastSearch(c *gin.Context) {
	var req PodcastSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "query is required",
		})
		return
	}

	lang := req.Lang
	if lang == "" {
		lang = s.cfg.Language
	}
	if lang == "" {
		lang = "zh"
	}

	var results []PodcastSearchResult

	// Determine which sources to search based on language and query
	if lang == "zh" {
		if containsChinese(req.Query) {
			// Chinese query: search xiaoyuzhou only
			result, err := searchXiaoyuzhouAPI(req.Query)
			if err != nil {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    500,
					Data:    nil,
					Message: fmt.Sprintf("xiaoyuzhou search failed: %v", err),
				})
				return
			}
			results = append(results, *result)
		} else {
			// English query with zh lang: search both
			var wg sync.WaitGroup
			var mu sync.Mutex
			var errors []string

			wg.Add(2)

			go func() {
				defer wg.Done()
				result, err := searchXiaoyuzhouAPI(req.Query)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("xiaoyuzhou: %v", err))
					mu.Unlock()
					return
				}
				mu.Lock()
				results = append(results, *result)
				mu.Unlock()
			}()

			go func() {
				defer wg.Done()
				result, err := searchITunesAPI(req.Query)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("itunes: %v", err))
					mu.Unlock()
					return
				}
				mu.Lock()
				results = append(results, *result)
				mu.Unlock()
			}()

			wg.Wait()

			if len(results) == 0 && len(errors) > 0 {
				c.JSON(http.StatusInternalServerError, Response{
					Code:    500,
					Data:    nil,
					Message: strings.Join(errors, "; "),
				})
				return
			}
		}
	} else {
		// Non-zh language: search iTunes only
		result, err := searchITunesAPI(req.Query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Code:    500,
				Data:    nil,
				Message: fmt.Sprintf("itunes search failed: %v", err),
			})
			return
		}
		results = append(results, *result)
	}

	c.JSON(http.StatusOK, Response{
		Code:    200,
		Data:    gin.H{"results": results},
		Message: "search completed",
	})
}

// handlePodcastEpisodes handles POST /api/podcast/episodes
func (s *Server) handlePodcastEpisodes(c *gin.Context) {
	var req PodcastEpisodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "podcast_id and source are required",
		})
		return
	}

	var episodes []PodcastEpisode
	var podcastTitle string
	var err error

	switch req.Source {
	case "xiaoyuzhou":
		episodes, podcastTitle, err = fetchXiaoyuzhouEpisodesAPI(req.PodcastID)
	case "itunes":
		episodes, podcastTitle, err = fetchITunesEpisodesAPI(req.PodcastID)
	default:
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Data:    nil,
			Message: "invalid source: must be xiaoyuzhou or itunes",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Data:    nil,
			Message: fmt.Sprintf("failed to fetch episodes: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code: 200,
		Data: gin.H{
			"podcast_title": podcastTitle,
			"episodes":      episodes,
		},
		Message: fmt.Sprintf("%d episodes found", len(episodes)),
	})
}

// Xiaoyuzhou API functions

type xiaoyuzhouSearchResponse struct {
	Data struct {
		Episodes []struct {
			Eid       string `json:"eid"`
			Pid       string `json:"pid"`
			Title     string `json:"title"`
			Duration  int    `json:"duration"`
			PlayCount int    `json:"playCount"`
			PubDate   string `json:"pubDate"`
			Enclosure struct {
				URL string `json:"url"`
			} `json:"enclosure"`
			Podcast struct {
				Title string `json:"title"`
			} `json:"podcast"`
		} `json:"episodes"`
		Podcasts []struct {
			Pid               string `json:"pid"`
			Title             string `json:"title"`
			Author            string `json:"author"`
			Brief             string `json:"brief"`
			SubscriptionCount int    `json:"subscriptionCount"`
			EpisodeCount      int    `json:"episodeCount"`
		} `json:"podcasts"`
	} `json:"data"`
}

func searchXiaoyuzhouAPI(query string) (*PodcastSearchResult, error) {
	apiURL := "https://ask.xiaoyuzhoufm.com/api/keyword/search"
	payload := fmt.Sprintf(`{"query": "%s"}`, query)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result xiaoyuzhouSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	searchResult := &PodcastSearchResult{
		Source:   "xiaoyuzhou",
		Podcasts: make([]PodcastChannel, 0),
		Episodes: make([]PodcastEpisode, 0),
	}

	for _, p := range result.Data.Podcasts {
		searchResult.Podcasts = append(searchResult.Podcasts, PodcastChannel{
			ID:           p.Pid,
			Title:        p.Title,
			Author:       p.Author,
			Description:  p.Brief,
			EpisodeCount: p.EpisodeCount,
			Source:       "xiaoyuzhou",
		})
	}

	for _, e := range result.Data.Episodes {
		searchResult.Episodes = append(searchResult.Episodes, PodcastEpisode{
			ID:          e.Eid,
			Title:       e.Title,
			PodcastName: e.Podcast.Title,
			Duration:    e.Duration,
			PubDate:     e.PubDate,
			DownloadURL: e.Enclosure.URL,
			Source:      "xiaoyuzhou",
		})
	}

	return searchResult, nil
}

func fetchXiaoyuzhouEpisodesAPI(podcastID string) ([]PodcastEpisode, string, error) {
	pageURL := fmt.Sprintf("https://www.xiaoyuzhoufm.com/podcast/%s", podcastID)

	resp, err := http.Get(pageURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	html := string(body)
	startMarker := `<script id="__NEXT_DATA__" type="application/json">`
	endMarker := `</script>`

	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		return nil, "", fmt.Errorf("could not find episode data on page")
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(html[startIdx:], endMarker)
	if endIdx == -1 {
		return nil, "", fmt.Errorf("could not parse episode data")
	}

	jsonData := html[startIdx : startIdx+endIdx]

	var nextData struct {
		Props struct {
			PageProps struct {
				Podcast struct {
					Title    string `json:"title"`
					Episodes []struct {
						Eid       string `json:"eid"`
						Title     string `json:"title"`
						Duration  int    `json:"duration"`
						Enclosure struct {
							URL string `json:"url"`
						} `json:"enclosure"`
					} `json:"episodes"`
				} `json:"podcast"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal([]byte(jsonData), &nextData); err != nil {
		return nil, "", fmt.Errorf("failed to parse episode data: %v", err)
	}

	podcast := nextData.Props.PageProps.Podcast
	if len(podcast.Episodes) == 0 {
		return nil, podcast.Title, nil
	}

	var episodes []PodcastEpisode
	for _, e := range podcast.Episodes {
		episodes = append(episodes, PodcastEpisode{
			ID:          e.Eid,
			Title:       e.Title,
			PodcastName: podcast.Title,
			Duration:    e.Duration,
			DownloadURL: e.Enclosure.URL,
			Source:      "xiaoyuzhou",
		})
	}

	return episodes, podcast.Title, nil
}

// iTunes API functions

type iTunesSearchResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		WrapperType          string `json:"wrapperType"`
		Kind                 string `json:"kind"`
		CollectionID         int    `json:"collectionId"`
		TrackID              int    `json:"trackId"`
		ArtistName           string `json:"artistName"`
		CollectionName       string `json:"collectionName"`
		TrackName            string `json:"trackName"`
		FeedURL              string `json:"feedUrl"`
		TrackCount           int    `json:"trackCount"`
		PrimaryGenreName     string `json:"primaryGenreName"`
		ReleaseDate          string `json:"releaseDate"`
		TrackTimeMillis      int    `json:"trackTimeMillis"`
		EpisodeURL           string `json:"episodeUrl"`
		EpisodeFileExtension string `json:"episodeFileExtension"`
		ShortDescription     string `json:"shortDescription"`
	} `json:"results"`
}

func searchITunesAPI(query string) (*PodcastSearchResult, error) {
	var wg sync.WaitGroup
	var podcastResult, episodeResult iTunesSearchResponse
	var podcastErr, episodeErr error

	wg.Add(2)

	// Fetch podcasts
	go func() {
		defer wg.Done()
		podcastURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&entity=podcast&limit=50",
			url.QueryEscape(query))
		resp, err := http.Get(podcastURL)
		if err != nil {
			podcastErr = err
			return
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&podcastResult); err != nil {
			podcastErr = err
		}
	}()

	// Fetch episodes
	go func() {
		defer wg.Done()
		episodeURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&media=podcast&entity=podcastEpisode&limit=200",
			url.QueryEscape(query))
		resp, err := http.Get(episodeURL)
		if err != nil {
			episodeErr = err
			return
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&episodeResult); err != nil {
			episodeErr = err
		}
	}()

	wg.Wait()

	if podcastErr != nil && episodeErr != nil {
		return nil, fmt.Errorf("both searches failed: %v; %v", podcastErr, episodeErr)
	}

	searchResult := &PodcastSearchResult{
		Source:   "itunes",
		Podcasts: make([]PodcastChannel, 0),
		Episodes: make([]PodcastEpisode, 0),
	}

	for _, p := range podcastResult.Results {
		searchResult.Podcasts = append(searchResult.Podcasts, PodcastChannel{
			ID:           fmt.Sprintf("%d", p.CollectionID),
			Title:        p.CollectionName,
			Author:       p.ArtistName,
			Description:  p.ShortDescription,
			EpisodeCount: p.TrackCount,
			FeedURL:      p.FeedURL,
			Source:       "itunes",
		})
	}

	for _, e := range episodeResult.Results {
		searchResult.Episodes = append(searchResult.Episodes, PodcastEpisode{
			ID:          fmt.Sprintf("%d", e.TrackID),
			Title:       e.TrackName,
			PodcastName: e.CollectionName,
			Duration:    e.TrackTimeMillis / 1000,
			PubDate:     e.ReleaseDate,
			DownloadURL: e.EpisodeURL,
			Source:      "itunes",
		})
	}

	return searchResult, nil
}

func fetchITunesEpisodesAPI(podcastID string) ([]PodcastEpisode, string, error) {
	lookupURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&entity=podcastEpisode&limit=50", podcastID)

	resp, err := http.Get(lookupURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var result struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			WrapperType          string `json:"wrapperType"`
			TrackID              int    `json:"trackId"`
			TrackName            string `json:"trackName"`
			CollectionName       string `json:"collectionName"`
			ArtistName           string `json:"artistName"`
			EpisodeURL           string `json:"episodeUrl"`
			EpisodeFileExtension string `json:"episodeFileExtension"`
			TrackTimeMillis      int    `json:"trackTimeMillis"`
			ReleaseDate          string `json:"releaseDate"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", err
	}

	var episodes []PodcastEpisode
	var podcastTitle string

	for _, r := range result.Results {
		// Skip the podcast itself (first result is usually the podcast info)
		if r.WrapperType != "podcastEpisode" {
			if podcastTitle == "" {
				podcastTitle = r.CollectionName
			}
			continue
		}

		if podcastTitle == "" {
			podcastTitle = r.CollectionName
		}

		episodes = append(episodes, PodcastEpisode{
			ID:          fmt.Sprintf("%d", r.TrackID),
			Title:       r.TrackName,
			PodcastName: r.CollectionName,
			Duration:    r.TrackTimeMillis / 1000,
			PubDate:     r.ReleaseDate,
			DownloadURL: r.EpisodeURL,
			Source:      "itunes",
		})
	}

	return episodes, podcastTitle, nil
}
