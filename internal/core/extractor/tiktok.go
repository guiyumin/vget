package extractor

import (
	"fmt"
	"net/url"
)

// TikTokExtractor handles TikTok video downloads
type TikTokExtractor struct{}

func (e *TikTokExtractor) Name() string {
	return "tiktok"
}

func (e *TikTokExtractor) Match(u *url.URL) bool {
	// Host matching is done by registry
	return true
}

func (e *TikTokExtractor) Extract(url string) (Media, error) {
	return nil, fmt.Errorf("TikTok support coming soon")
}

func init() {
	Register(&TikTokExtractor{},
		"tiktok.com",
		"vm.tiktok.com",
	)
}
