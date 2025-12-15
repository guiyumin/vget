// Package torrent provides integration with various torrent clients for remote download management.
// vget doesn't download torrents directly - it dispatches jobs to existing torrent clients
// running on NAS devices or local machines.
package torrent

import (
	"errors"
	"fmt"
)

// TorrentState represents the current state of a torrent
type TorrentState int

const (
	StateStopped TorrentState = iota
	StateQueued
	StateDownloading
	StateSeeding
	StatePaused
	StateChecking
	StateError
	StateUnknown
)

func (s TorrentState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateQueued:
		return "queued"
	case StateDownloading:
		return "downloading"
	case StateSeeding:
		return "seeding"
	case StatePaused:
		return "paused"
	case StateChecking:
		return "checking"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// TorrentInfo contains information about a torrent
type TorrentInfo struct {
	ID            string       // Client-specific ID (hash or numeric)
	Hash          string       // InfoHash
	Name          string       // Torrent name
	State         TorrentState // Current state
	Progress      float64      // Download progress (0.0 - 1.0)
	Size          int64        // Total size in bytes
	Downloaded    int64        // Downloaded bytes
	Uploaded      int64        // Uploaded bytes
	DownloadSpeed int64        // Current download speed (bytes/sec)
	UploadSpeed   int64        // Current upload speed (bytes/sec)
	Ratio         float64      // Upload/Download ratio
	ETA           int64        // Estimated time remaining (seconds), -1 if unknown
	SavePath      string       // Download location
	Error         string       // Error message if State == StateError
}

// AddOptions contains options for adding a torrent
type AddOptions struct {
	// SavePath overrides the default download directory
	SavePath string

	// Paused starts the torrent in paused state
	Paused bool

	// Labels/Tags to apply (not all clients support this)
	Labels []string

	// Category (qBittorrent specific, but useful abstraction)
	Category string

	// DownloadSpeedLimit in bytes/sec (0 = unlimited)
	DownloadSpeedLimit int64

	// UploadSpeedLimit in bytes/sec (0 = unlimited)
	UploadSpeedLimit int64
}

// AddResult contains the result of adding a torrent
type AddResult struct {
	ID        string // Client-specific ID
	Hash      string // InfoHash
	Name      string // Torrent name (may be empty if magnet hasn't resolved yet)
	Duplicate bool   // True if torrent was already in the client
}

// Client defines the interface for torrent client implementations
type Client interface {
	// Name returns the client name (e.g., "transmission", "qbittorrent")
	Name() string

	// Connect establishes connection and authenticates with the client
	// Should be called before other operations
	Connect() error

	// Close cleans up any resources (e.g., logout)
	Close() error

	// AddMagnet adds a torrent via magnet link
	AddMagnet(magnetURL string, opts *AddOptions) (*AddResult, error)

	// AddTorrentURL adds a torrent via HTTP/HTTPS URL to a .torrent file
	AddTorrentURL(url string, opts *AddOptions) (*AddResult, error)

	// AddTorrentFile adds a torrent from a local .torrent file
	AddTorrentFile(path string, opts *AddOptions) (*AddResult, error)

	// GetTorrent retrieves info about a specific torrent by ID or hash
	GetTorrent(id string) (*TorrentInfo, error)

	// ListTorrents retrieves info about all torrents
	ListTorrents() ([]TorrentInfo, error)

	// RemoveTorrent removes a torrent
	// If deleteData is true, also deletes downloaded files
	RemoveTorrent(id string, deleteData bool) error

	// PauseTorrent pauses a torrent
	PauseTorrent(id string) error

	// ResumeTorrent resumes a paused torrent
	ResumeTorrent(id string) error
}

// ClientType represents supported torrent client types
type ClientType string

const (
	ClientTransmission ClientType = "transmission"
	ClientQBittorrent  ClientType = "qbittorrent"
	ClientSynology     ClientType = "synology"
)

// Config holds configuration for connecting to a torrent client
type Config struct {
	Type     ClientType
	Host     string // hostname:port
	Username string
	Password string
	UseHTTPS bool
}

// Common errors
var (
	ErrNotConnected    = errors.New("not connected to torrent client")
	ErrAuthFailed      = errors.New("authentication failed")
	ErrTorrentNotFound = errors.New("torrent not found")
	ErrDuplicateTorrent = errors.New("torrent already exists")
	ErrInvalidMagnet   = errors.New("invalid magnet link")
	ErrInvalidTorrent  = errors.New("invalid torrent file")
	ErrConnectionFailed = errors.New("failed to connect to torrent client")
)

// NewClient creates a new torrent client based on the config
func NewClient(cfg *Config) (Client, error) {
	switch cfg.Type {
	case ClientTransmission:
		return NewTransmissionClient(cfg), nil
	case ClientQBittorrent:
		return NewQBittorrentClient(cfg), nil
	case ClientSynology:
		return NewSynologyClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported torrent client type: %s", cfg.Type)
	}
}

// IsMagnetLink checks if a URL is a magnet link
func IsMagnetLink(url string) bool {
	return len(url) > 8 && url[:8] == "magnet:?"
}

// IsTorrentURL checks if a URL points to a .torrent file
func IsTorrentURL(url string) bool {
	// Simple check - could be more sophisticated
	return len(url) > 8 && (url[:7] == "http://" || url[:8] == "https://")
}
