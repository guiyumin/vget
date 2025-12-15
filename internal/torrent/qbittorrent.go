package torrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// QBittorrentClient implements the Client interface for qBittorrent Web UI
// qBittorrent Web API: https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)
type QBittorrentClient struct {
	config     *Config
	client     *http.Client
	baseURL    string
	apiVersion string
}

// qBittorrent torrent states
const (
	qbStateError              = "error"
	qbStateMissingFiles       = "missingFiles"
	qbStateUploading          = "uploading"
	qbStatePausedUP           = "pausedUP"
	qbStateQueuedUP           = "queuedUP"
	qbStateStalledUP          = "stalledUP"
	qbStateCheckingUP         = "checkingUP"
	qbStateForcedUP           = "forcedUP"
	qbStateAllocating         = "allocating"
	qbStateDownloading        = "downloading"
	qbStateMetaDL             = "metaDL"
	qbStatePausedDL           = "pausedDL"
	qbStateQueuedDL           = "queuedDL"
	qbStateStalledDL          = "stalledDL"
	qbStateCheckingDL         = "checkingDL"
	qbStateForcedDL           = "forcedDL"
	qbStateCheckingResumeData = "checkingResumeData"
	qbStateMoving             = "moving"
	qbStateUnknown            = "unknown"
)

// NewQBittorrentClient creates a new qBittorrent client
func NewQBittorrentClient(cfg *Config) *QBittorrentClient {
	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	// Create cookie jar for session management
	jar, _ := cookiejar.New(nil)

	return &QBittorrentClient{
		config:  cfg,
		baseURL: fmt.Sprintf("%s://%s", scheme, cfg.Host),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *QBittorrentClient) Name() string {
	return "qbittorrent"
}

// Connect authenticates with qBittorrent
func (c *QBittorrentClient) Connect() error {
	// Login endpoint
	loginURL := c.baseURL + "/api/v2/auth/login"

	data := url.Values{}
	data.Set("username", c.config.Username)
	data.Set("password", c.config.Password)

	resp, err := c.client.PostForm(loginURL, data)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden {
		return ErrAuthFailed
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	// qBittorrent returns "Ok." on success, "Fails." on failure
	if strings.TrimSpace(string(body)) == "Fails." {
		return ErrAuthFailed
	}

	// Get API version for compatibility checks
	c.getAPIVersion()

	return nil
}

func (c *QBittorrentClient) getAPIVersion() {
	resp, err := c.client.Get(c.baseURL + "/api/v2/app/webapiVersion")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.apiVersion = strings.TrimSpace(string(body))
}

// Close logs out from qBittorrent
func (c *QBittorrentClient) Close() error {
	resp, err := c.client.Post(c.baseURL+"/api/v2/auth/logout", "", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// AddMagnet adds a torrent via magnet link
func (c *QBittorrentClient) AddMagnet(magnetURL string, opts *AddOptions) (*AddResult, error) {
	if !IsMagnetLink(magnetURL) {
		return nil, ErrInvalidMagnet
	}

	return c.addTorrent(magnetURL, nil, opts)
}

// AddTorrentURL adds a torrent via HTTP/HTTPS URL
func (c *QBittorrentClient) AddTorrentURL(torrentURL string, opts *AddOptions) (*AddResult, error) {
	return c.addTorrent(torrentURL, nil, opts)
}

// AddTorrentFile adds a torrent from a local .torrent file
func (c *QBittorrentClient) AddTorrentFile(path string, opts *AddOptions) (*AddResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}

	return c.addTorrent("", &torrentFile{
		name: filepath.Base(path),
		data: data,
	}, opts)
}

type torrentFile struct {
	name string
	data []byte
}

func (c *QBittorrentClient) addTorrent(urls string, file *torrentFile, opts *AddOptions) (*AddResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add URLs (magnet or http)
	if urls != "" {
		if err := writer.WriteField("urls", urls); err != nil {
			return nil, err
		}
	}

	// Add torrent file
	if file != nil {
		part, err := writer.CreateFormFile("torrents", file.name)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file.data); err != nil {
			return nil, err
		}
	}

	// Add options
	if opts != nil {
		if opts.SavePath != "" {
			writer.WriteField("savepath", opts.SavePath)
		}
		if opts.Paused {
			writer.WriteField("paused", "true")
		}
		if opts.Category != "" {
			writer.WriteField("category", opts.Category)
		}
		if len(opts.Labels) > 0 {
			writer.WriteField("tags", strings.Join(opts.Labels, ","))
		}
		if opts.DownloadSpeedLimit > 0 {
			writer.WriteField("dlLimit", fmt.Sprintf("%d", opts.DownloadSpeedLimit))
		}
		if opts.UploadSpeedLimit > 0 {
			writer.WriteField("upLimit", fmt.Sprintf("%d", opts.UploadSpeedLimit))
		}
	}

	writer.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/torrents/add", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to add torrent: %s", string(respBody))
	}

	// qBittorrent returns "Ok." on success
	if strings.TrimSpace(string(respBody)) != "Ok." {
		return nil, fmt.Errorf("failed to add torrent: %s", string(respBody))
	}

	// qBittorrent doesn't return the hash on add, we'd need to extract it from magnet
	// or wait and query. For now, return minimal info.
	result := &AddResult{
		Duplicate: false,
	}

	// Try to extract hash from magnet link
	if IsMagnetLink(urls) {
		if hash := extractHashFromMagnet(urls); hash != "" {
			result.Hash = hash
			result.ID = hash
		}
	}

	return result, nil
}

