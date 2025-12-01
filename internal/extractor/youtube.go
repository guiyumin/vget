package extractor

import (
	"fmt"
	"net/url"
)

// YouTubeExtractor handles YouTube video downloads
type YouTubeExtractor struct{}

func (e *YouTubeExtractor) Name() string {
	return "youtube"
}

func (e *YouTubeExtractor) Match(u *url.URL) bool {
	host := u.Hostname()
	return host == "youtube.com" || host == "www.youtube.com" ||
		host == "youtu.be" || host == "m.youtube.com"
}

func (e *YouTubeExtractor) Extract(url string) (*VideoInfo, error) {
	return nil, fmt.Errorf("YouTube support coming soon")
}

func init() {
	Register(&YouTubeExtractor{})
}
