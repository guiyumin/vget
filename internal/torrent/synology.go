package torrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SynologyClient implements the Client interface for Synology Download Station
// Synology Download Station API: https://global.download.synology.com/download/Document/Software/DeveloperGuide/Package/DownloadStation/All/enu/Synology_Download_Station_Web_API.pdf
type SynologyClient struct {
	config  *Config
	client  *http.Client
	baseURL string
	sid     string // Session ID
}

// Synology API response structure
type synResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *synError       `json:"error,omitempty"`
}

type synError struct {
	Code int `json:"code"`
}

// Synology error codes (common)
const (
	synErrUnknown           = 100
	synErrInvalidParam      = 101
	synErrAPINotExists      = 102
	synErrMethodNotExists   = 103
	synErrVersionNotSupport = 104
	synErrPermDenied        = 105
	synErrTimeout           = 106
	synErrDuplicate         = 107
)

// Note: Synology uses overlapping error codes across APIs (Auth vs DownloadStation)
// Error codes 400-405 have different meanings depending on the API being called.
// We handle them numerically in convertError() to avoid constant duplication.

// Synology task status
const (
	synStatusWaiting     = "waiting"
	synStatusDownloading = "downloading"
	synStatusPaused      = "paused"
	synStatusFinishing   = "finishing"
	synStatusFinished    = "finished"
	synStatusHashChecking = "hash_checking"
	synStatusSeeding     = "seeding"
	synStatusFileHosting = "filehosting_waiting"
	synStatusExtracting  = "extracting"
	synStatusError       = "error"
)