func extractHashFromMagnet(magnet string) string {
	// magnet:?xt=urn:btih:HASH&...
	lower := strings.ToLower(magnet)
	idx := strings.Index(lower, "btih:")
	if idx == -1 {
		return ""
	}

	start := idx + 5
	end := start
	for end < len(magnet) && magnet[end] != '&' {
		end++
	}

	hash := magnet[start:end]
	// Hash can be hex (40 chars) or base32 (32 chars)
	if len(hash) == 40 || len(hash) == 32 {
		return strings.ToLower(hash)
	}
	return ""
}

// GetTorrent retrieves info about a specific torrent by hash
func (c *QBittorrentClient) GetTorrent(id string) (*TorrentInfo, error) {
	torrents, err := c.getTorrents(id)
	if err != nil {
		return nil, err
	}

	if len(torrents) == 0 {
		return nil, ErrTorrentNotFound
	}

	return &torrents[0], nil
}

// ListTorrents retrieves info about all torrents
func (c *QBittorrentClient) ListTorrents() ([]TorrentInfo, error) {
	return c.getTorrents("")
}

func (c *QBittorrentClient) getTorrents(hash string) ([]TorrentInfo, error) {
	url := c.baseURL + "/api/v2/torrents/info"
	if hash != "" {
		url += "?hashes=" + hash
	}

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrents: status %d", resp.StatusCode)
	}

	var qbTorrents []qbTorrent
	if err := json.NewDecoder(resp.Body).Decode(&qbTorrents); err != nil {
		return nil, err
	}

	torrents := make([]TorrentInfo, len(qbTorrents))
	for i, t := range qbTorrents {
		torrents[i] = c.convertTorrent(&t)
	}

	return torrents, nil
}

type qbTorrent struct {
	Hash           string  `json:"hash"`
	Name           string  `json:"name"`
	State          string  `json:"state"`
	Progress       float64 `json:"progress"`
	Size           int64   `json:"size"`
	Downloaded     int64   `json:"downloaded"`
	Uploaded       int64   `json:"uploaded"`
	DlSpeed        int64   `json:"dlspeed"`
	UpSpeed        int64   `json:"upspeed"`
	Ratio          float64 `json:"ratio"`
	ETA            int64   `json:"eta"`
	SavePath       string  `json:"save_path"`
	Category       string  `json:"category"`
	Tags           string  `json:"tags"`
	AddedOn        int64   `json:"added_on"`
	CompletionOn   int64   `json:"completion_on"`
	Tracker        string  `json:"tracker"`
	NumSeeds       int     `json:"num_seeds"`
	NumLeechers    int     `json:"num_leechs"`
	AvailablePeers int     `json:"num_incomplete"`
}

func (c *QBittorrentClient) convertTorrent(t *qbTorrent) TorrentInfo {
	return TorrentInfo{
		ID:            t.Hash,
		Hash:          t.Hash,
		Name:          t.Name,
		State:         c.convertState(t.State),
		Progress:      t.Progress,
		Size:          t.Size,
		Downloaded:    t.Downloaded,
		Uploaded:      t.Uploaded,
		DownloadSpeed: t.DlSpeed,
		UploadSpeed:   t.UpSpeed,
		Ratio:         t.Ratio,
		ETA:           t.ETA,
		SavePath:      t.SavePath,
	}
}

func (c *QBittorrentClient) convertState(state string) TorrentState {
	switch state {
	case qbStateError, qbStateMissingFiles:
		return StateError
	case qbStateUploading, qbStateForcedUP, qbStateStalledUP:
		return StateSeeding
	case qbStatePausedUP, qbStatePausedDL:
		return StatePaused
	case qbStateQueuedUP, qbStateQueuedDL:
		return StateQueued
	case qbStateCheckingUP, qbStateCheckingDL, qbStateCheckingResumeData:
		return StateChecking
	case qbStateDownloading, qbStateForcedDL, qbStateStalledDL, qbStateMetaDL, qbStateAllocating:
		return StateDownloading
	case qbStateMoving:
		return StateDownloading // Close enough
	default:
		return StateUnknown
	}
}

// RemoveTorrent removes a torrent
func (c *QBittorrentClient) RemoveTorrent(id string, deleteData bool) error {
	data := url.Values{}
	data.Set("hashes", id)
	data.Set("deleteFiles", fmt.Sprintf("%t", deleteData))

	resp, err := c.client.PostForm(c.baseURL+"/api/v2/torrents/delete", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove torrent: status %d", resp.StatusCode)
	}

	return nil
}

// PauseTorrent pauses a torrent
func (c *QBittorrentClient) PauseTorrent(id string) error {
	data := url.Values{}
	data.Set("hashes", id)

	resp, err := c.client.PostForm(c.baseURL+"/api/v2/torrents/pause", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pause torrent: status %d", resp.StatusCode)
	}

	return nil
}

// ResumeTorrent resumes a paused torrent
func (c *QBittorrentClient) ResumeTorrent(id string) error {
	data := url.Values{}
	data.Set("hashes", id)

	resp, err := c.client.PostForm(c.baseURL+"/api/v2/torrents/resume", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to resume torrent: status %d", resp.StatusCode)
	}

	return nil
}
