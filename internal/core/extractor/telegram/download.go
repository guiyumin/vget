package telegram

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
)

// DownloadResult contains the result of a Telegram download
type DownloadResult struct {
	Title    string
	Filename string
	Size     int64
}

// Download downloads media from a Telegram URL directly.
// This combines extraction and download because Telegram requires
// the download to happen within the authenticated client context.
func Download(urlStr string, outputPath string, progressFn func(downloaded, total int64)) (*DownloadResult, error) {
	if !SessionExists() {
		return nil, fmt.Errorf("not logged in to Telegram. Run 'vget telegram login' first")
	}

	msg, err := ParseURL(urlStr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	storage := &session.FileStorage{Path: SessionFile()}

	client := telegram.NewClient(DesktopAppID, DesktopAppHash, telegram.Options{
		SessionStorage: storage,
	})

	var result *DownloadResult

	err = client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to check auth status: %w", err)
		}
		if !status.Authorized {
			return fmt.Errorf("not authorized. Run 'vget telegram login' first")
		}

		api := client.API()

		// Resolve channel
		inputChannel, err := resolveChannel(ctx, api, msg)
		if err != nil {
			return err
		}

		// Get the message
		msgResult, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: inputChannel,
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msg.MessageID}},
		})
		if err != nil {
			return fmt.Errorf("failed to get message: %w", err)
		}

		// Extract message
		tgMsg, err := extractMessage(msgResult)
		if err != nil {
			return err
		}

		if tgMsg.Media == nil {
			return fmt.Errorf("message has no media")
		}

		// Download based on media type
		dl := downloader.NewDownloader()

		switch media := tgMsg.Media.(type) {
		case *tg.MessageMediaDocument:
			result, err = downloadDocument(ctx, api, dl, media, tgMsg, outputPath, progressFn)
			return err

		case *tg.MessageMediaPhoto:
			result, err = downloadPhoto(ctx, api, dl, media, tgMsg, outputPath, progressFn)
			return err

		default:
			return fmt.Errorf("unsupported media type: %T", media)
		}
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func resolveChannel(ctx context.Context, api *tg.Client, msg *Message) (*tg.InputChannel, error) {
	if msg.IsPrivate {
		channel, err := resolvePrivateChannel(ctx, api, msg.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve private channel: %w", err)
		}
		return &tg.InputChannel{
			ChannelID:  channel.ID,
			AccessHash: channel.AccessHash,
		}, nil
	}

	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: msg.ChannelUsername,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve username '%s': %w", msg.ChannelUsername, err)
	}

	if len(resolved.Chats) > 0 {
		if channel, ok := resolved.Chats[0].(*tg.Channel); ok {
			return &tg.InputChannel{
				ChannelID:  channel.ID,
				AccessHash: channel.AccessHash,
			}, nil
		}
	}

	return nil, fmt.Errorf("could not resolve '%s' to a channel", msg.ChannelUsername)
}

// ChannelInfo holds basic channel information for display
type ChannelInfo struct {
	ID         int64
	AccessHash int64
	Title      string
	Username   string
}

func resolvePrivateChannel(ctx context.Context, api *tg.Client, channelID int64) (*ChannelInfo, error) {
	channels, err := getAllChannels(ctx, api)
	if err != nil {
		return nil, err
	}

	// Look for the target channel by ID
	for _, ch := range channels {
		if ch.ID == channelID {
			return &ch, nil
		}
	}

	return nil, fmt.Errorf("channel not found. Make sure you've joined the channel")
}