// NewSynologyClient creates a new Synology Download Station client
func NewSynologyClient(cfg *Config) *SynologyClient {
	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	// Synology uses port 5000 for HTTP, 5001 for HTTPS by default
	host := cfg.Host
	if !strings.Contains(host, ":") {
		if cfg.UseHTTPS {
			host += ":5001"
		} else {
			host += ":5000"
		}
	}

	return &SynologyClient{
		config:  cfg,
		baseURL: fmt.Sprintf("%s://%s", scheme, host),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SynologyClient) Name() string {
	return "synology"
}

// Connect authenticates with Synology DSM
func (c *SynologyClient) Connect() error {
	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "3")
	params.Set("method", "login")
	params.Set("account", c.config.Username)
	params.Set("passwd", c.config.Password)
	params.Set("session", "DownloadStation")
	params.Set("format", "sid")

	resp, err := c.doRequest("/webapi/auth.cgi", params, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	var data struct {
		Sid string `json:"sid"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return fmt.Errorf("failed to parse auth response: %w", err)
	}

	c.sid = data.Sid
	return nil
}

// Close logs out from Synology DSM
func (c *SynologyClient) Close() error {
	if c.sid == "" {
		return nil
	}

	params := url.Values{}
	params.Set("api", "SYNO.API.Auth")
	params.Set("version", "1")
	params.Set("method", "logout")
	params.Set("session", "DownloadStation")
	params.Set("_sid", c.sid)

	c.client.Get(c.baseURL + "/webapi/auth.cgi?" + params.Encode())
	c.sid = ""
	return nil
}

// doRequest performs an API request
func (c *SynologyClient) doRequest(path string, params url.Values, body io.Reader) (*synResponse, error) {
	// Add session ID to all requests
	if c.sid != "" && params.Get("_sid") == "" {
		params.Set("_sid", c.sid)
	}

	var resp *http.Response
	var err error

	if body != nil {
		// POST request with body
		req, err := http.NewRequest("POST", c.baseURL+path+"?"+params.Encode(), body)
		if err != nil {
			return nil, err
		}
		if mw, ok := body.(*bytes.Buffer); ok {
			_ = mw // For multipart, content-type is set separately
		}
		resp, err = c.client.Do(req)
		if err != nil {
			return nil, err
		}
	} else {
		// GET request
		resp, err = c.client.Get(c.baseURL + path + "?" + params.Encode())
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	var synResp synResponse
	if err := json.NewDecoder(resp.Body).Decode(&synResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !synResp.Success {
		return nil, c.convertError(synResp.Error)
	}

	return &synResp, nil
}

func (c *SynologyClient) convertError(err *synError) error {
	if err == nil {
		return fmt.Errorf("unknown error")
	}

	// Synology uses overlapping error codes across different APIs
	// Handle common codes first, then specific ranges
	switch err.Code {
	case synErrDuplicate:
		return ErrDuplicateTorrent
	case synErrInvalidParam:
		return fmt.Errorf("invalid parameter")
	case synErrPermDenied:
		return fmt.Errorf("permission denied")
	case 400: // Auth failed or DS file upload failed
		return ErrAuthFailed
	case 401: // Auth no permission or DS max tasks
		return fmt.Errorf("permission denied or maximum tasks reached")
	case 402: // Auth account locked or DS dest denied
		return fmt.Errorf("account locked or destination denied")
	case 403: // DS destination not exist
		return fmt.Errorf("destination does not exist")
	case 405: // DS invalid task ID
		return ErrTorrentNotFound
	default:
		return fmt.Errorf("synology error code: %d", err.Code)
	}
}

// AddMagnet adds a torrent via magnet link
func (c *SynologyClient) AddMagnet(magnetURL string, opts *AddOptions) (*AddResult, error) {
	if !IsMagnetLink(magnetURL) {
		return nil, ErrInvalidMagnet
	}

	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "3")
	params.Set("method", "create")
	params.Set("uri", magnetURL)

	if opts != nil && opts.SavePath != "" {
		params.Set("destination", opts.SavePath)
	}

	_, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	if err != nil {
		return nil, err
	}

	// Synology doesn't return task ID on create, extract hash from magnet
	result := &AddResult{}
	if hash := extractHashFromMagnet(magnetURL); hash != "" {
		result.Hash = hash
		result.ID = hash
	}

	return result, nil
}

// AddTorrentURL adds a torrent via HTTP/HTTPS URL
func (c *SynologyClient) AddTorrentURL(torrentURL string, opts *AddOptions) (*AddResult, error) {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "3")
	params.Set("method", "create")
	params.Set("uri", torrentURL)

	if opts != nil && opts.SavePath != "" {
		params.Set("destination", opts.SavePath)
	}

	_, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	if err != nil {
		return nil, err
	}

	return &AddResult{}, nil
}

// AddTorrentFile adds a torrent from a local .torrent file
func (c *SynologyClient) AddTorrentFile(path string, opts *AddOptions) (*AddResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add torrent file
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}

	// Add API parameters as form fields
	writer.WriteField("api", "SYNO.DownloadStation.Task")
	writer.WriteField("version", "3")
	writer.WriteField("method", "create")

	if opts != nil && opts.SavePath != "" {
		writer.WriteField("destination", opts.SavePath)
	}

	writer.Close()

	params := url.Values{}
	params.Set("_sid", c.sid)

	req, err := http.NewRequest("POST", c.baseURL+"/webapi/DownloadStation/task.cgi?"+params.Encode(), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var synResp synResponse
	if err := json.NewDecoder(resp.Body).Decode(&synResp); err != nil {
		return nil, err
	}

	if !synResp.Success {
		return nil, c.convertError(synResp.Error)
	}

	return &AddResult{}, nil
}

// GetTorrent retrieves info about a specific torrent by ID
func (c *SynologyClient) GetTorrent(id string) (*TorrentInfo, error) {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "1")
	params.Set("method", "getinfo")
	params.Set("id", id)
	params.Set("additional", "detail,transfer")

	resp, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		Tasks []synTask `json:"tasks"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, err
	}

	if len(data.Tasks) == 0 {
		return nil, ErrTorrentNotFound
	}

	return c.convertTask(&data.Tasks[0]), nil
}

// ListTorrents retrieves info about all torrents
func (c *SynologyClient) ListTorrents() ([]TorrentInfo, error) {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "1")
	params.Set("method", "list")
	params.Set("additional", "detail,transfer")

	resp, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	if err != nil {
		return nil, err
	}

	var data struct {
		Tasks  []synTask `json:"tasks"`
		Total  int       `json:"total"`
		Offset int       `json:"offset"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, err
	}

	torrents := make([]TorrentInfo, 0, len(data.Tasks))
	for _, t := range data.Tasks {
		// Only include BT tasks (filter out HTTP downloads, etc.)
		if t.Type == "bt" {
			torrents = append(torrents, *c.convertTask(&t))
		}
	}

	return torrents, nil
}

type synTask struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"` // bt, http, ftp, etc.
	Title      string        `json:"title"`
	Size       int64         `json:"size"`
	Status     string        `json:"status"`
	Additional synAdditional `json:"additional"`
}

type synAdditional struct {
	Detail   synDetail   `json:"detail"`
	Transfer synTransfer `json:"transfer"`
}

type synDetail struct {
	Destination       string `json:"destination"`
	URI               string `json:"uri"`
	CreateTime        int64  `json:"create_time"`
	CompletedTime     int64  `json:"completed_time"`
	TotalPeers        int    `json:"total_peers"`
	ConnectedSeeders  int    `json:"connected_seeders"`
	ConnectedLeechers int    `json:"connected_leechers"`
}

type synTransfer struct {
	SizeDownloaded   int64 `json:"size_downloaded"`
	SizeUploaded     int64 `json:"size_uploaded"`
	SpeedDownload    int64 `json:"speed_download"`
	SpeedUpload      int64 `json:"speed_upload"`
}

func (c *SynologyClient) convertTask(t *synTask) *TorrentInfo {
	var progress float64
	if t.Size > 0 {
		progress = float64(t.Additional.Transfer.SizeDownloaded) / float64(t.Size)
	}

	var ratio float64
	if t.Additional.Transfer.SizeDownloaded > 0 {
		ratio = float64(t.Additional.Transfer.SizeUploaded) / float64(t.Additional.Transfer.SizeDownloaded)
	}

	// Estimate ETA
	var eta int64 = -1
	if t.Additional.Transfer.SpeedDownload > 0 {
		remaining := t.Size - t.Additional.Transfer.SizeDownloaded
		eta = remaining / t.Additional.Transfer.SpeedDownload
	}

	return &TorrentInfo{
		ID:            t.ID,
		Hash:          "", // Synology doesn't expose hash directly in task list
		Name:          t.Title,
		State:         c.convertStatus(t.Status),
		Progress:      progress,
		Size:          t.Size,
		Downloaded:    t.Additional.Transfer.SizeDownloaded,
		Uploaded:      t.Additional.Transfer.SizeUploaded,
		DownloadSpeed: t.Additional.Transfer.SpeedDownload,
		UploadSpeed:   t.Additional.Transfer.SpeedUpload,
		Ratio:         ratio,
		ETA:           eta,
		SavePath:      t.Additional.Detail.Destination,
	}
}

func (c *SynologyClient) convertStatus(status string) TorrentState {
	switch status {
	case synStatusWaiting:
		return StateQueued
	case synStatusDownloading:
		return StateDownloading
	case synStatusPaused:
		return StatePaused
	case synStatusFinishing, synStatusFinished:
		return StateStopped
	case synStatusHashChecking:
		return StateChecking
	case synStatusSeeding:
		return StateSeeding
	case synStatusFileHosting, synStatusExtracting:
		return StateDownloading
	case synStatusError:
		return StateError
	default:
		return StateUnknown
	}
}

// RemoveTorrent removes a torrent
func (c *SynologyClient) RemoveTorrent(id string, deleteData bool) error {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "1")
	params.Set("method", "delete")
	params.Set("id", id)
	params.Set("force_complete", strconv.FormatBool(deleteData))

	_, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	return err
}

// PauseTorrent pauses a torrent
func (c *SynologyClient) PauseTorrent(id string) error {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "1")
	params.Set("method", "pause")
	params.Set("id", id)

	_, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	return err
}

// ResumeTorrent resumes a paused torrent
func (c *SynologyClient) ResumeTorrent(id string) error {
	params := url.Values{}
	params.Set("api", "SYNO.DownloadStation.Task")
	params.Set("version", "1")
	params.Set("method", "resume")
	params.Set("id", id)

	_, err := c.doRequest("/webapi/DownloadStation/task.cgi", params, nil)
	return err
}
