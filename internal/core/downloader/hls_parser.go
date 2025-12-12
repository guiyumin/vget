package downloader

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// M3U8Playlist represents a parsed m3u8 playlist
type M3U8Playlist struct {
	Variants      []Variant // For master playlists
	Segments      []Segment // For media playlists
	TotalDuration float64   // Total duration in seconds
	IsMaster      bool      // True if this is a master playlist
	IsEncrypted   bool      // True if segments are encrypted
	KeyURL        string    // URL of encryption key
	KeyIV         string    // Initialization vector for encryption
}

// Variant represents a stream variant in a master playlist
type Variant struct {
	URL        string
	Bandwidth  int
	Resolution string // e.g., "1920x1080"
	Codecs     string
	Name       string // Name or description
}

// Segment represents a single media segment
type Segment struct {
	URL      string
	Duration float64
	Index    int
	Title    string
}

var (
	bandwidthRegex   = regexp.MustCompile(`BANDWIDTH=(\d+)`)
	resolutionRegex  = regexp.MustCompile(`RESOLUTION=(\d+x\d+)`)
	codecsRegex      = regexp.MustCompile(`CODECS="([^"]+)"`)
	nameRegex        = regexp.MustCompile(`NAME="([^"]+)"`)
	extinfoRegex     = regexp.MustCompile(`#EXTINF:([\d.]+)(?:,(.*))?`)
	keyMethodRegex   = regexp.MustCompile(`METHOD=([^,]+)`)
	keyURIRegex      = regexp.MustCompile(`URI="([^"]+)"`)
	keyIVRegex       = regexp.MustCompile(`IV=0x([0-9a-fA-F]+)`)
)

// ParseM3U8 parses an m3u8 playlist from a URL
func ParseM3U8(m3u8URL string) (*M3U8Playlist, error) {
	return ParseM3U8WithHeaders(m3u8URL, nil)
}

// ParseM3U8WithHeaders parses an m3u8 playlist from a URL with custom headers
func ParseM3U8WithHeaders(m3u8URL string, headers map[string]string) (*M3U8Playlist, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:                  http.ProxyFromEnvironment,
			ResponseHeaderTimeout:  30 * time.Second,
			IdleConnTimeout:        90 * time.Second,
		},
	}

	req, err := http.NewRequest("GET", m3u8URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// Apply custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch m3u8: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return parseM3U8Content(resp.Body, m3u8URL)
}

// parseM3U8Content parses m3u8 content from a reader
func parseM3U8Content(reader io.Reader, baseURL string) (*M3U8Playlist, error) {
	scanner := bufio.NewScanner(reader)
	playlist := &M3U8Playlist{}

	var currentSegmentDuration float64
	var currentSegmentTitle string
	var segmentIndex int

	// Parse base URL for resolving relative URLs
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Check for master playlist indicators
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			playlist.IsMaster = true
			variant := parseVariant(line)

			// Next non-comment line should be the URL
			for scanner.Scan() {
				nextLine := strings.TrimSpace(scanner.Text())
				if nextLine == "" || strings.HasPrefix(nextLine, "#") {
					continue
				}
				variant.URL = resolveURL(base, nextLine)
				break
			}
			playlist.Variants = append(playlist.Variants, variant)
			continue
		}

		// Parse encryption key
		if strings.HasPrefix(line, "#EXT-X-KEY:") {
			method := extractRegex(keyMethodRegex, line)
			if method != "NONE" && method != "" {
				playlist.IsEncrypted = true
				keyURI := extractRegex(keyURIRegex, line)
				if keyURI != "" {
					playlist.KeyURL = resolveURL(base, keyURI)
				}
				playlist.KeyIV = extractRegex(keyIVRegex, line)
			}
			continue
		}

		// Parse segment info
		if strings.HasPrefix(line, "#EXTINF:") {
			matches := extinfoRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				currentSegmentDuration, _ = strconv.ParseFloat(matches[1], 64)
				if len(matches) >= 3 {
					currentSegmentTitle = matches[2]
				}
			}
			continue
		}

		// Skip other directives
		if strings.HasPrefix(line, "#") {
			continue
		}

		// This is a segment URL
		if currentSegmentDuration > 0 || !playlist.IsMaster {
			segment := Segment{
				URL:      resolveURL(base, line),
				Duration: currentSegmentDuration,
				Index:    segmentIndex,
				Title:    currentSegmentTitle,
			}
			playlist.Segments = append(playlist.Segments, segment)
			playlist.TotalDuration += currentSegmentDuration
			segmentIndex++
			currentSegmentDuration = 0
			currentSegmentTitle = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading m3u8: %w", err)
	}

	return playlist, nil
}

// parseVariant extracts variant information from EXT-X-STREAM-INF line
func parseVariant(line string) Variant {
	return Variant{
		Bandwidth:  extractInt(bandwidthRegex, line),
		Resolution: extractRegex(resolutionRegex, line),
		Codecs:     extractRegex(codecsRegex, line),
		Name:       extractRegex(nameRegex, line),
	}
}

// resolveURL resolves a potentially relative URL against a base URL
func resolveURL(base *url.URL, ref string) string {
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(refURL).String()
}

// extractRegex extracts the first capturing group from a regex match
func extractRegex(re *regexp.Regexp, s string) string {
	matches := re.FindStringSubmatch(s)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// extractInt extracts an integer from the first capturing group
func extractInt(re *regexp.Regexp, s string) int {
	str := extractRegex(re, s)
	if str == "" {
		return 0
	}
	val, _ := strconv.Atoi(str)
	return val
}

// SelectBestVariant returns the highest bandwidth variant
func (p *M3U8Playlist) SelectBestVariant() *Variant {
	if len(p.Variants) == 0 {
		return nil
	}

	best := &p.Variants[0]
	for i := range p.Variants {
		if p.Variants[i].Bandwidth > best.Bandwidth {
			best = &p.Variants[i]
		}
	}
	return best
}

// SelectVariantByResolution returns the variant matching the resolution
func (p *M3U8Playlist) SelectVariantByResolution(resolution string) *Variant {
	for i := range p.Variants {
		if p.Variants[i].Resolution == resolution {
			return &p.Variants[i]
		}
	}
	return nil
}
