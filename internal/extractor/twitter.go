package extractor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Public bearer token (same as used by web client)
	twitterBearerToken = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs=1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"

	twitterGuestTokenURL  = "https://api.x.com/1.1/guest/activate.json"
	twitterGraphQLURL     = "https://x.com/i/api/graphql/2ICDjqPd81tulZcYrtpTuQ/TweetResultByRestId"
	twitterSyndicationURL = "https://cdn.syndication.twimg.com/tweet-result"
)

var (
	// Matches twitter.com and x.com URLs with status
	twitterURLRegex = regexp.MustCompile(`(?:twitter\.com|x\.com)/(?:[^/]+)/status/(\d+)`)
)

// Twitter-specific error types for i18n support
type TwitterError struct {
	Code    string // "nsfw", "protected", "unavailable"
	Message string // Original message for fallback
}

func (e *TwitterError) Error() string {
	return e.Message
}

// Error code constants
const (
	TwitterErrorNSFW        = "nsfw"
	TwitterErrorProtected   = "protected"
	TwitterErrorUnavailable = "unavailable"
)

// TwitterExtractor handles Twitter/X media extraction
type TwitterExtractor struct {
	client     *http.Client
	guestToken string
	authToken  string // auth_token cookie for authenticated requests
	csrfToken  string // ct0 cookie for CSRF protection
}

// Name returns the extractor name
func (t *TwitterExtractor) Name() string {
	return "twitter"
}

// Match checks if URL is a Twitter/X status URL
func (t *TwitterExtractor) Match(u *url.URL) bool {
	// Host matching is done by registry, check path pattern
	return twitterURLRegex.MatchString(u.String())
}

// SetAuth sets authentication credentials for accessing restricted content
func (t *TwitterExtractor) SetAuth(authToken string) {
	t.authToken = authToken
}

// IsAuthenticated returns true if auth credentials are set
func (t *TwitterExtractor) IsAuthenticated() bool {
	return t.authToken != ""
}

// Extract retrieves media from a Twitter/X URL
func (t *TwitterExtractor) Extract(urlStr string) (Media, error) {
	// Initialize HTTP client
	if t.client == nil {
		t.client = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		}
	}

	// Extract tweet ID from URL
	matches := twitterURLRegex.FindStringSubmatch(urlStr)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract tweet ID from URL")
	}
	tweetID := matches[1]

	// If authenticated, use GraphQL API directly (supports NSFW content)
	if t.IsAuthenticated() {
		media, err := t.fetchFromGraphQLAuth(tweetID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tweet: %w", err)
		}
		return media, nil
	}

	// Try syndication API first (simpler, no auth needed for public tweets)
	media, err := t.fetchFromSyndication(tweetID)
	if err == nil {
		return media, nil
	}

	// Fallback to GraphQL API with guest token
	if err := t.fetchGuestToken(); err != nil {
		return nil, fmt.Errorf("failed to get guest token: %w", err)
	}

	media, err = t.fetchFromGraphQL(tweetID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tweet: %w", err)
	}

	return media, nil
}

// fetchFromSyndication tries the syndication endpoint (works for public tweets)
func (t *TwitterExtractor) fetchFromSyndication(tweetID string) (Media, error) {
	params := url.Values{}
	params.Set("id", tweetID)
	params.Set("token", "x") // Required but value doesn't matter

	reqURL := twitterSyndicationURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("syndication request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var data syndicationResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse syndication response: %w", err)
	}

	return t.parseSyndicationResponse(&data, tweetID)
}

