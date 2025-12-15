package torrent

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// TransmissionClient implements the Client interface for Transmission daemon
// Transmission RPC specification: https://github.com/transmission/transmission/blob/main/docs/rpc-spec.md
type TransmissionClient struct {
	config    *Config
	client    *http.Client
	baseURL   string
	sessionID string // X-Transmission-Session-Id for CSRF protection
}

// Transmission RPC request/response structures
type trRequest struct {
	Method    string      `json:"method"`
	Arguments interface{} `json:"arguments,omitempty"`
	Tag       int         `json:"tag,omitempty"`
}

type trResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Tag       int             `json:"tag,omitempty"`
}

// Transmission torrent status codes
const (
	trStatusStopped      = 0
	trStatusQueuedVerify = 1
	trStatusVerifying    = 2
	trStatusQueuedDown   = 3
	trStatusDownloading  = 4
	trStatusQueuedSeed   = 5
	trStatusSeeding      = 6
)

// NewTransmissionClient creates a new Transmission client
func NewTransmissionClient(cfg *Config) *TransmissionClient {
	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	return &TransmissionClient{
		config:  cfg,
		baseURL: fmt.Sprintf("%s://%s/transmission/rpc", scheme, cfg.Host),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TransmissionClient) Name() string {
	return "transmission"
}

// Connect establishes connection and gets session ID
func (c *TransmissionClient) Connect() error {
	// Transmission uses CSRF protection via X-Transmission-Session-Id
	// Make a dummy request to get the session ID
	_, err := c.doRequest("session-get", nil)
	if err != nil && c.sessionID == "" {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	return nil
}

func (c *TransmissionClient) Close() error {
	// Transmission doesn't have explicit logout
	c.sessionID = ""
	return nil
}

// doRequest performs an RPC request with automatic session ID handling
func (c *TransmissionClient) doRequest(method string, args interface{}) (*trResponse, error) {
	req := trRequest{
		Method:    method,
		Arguments: args,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Try up to 2 times (for session ID refresh)
	for i := 0; i < 2; i++ {
		httpReq, err := http.NewRequest("POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if c.sessionID != "" {
			httpReq.Header.Set("X-Transmission-Session-Id", c.sessionID)
		}

		// Add basic auth if configured
		if c.config.Username != "" {
			httpReq.SetBasicAuth(c.config.Username, c.config.Password)
		}

		resp, err := c.client.Do(httpReq)
		if err != nil {
			return nil, err
		}

		// Handle 409 Conflict - need to update session ID
		if resp.StatusCode == http.StatusConflict {
			newSessionID := resp.Header.Get("X-Transmission-Session-Id")
			resp.Body.Close()
			if newSessionID != "" {
				c.sessionID = newSessionID
				continue // Retry with new session ID
			}
			return nil, fmt.Errorf("received 409 but no session ID in response")
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuthFailed
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var trResp trResponse
		if err := json.Unmarshal(respBody, &trResp); err != nil {
			return nil, err
		}

		if trResp.Result != "success" {
			return nil, fmt.Errorf("transmission error: %s", trResp.Result)
		}

		return &trResp, nil
	}

	return nil, fmt.Errorf("failed to get valid session ID")
}

// AddMagnet adds a torrent via magnet link
func (c *TransmissionClient) AddMagnet(magnetURL string, opts *AddOptions) (*AddResult, error) {
	if !IsMagnetLink(magnetURL) {
		return nil, ErrInvalidMagnet
	}

	args := map[string]interface{}{
		"filename": magnetURL,
	}

	if opts != nil {
		if opts.SavePath != "" {
			args["download-dir"] = opts.SavePath
		}
		if opts.Paused {
			args["paused"] = true
		}
	}

	return c.addTorrent(args)
}

// AddTorrentURL adds a torrent via HTTP/HTTPS URL
func (c *TransmissionClient) AddTorrentURL(url string, opts *AddOptions) (*AddResult, error) {
	args := map[string]interface{}{
		"filename": url,
	}

	if opts != nil {
		if opts.SavePath != "" {
			args["download-dir"] = opts.SavePath
		}
		if opts.Paused {
			args["paused"] = true
		}
	}

	return c.addTorrent(args)
}

// AddTorrentFile adds a torrent from a local .torrent file
func (c *TransmissionClient) AddTorrentFile(path string, opts *AddOptions) (*AddResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}

	args := map[string]interface{}{
		"metainfo": base64.StdEncoding.EncodeToString(data),
	}

	if opts != nil {
		if opts.SavePath != "" {
			args["download-dir"] = opts.SavePath
		}
		if opts.Paused {
			args["paused"] = true
		}
	}

	return c.addTorrent(args)
}

func (c *TransmissionClient) addTorrent(args map[string]interface{}) (*AddResult, error) {
	resp, err := c.doRequest("torrent-add", args)
	if err != nil {
		return nil, err
	}

	var result struct {
		TorrentAdded     *trTorrentAdded `json:"torrent-added"`
		TorrentDuplicate *trTorrentAdded `json:"torrent-duplicate"`
	}

	if err := json.Unmarshal(resp.Arguments, &result); err != nil {
		return nil, err
	}

	if result.TorrentDuplicate != nil {
		return &AddResult{
			ID:        fmt.Sprintf("%d", result.TorrentDuplicate.ID),
			Hash:      result.TorrentDuplicate.HashString,
			Name:      result.TorrentDuplicate.Name,
			Duplicate: true,
		}, nil
	}

	if result.TorrentAdded != nil {
		return &AddResult{
			ID:        fmt.Sprintf("%d", result.TorrentAdded.ID),
			Hash:      result.TorrentAdded.HashString,
			Name:      result.TorrentAdded.Name,
			Duplicate: false,
		}, nil
	}

	return nil, fmt.Errorf("unexpected response: no torrent info returned")
}

type trTorrentAdded struct {
	ID         int    `json:"id"`
	HashString string `json:"hashString"`
	Name       string `json:"name"`
}

// GetTorrent retrieves info about a specific torrent
func (c *TransmissionClient) GetTorrent(id string) (*TorrentInfo, error) {
	args := map[string]interface{}{
		"ids":    []string{id},
		"fields": trTorrentFields,
	}

	resp, err := c.doRequest("torrent-get", args)
	if err != nil {
		return nil, err
	}

	var result struct {
		Torrents []trTorrent `json:"torrents"`
	}

	if err := json.Unmarshal(resp.Arguments, &result); err != nil {
		return nil, err
	}

	if len(result.Torrents) == 0 {
		return nil, ErrTorrentNotFound
	}

	return c.convertTorrent(&result.Torrents[0]), nil
}

// ListTorrents retrieves info about all torrents
func (c *TransmissionClient) ListTorrents() ([]TorrentInfo, error) {
	args := map[string]interface{}{
		"fields": trTorrentFields,
	}

	resp, err := c.doRequest("torrent-get", args)
	if err != nil {
		return nil, err
	}

	var result struct {
		Torrents []trTorrent `json:"torrents"`
	}

	if err := json.Unmarshal(resp.Arguments, &result); err != nil {
		return nil, err
	}

	torrents := make([]TorrentInfo, len(result.Torrents))
	for i, t := range result.Torrents {
		torrents[i] = *c.convertTorrent(&t)
	}

	return torrents, nil
}

var trTorrentFields = []string{
	"id", "hashString", "name", "status", "percentDone",
	"totalSize", "downloadedEver", "uploadedEver",
	"rateDownload", "rateUpload", "uploadRatio",
	"eta", "downloadDir", "errorString", "error",
}

type trTorrent struct {
	ID             int     `json:"id"`
	HashString     string  `json:"hashString"`
	Name           string  `json:"name"`
	Status         int     `json:"status"`
	PercentDone    float64 `json:"percentDone"`
	TotalSize      int64   `json:"totalSize"`
	DownloadedEver int64   `json:"downloadedEver"`
	UploadedEver   int64   `json:"uploadedEver"`
	RateDownload   int64   `json:"rateDownload"`
	RateUpload     int64   `json:"rateUpload"`
	UploadRatio    float64 `json:"uploadRatio"`
	ETA            int64   `json:"eta"`
	DownloadDir    string  `json:"downloadDir"`
	ErrorString    string  `json:"errorString"`
	Error          int     `json:"error"`
}

func (c *TransmissionClient) convertTorrent(t *trTorrent) *TorrentInfo {
	state := c.convertStatus(t.Status)
	if t.Error != 0 {
		state = StateError
	}

	return &TorrentInfo{
		ID:            fmt.Sprintf("%d", t.ID),
		Hash:          t.HashString,
		Name:          t.Name,
		State:         state,
		Progress:      t.PercentDone,
		Size:          t.TotalSize,
		Downloaded:    t.DownloadedEver,
		Uploaded:      t.UploadedEver,
		DownloadSpeed: t.RateDownload,
		UploadSpeed:   t.RateUpload,
		Ratio:         t.UploadRatio,
		ETA:           t.ETA,
		SavePath:      t.DownloadDir,
		Error:         t.ErrorString,
	}
}

func (c *TransmissionClient) convertStatus(status int) TorrentState {
	switch status {
	case trStatusStopped:
		return StateStopped
	case trStatusQueuedVerify, trStatusVerifying:
		return StateChecking
	case trStatusQueuedDown:
		return StateQueued
	case trStatusDownloading:
		return StateDownloading
	case trStatusQueuedSeed, trStatusSeeding:
		return StateSeeding
	default:
		return StateUnknown
	}
}

// RemoveTorrent removes a torrent
func (c *TransmissionClient) RemoveTorrent(id string, deleteData bool) error {
	args := map[string]interface{}{
		"ids":               []string{id},
		"delete-local-data": deleteData,
	}

	_, err := c.doRequest("torrent-remove", args)
	return err
}

// PauseTorrent pauses a torrent
func (c *TransmissionClient) PauseTorrent(id string) error {
	args := map[string]interface{}{
		"ids": []string{id},
	}

	_, err := c.doRequest("torrent-stop", args)
	return err
}

// ResumeTorrent resumes a paused torrent
func (c *TransmissionClient) ResumeTorrent(id string) error {
	args := map[string]interface{}{
		"ids": []string{id},
	}

	_, err := c.doRequest("torrent-start", args)
	return err
}

// Helper to check if string looks like a hash (for ID vs hash detection)
func isHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range strings.ToLower(s) {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