func getAllChannels(ctx context.Context, api *tg.Client) ([]ChannelInfo, error) {
	var channels []ChannelInfo
	var offsetDate int
	var offsetID int
	var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}

	for {
		dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: offsetPeer,
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
			Limit:      100,
		})
		if err != nil {
			return channels, err
		}

		var chats []tg.ChatClass
		var messages []tg.MessageClass
		var done bool

		switch d := dialogs.(type) {
		case *tg.MessagesDialogs:
			chats = d.Chats
			messages = d.Messages
			done = true
		case *tg.MessagesDialogsSlice:
			chats = d.Chats
			messages = d.Messages
			done = len(d.Dialogs) < 100
		default:
			done = true
		}

		for _, chat := range chats {
			if channel, ok := chat.(*tg.Channel); ok {
				channels = append(channels, ChannelInfo{
					ID:         channel.ID,
					AccessHash: channel.AccessHash,
					Title:      channel.Title,
					Username:   channel.Username,
				})
			}
		}

		if done || len(messages) == 0 {
			break
		}

		lastMsg := messages[len(messages)-1]
		if msg, ok := lastMsg.(*tg.Message); ok {
			offsetDate = msg.Date
			offsetID = msg.ID
			if len(channels) > 0 {
				offsetPeer = &tg.InputPeerChannel{
					ChannelID:  channels[len(channels)-1].ID,
					AccessHash: channels[len(channels)-1].AccessHash,
				}
			}
		} else {
			break
		}
	}

	return channels, nil
}

func extractMessage(result tg.MessagesMessagesClass) (*tg.Message, error) {
	var messages []tg.MessageClass

	switch r := result.(type) {
	case *tg.MessagesChannelMessages:
		messages = r.Messages
	default:
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	msg, ok := messages[0].(*tg.Message)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: %T", messages[0])
	}

	return msg, nil
}

func downloadDocument(
	ctx context.Context,
	api *tg.Client,
	dl *downloader.Downloader,
	media *tg.MessageMediaDocument,
	msg *tg.Message,
	outputPath string,
	progressFn func(downloaded, total int64),
) (*DownloadResult, error) {
	doc, ok := media.Document.(*tg.Document)
	if !ok {
		return nil, fmt.Errorf("invalid document")
	}

	info := ExtractDocumentInfo(doc, msg.Message, msg.ID)

	// Determine output filename
	outFile := outputPath
	if outFile == "" {
		if info.Filename != "" {
			outFile = info.Filename
		} else {
			outFile = fmt.Sprintf("%s.%s", sanitizeFilename(info.Title), info.Ext)
		}
	}

	// Create output file
	f, err := os.Create(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	// Download with progress
	var writer io.Writer = f
	if progressFn != nil {
		writer = &progressWriter{w: f, fn: progressFn, total: doc.Size}
	}

	_, err = dl.Download(api, &tg.InputDocumentFileLocation{
		ID:            doc.ID,
		AccessHash:    doc.AccessHash,
		FileReference: doc.FileReference,
	}).Stream(ctx, writer)
	if err != nil {
		os.Remove(outFile)
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return &DownloadResult{
		Title:    info.Title,
		Filename: outFile,
		Size:     doc.Size,
	}, nil
}

func downloadPhoto(
	ctx context.Context,
	api *tg.Client,
	dl *downloader.Downloader,
	media *tg.MessageMediaPhoto,
	msg *tg.Message,
	outputPath string,
	progressFn func(downloaded, total int64),
) (*DownloadResult, error) {
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

	outFile := outputPath
	if outFile == "" {
		outFile = fmt.Sprintf("%s.jpg", sanitizeFilename(title))
	}

	f, err := os.Create(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	_, err = dl.Download(api, &tg.InputPhotoFileLocation{
		ID:            photo.ID,
		AccessHash:    photo.AccessHash,
		FileReference: photo.FileReference,
		ThumbSize:     largest.Type,
	}).Stream(ctx, f)
	if err != nil {
		os.Remove(outFile)
		return nil, fmt.Errorf("download failed: %w", err)
	}

	return &DownloadResult{
		Title:    title,
		Filename: outFile,
		Size:     int64(largest.Size),
	}, nil
}

// progressWriter wraps an io.Writer to report progress
type progressWriter struct {
	w          io.Writer
	fn         func(downloaded, total int64)
	total      int64
	downloaded int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	pw.downloaded += int64(n)
	if pw.fn != nil {
		pw.fn(pw.downloaded, pw.total)
	}
	return n, err
}

func sanitizeFilename(name string) string {
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

	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	result = urlRegex.ReplaceAllString(result, "")

	result = strings.TrimSpace(result)
	result = strings.Trim(result, ".")

	spaceRegex := regexp.MustCompile(`\s+`)
	result = spaceRegex.ReplaceAllString(result, " ")

	runes := []rune(result)
	if len(runes) > 200 {
		result = string(runes[:200])
	}

	return strings.TrimSpace(result)
}