// fetchGuestToken obtains a guest token for API access
func (t *TwitterExtractor) fetchGuestToken() error {
	req, err := http.NewRequest("POST", twitterGuestTokenURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+twitterBearerToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("guest token request failed with status %d", resp.StatusCode)
	}

	var result struct {
		GuestToken string `json:"guest_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	t.guestToken = result.GuestToken
	return nil
}

// fetchFromGraphQL uses the GraphQL API
func (t *TwitterExtractor) fetchFromGraphQL(tweetID string) (Media, error) {
	variables := map[string]interface{}{
		"tweetId":                tweetID,
		"withCommunity":          false,
		"includePromotedContent": false,
		"withVoice":              false,
	}

	// Features from yt-dlp (actively maintained)
	features := map[string]interface{}{
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	variablesJSON, _ := json.Marshal(variables)
	featuresJSON, _ := json.Marshal(features)

	params := url.Values{}
	params.Set("variables", string(variablesJSON))
	params.Set("features", string(featuresJSON))

	reqURL := twitterGraphQLURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+twitterBearerToken)
	req.Header.Set("x-guest-token", t.guestToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return t.parseGraphQLResponse(body, tweetID)
}

// fetchCsrfToken fetches the ct0 CSRF token by making a request to Twitter
func (t *TwitterExtractor) fetchCsrfToken() error {
	req, err := http.NewRequest("GET", "https://x.com", nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: t.authToken})

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "ct0" {
			t.csrfToken = cookie.Value
			return nil
		}
	}

	return fmt.Errorf("could not obtain CSRF token")
}

// fetchFromGraphQLAuth uses the GraphQL API with authentication (for NSFW content)
func (t *TwitterExtractor) fetchFromGraphQLAuth(tweetID string) (Media, error) {
	// Fetch CSRF token if not already set
	if t.csrfToken == "" {
		if err := t.fetchCsrfToken(); err != nil {
			return nil, fmt.Errorf("failed to get CSRF token: %w", err)
		}
	}

	variables := map[string]interface{}{
		"tweetId":                tweetID,
		"withCommunity":          false,
		"includePromotedContent": false,
		"withVoice":              false,
	}

	// Features from yt-dlp (actively maintained)
	features := map[string]interface{}{
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"tweetypie_unmention_optimization_enabled":                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                false,
		"tweet_awards_web_tipping_enabled":                                        false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"responsive_web_media_download_video_enabled":                             false,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	variablesJSON, _ := json.Marshal(variables)
	featuresJSON, _ := json.Marshal(features)

	params := url.Values{}
	params.Set("variables", string(variablesJSON))
	params.Set("features", string(featuresJSON))

	reqURL := twitterGraphQLURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Set authentication headers
	req.Header.Set("Authorization", "Bearer "+twitterBearerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("x-twitter-client-language", "en")
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("x-csrf-token", t.csrfToken)

	// Set cookies for authentication
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: t.authToken})
	req.AddCookie(&http.Cookie{Name: "ct0", Value: t.csrfToken})

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return t.parseGraphQLResponse(body, tweetID)
}

// parseSyndicationResponse extracts media from syndication API response
func (t *TwitterExtractor) parseSyndicationResponse(data *syndicationResponse, tweetID string) (Media, error) {
	if len(data.MediaDetails) == 0 {
		return nil, fmt.Errorf("no media found in tweet")
	}

	title := truncateText(data.Text, 100)
	uploader := data.User.ScreenName

	// Collect videos separately for each media item
	var videos []*VideoMedia
	var images []Image
	videoIndex := 0

	for _, media := range data.MediaDetails {
		switch media.Type {
		case "video", "animated_gif":
			var formats []VideoFormat
			for _, variant := range media.VideoInfo.Variants {
				if variant.ContentType != "video/mp4" {
					continue
				}

				format := VideoFormat{
					URL:     variant.URL,
					Ext:     "mp4",
					Bitrate: variant.Bitrate,
				}

				if w, h := extractResolutionFromURL(variant.URL); w > 0 {
					format.Width = w
					format.Height = h
					format.Quality = fmt.Sprintf("%dp", h)
				} else if variant.Bitrate > 0 {
					format.Quality = estimateQualityFromBitrate(variant.Bitrate)
				}

				formats = append(formats, format)
			}

			if len(formats) > 0 {
				// Sort by bitrate (highest first)
				sort.Slice(formats, func(i, j int) bool {
					if formats[i].Bitrate != formats[j].Bitrate {
						return formats[i].Bitrate > formats[j].Bitrate
					}
					return formats[i].Height > formats[j].Height
				})

				videoIndex++
				videos = append(videos, &VideoMedia{
					ID:       fmt.Sprintf("%s_%d", tweetID, videoIndex),
					Title:    title,
					Uploader: uploader,
					Formats:  formats,
				})
			}

		case "photo":
			imageURL := getHighQualityImageURL(media.MediaURLHTTPS)
			ext := getImageExtension(media.MediaURLHTTPS)

			img := Image{
				URL: imageURL,
				Ext: ext,
			}

			if media.OriginalWidth > 0 {
				img.Width = media.OriginalWidth
				img.Height = media.OriginalHeight
			}

			images = append(images, img)
		}
	}

	// Also check video field directly (for single video tweets)
	if len(videos) == 0 && data.Video.Variants != nil {
		var formats []VideoFormat
		for _, variant := range data.Video.Variants {
			if variant.Type != "video/mp4" {
				continue
			}

			format := VideoFormat{
				URL: variant.Src,
				Ext: "mp4",
			}

			if w, h := extractResolutionFromURL(variant.Src); w > 0 {
				format.Width = w
				format.Height = h
				format.Quality = fmt.Sprintf("%dp", h)
			}

			formats = append(formats, format)
		}

		if len(formats) > 0 {
			videos = append(videos, &VideoMedia{
				ID:       tweetID,
				Title:    title,
				Uploader: uploader,
				Formats:  formats,
			})
		}
	}

	// Return appropriate media type
	if len(videos) > 1 {
		// Multiple videos - return MultiVideoMedia
		return &MultiVideoMedia{
			ID:       tweetID,
			Title:    title,
			Uploader: uploader,
			Videos:   videos,
		}, nil
	}

	if len(videos) == 1 {
		// Single video - return VideoMedia directly
		videos[0].ID = tweetID // Use original tweet ID for single video
		return videos[0], nil
	}

	if len(images) > 0 {
		return &ImageMedia{
			ID:       tweetID,
			Title:    title,
			Uploader: uploader,
			Images:   images,
		}, nil
	}

	return nil, fmt.Errorf("no media found in tweet")
}

// parseGraphQLResponse extracts media from GraphQL API response
func (t *TwitterExtractor) parseGraphQLResponse(body []byte, tweetID string) (Media, error) {
	var resp graphQLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	result := resp.Data.TweetResult.Result
	if result == nil {
		return nil, fmt.Errorf("tweet not found or not accessible")
	}

	// Handle different result types
	switch result.TypeName {
	case "TweetTombstone":
		msg := "tweet is unavailable"
		if result.Tombstone != nil && result.Tombstone.Text.Text != "" {
			msg = result.Tombstone.Text.Text
		}
		return nil, &TwitterError{Code: TwitterErrorUnavailable, Message: msg}
	case "TweetUnavailable":
		switch result.Reason {
		case "NsfwLoggedOut":
			return nil, &TwitterError{Code: TwitterErrorNSFW, Message: "age-restricted content requires login"}
		case "Protected":
			return nil, &TwitterError{Code: TwitterErrorProtected, Message: "protected tweet requires authorization"}
		default:
			msg := "tweet is unavailable"
			if result.Reason != "" {
				msg = fmt.Sprintf("tweet unavailable: %s", result.Reason)
			}
			return nil, &TwitterError{Code: TwitterErrorUnavailable, Message: msg}
		}
	}

	// Handle tweet with visibility results
	legacy := result.Legacy
	if legacy == nil && result.Tweet != nil {
		legacy = result.Tweet.Legacy
	}

	if legacy == nil {
		return nil, fmt.Errorf("could not find tweet data (type: %s)", result.TypeName)
	}

	title := truncateText(legacy.FullText, 100)
	var uploader string
	if result.Core != nil && result.Core.UserResults.Result != nil {
		uploader = result.Core.UserResults.Result.Legacy.ScreenName
	}

	if legacy.ExtendedEntities == nil || len(legacy.ExtendedEntities.Media) == 0 {
		return nil, fmt.Errorf("no media found in tweet")
	}

	// Collect videos separately for each media item
	var videos []*VideoMedia
	var images []Image
	videoIndex := 0

	for _, media := range legacy.ExtendedEntities.Media {
		switch media.Type {
		case "video", "animated_gif":
			var formats []VideoFormat
			duration := media.VideoInfo.DurationMillis / 1000

			for _, variant := range media.VideoInfo.Variants {
				if variant.ContentType != "video/mp4" {
					continue
				}

				format := VideoFormat{
					URL:     variant.URL,
					Ext:     "mp4",
					Bitrate: variant.Bitrate,
				}

				if w, h := extractResolutionFromURL(variant.URL); w > 0 {
					format.Width = w
					format.Height = h
					format.Quality = fmt.Sprintf("%dp", h)
				} else if variant.Bitrate > 0 {
					format.Quality = estimateQualityFromBitrate(variant.Bitrate)
				}

				formats = append(formats, format)
			}

			if len(formats) > 0 {
				// Sort by bitrate (highest first)
				sort.Slice(formats, func(i, j int) bool {
					return formats[i].Bitrate > formats[j].Bitrate
				})

				videoIndex++
				videos = append(videos, &VideoMedia{
					ID:       fmt.Sprintf("%s_%d", tweetID, videoIndex),
					Title:    title,
					Uploader: uploader,
					Duration: duration,
					Formats:  formats,
				})
			}

		case "photo":
			imageURL := getHighQualityImageURL(media.MediaURLHTTPS)
			ext := getImageExtension(media.MediaURLHTTPS)

			img := Image{
				URL: imageURL,
				Ext: ext,
			}

			if media.OriginalInfo.Width > 0 {
				img.Width = media.OriginalInfo.Width
				img.Height = media.OriginalInfo.Height
			}

			images = append(images, img)
		}
	}

	// Return appropriate media type
	if len(videos) > 1 {
		// Multiple videos - return MultiVideoMedia
		return &MultiVideoMedia{
			ID:       tweetID,
			Title:    title,
			Uploader: uploader,
			Videos:   videos,
		}, nil
	}

	if len(videos) == 1 {
		// Single video - return VideoMedia directly
		videos[0].ID = tweetID // Use original tweet ID for single video
		return videos[0], nil
	}

	if len(images) > 0 {
		return &ImageMedia{
			ID:       tweetID,
			Title:    title,
			Uploader: uploader,
			Images:   images,
		}, nil
	}

	return nil, fmt.Errorf("no media found in tweet")
}

// Syndication API response structures
type syndicationResponse struct {
	Text string `json:"text"`
	User struct {
		ScreenName string `json:"screen_name"`
		Name       string `json:"name"`
	} `json:"user"`
	MediaDetails []struct {
		Type           string `json:"type"`
		MediaURLHTTPS  string `json:"media_url_https"`
		OriginalWidth  int    `json:"original_info_width"`
		OriginalHeight int    `json:"original_info_height"`
		VideoInfo      struct {
			Variants []struct {
				Bitrate     int    `json:"bitrate"`
				ContentType string `json:"content_type"`
				URL         string `json:"url"`
			} `json:"variants"`
		} `json:"video_info"`
	} `json:"mediaDetails"`
	Video struct {
		Variants []struct {
			Type string `json:"type"`
			Src  string `json:"src"`
		} `json:"variants"`
	} `json:"video"`
}

// GraphQL API response structures
type graphQLResponse struct {
	Data struct {
		TweetResult struct {
			Result *graphQLTweetResult `json:"result"`
		} `json:"tweetResult"`
	} `json:"data"`
}

type graphQLTweetResult struct {
	TypeName  string              `json:"__typename"`
	Legacy    *graphQLLegacy      `json:"legacy"`
	Core      *graphQLCore        `json:"core"`
	Tweet     *graphQLTweetResult `json:"tweet"`     // For TweetWithVisibilityResults
	Reason    string              `json:"reason"`    // For TweetUnavailable
	Tombstone *struct {
		Text struct {
			Text string `json:"text"`
		} `json:"text"`
	} `json:"tombstone"` // For TweetTombstone
}

type graphQLCore struct {
	UserResults struct {
		Result *struct {
			Legacy struct {
				ScreenName string `json:"screen_name"`
			} `json:"legacy"`
		} `json:"result"`
	} `json:"user_results"`
}

type graphQLLegacy struct {
	FullText         string `json:"full_text"`
	ExtendedEntities *struct {
		Media []struct {
			Type          string `json:"type"`
			MediaURLHTTPS string `json:"media_url_https"`
			OriginalInfo  struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"original_info"`
			VideoInfo struct {
				DurationMillis int `json:"duration_millis"`
				Variants       []struct {
					Bitrate     int    `json:"bitrate"`
					ContentType string `json:"content_type"`
					URL         string `json:"url"`
				} `json:"variants"`
			} `json:"video_info"`
		} `json:"media"`
	} `json:"extended_entities"`
}

