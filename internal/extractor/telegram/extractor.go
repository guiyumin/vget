package telegram

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// Extractor handles Telegram media extraction
type Extractor struct{}

// Name returns the extractor name
func (e *Extractor) Name() string {
	return "telegram"
}

// Match checks if URL is a Telegram message URL
func (e *Extractor) Match(u *url.URL) bool {
	host := strings.ToLower(u.Host)
	if host != "t.me" && host != "telegram.me" {
		return false
	}
	return MatchURL(u.String())
}

// MediaInfo contains extracted media information for the extractor interface
type MediaInfo struct {
	ID       string
	Title    string
	Uploader string
	URL      string
	Ext      string
	Width    int
	Height   int
	Size     int64
	IsVideo  bool
	IsAudio  bool
	IsPhoto  bool
}

// Extract retrieves media info from a Telegram URL
func (e *Extractor) Extract(urlStr string) (*MediaInfo, error) {
	if !SessionExists() {
		return nil, fmt.Errorf("not logged in to Telegram. Run 'vget telegram login' first")
	}

	msg, err := ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	return e.extractMedia(ctx, msg)
}

func (e *Extractor) extractMedia(ctx context.Context, msg *Message) (*MediaInfo, error) {
	storage := &session.FileStorage{Path: SessionFile()}

	client := telegram.NewClient(DesktopAppID, DesktopAppHash, telegram.Options{
		SessionStorage: storage,
	})

	var result *MediaInfo

	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to check auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("not authorized. Run 'vget telegram login' first")
		}

		api := client.API()

		inputChannel, err := resolveChannel(ctx, api, msg)
		if err != nil {
			return err
		}

		msgResult, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: inputChannel,
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msg.MessageID}},
		})
		if err != nil {
			return fmt.Errorf("failed to get channel message: %w", err)
		}

		tgMsg, err := extractMessage(msgResult)
		if err != nil {
			return err
		}

		if tgMsg.Media == nil {
			return fmt.Errorf("message has no media")
		}

		result, err = e.extractMediaInfo(tgMsg)
		return err
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (e *Extractor) extractMediaInfo(msg *tg.Message) (*MediaInfo, error) {
	switch media := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return nil, fmt.Errorf("invalid document")
		}

		info := ExtractDocumentInfo(doc, msg.Message, msg.ID)

		return &MediaInfo{
			ID:       fmt.Sprintf("%d", msg.ID),
			Title:    info.Title,
			Uploader: "Telegram",
			URL:      fmt.Sprintf("tg://document/%d_%d", doc.ID, doc.AccessHash),
			Ext:      info.Ext,
			Width:    info.Width,
			Height:   info.Height,
			Size:     info.Size,
			IsVideo:  info.IsVideo,
			IsAudio:  info.IsAudio,
		}, nil

	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return nil, fmt.Errorf("invalid photo")
		}

		largest := FindLargestPhotoSize(photo.Sizes)
		if largest == nil {
			return nil, fmt.Errorf("no photo sizes available")
		}

		title := truncateText(msg.Message, 100)
		if title == "" {
			title = fmt.Sprintf("telegram_%d", msg.ID)
		}

		return &MediaInfo{
			ID:       fmt.Sprintf("%d", msg.ID),
			Title:    title,
			Uploader: "Telegram",
			URL:      fmt.Sprintf("tg://photo/%d_%d", photo.ID, photo.AccessHash),
			Ext:      "jpg",
			Width:    largest.W,
			Height:   largest.H,
			Size:     int64(largest.Size),
			IsPhoto:  true,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported media type: %T", media)
	}
}
