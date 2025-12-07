package telegram

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	// t.me/channel/123 or t.me/username/123
	publicURLRegex = regexp.MustCompile(`t\.me/([^/]+)/(\d+)`)
	// t.me/c/123456789/123 (private channel)
	privateURLRegex = regexp.MustCompile(`t\.me/c/(\d+)/(\d+)`)
)

// Message represents a parsed Telegram message URL
type Message struct {
	ChannelUsername string // For public channels/users
	ChannelID       int64  // For private channels (from /c/ URLs)
	MessageID       int
	IsPrivate       bool
}

// ParseURL parses a t.me URL into its components
func ParseURL(urlStr string) (*Message, error) {
	// Try private channel format first: t.me/c/123456789/123
	if matches := privateURLRegex.FindStringSubmatch(urlStr); len(matches) >= 3 {
		channelID, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid channel ID: %w", err)
		}
		msgID, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("invalid message ID: %w", err)
		}
		return &Message{
			ChannelID: channelID,
			MessageID: msgID,
			IsPrivate: true,
		}, nil
	}

	// Try public format: t.me/channel/123
	if matches := publicURLRegex.FindStringSubmatch(urlStr); len(matches) >= 3 {
		msgID, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("invalid message ID: %w", err)
		}
		return &Message{
			ChannelUsername: matches[1],
			MessageID:       msgID,
			IsPrivate:       false,
		}, nil
	}

	return nil, fmt.Errorf("could not parse Telegram URL: %s", urlStr)
}

// MatchURL checks if a URL string matches Telegram message patterns
func MatchURL(urlStr string) bool {
	return publicURLRegex.MatchString(urlStr) || privateURLRegex.MatchString(urlStr)
}