// Helper functions

func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

var resolutionRegex = regexp.MustCompile(`/(\d+)x(\d+)/`)

func extractResolutionFromURL(url string) (width, height int) {
	matches := resolutionRegex.FindStringSubmatch(url)
	if len(matches) >= 3 {
		w, _ := strconv.Atoi(matches[1])
		h, _ := strconv.Atoi(matches[2])
		return w, h
	}
	return 0, 0
}

func estimateQualityFromBitrate(bitrate int) string {
	switch {
	case bitrate >= 2000000:
		return "1080p"
	case bitrate >= 1000000:
		return "720p"
	case bitrate >= 500000:
		return "480p"
	default:
		return "360p"
	}
}

// getHighQualityImageURL converts a Twitter image URL to highest quality version
func getHighQualityImageURL(imageURL string) string {
	baseURL := strings.Split(imageURL, "?")[0]

	format := "jpg"
	if strings.Contains(baseURL, ".png") {
		format = "png"
	} else if strings.Contains(baseURL, ".webp") {
		format = "webp"
	}

	return baseURL + "?format=" + format + "&name=orig"
}

// getImageExtension extracts the image extension from URL
func getImageExtension(imageURL string) string {
	baseURL := strings.Split(imageURL, "?")[0]
	if strings.HasSuffix(baseURL, ".png") {
		return "png"
	} else if strings.HasSuffix(baseURL, ".webp") {
		return "webp"
	} else if strings.HasSuffix(baseURL, ".gif") {
		return "gif"
	}
	return "jpg"
}

func init() {
	Register(&TwitterExtractor{},
		"twitter.com",
		"x.com",
		"mobile.twitter.com",
		"mobile.x.com",
	)
}
