package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/guiyumin/vget/internal/core/i18n"
)

// ErrAuthServerDown indicates the auth server is unreachable
var ErrAuthServerDown = errors.New("auth server unreachable")

// AuthEndpoint is the backend endpoint for authentication.
var AuthEndpoint = getAuthEndpoint()

func getAuthEndpoint() string {
	// Allow override via environment variable (for development)
	if endpoint := os.Getenv("VGET_AUTH_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return "https://vget.io/api/v1/token"
}

// User-Agent for vget requests
const vgetUserAgent = "vget/1.0 (+https://github.com/guiyumin/vget)"

// AuthData represents the cached device registration.
type AuthData struct {
	Email       string `json:"email"`
	Fingerprint string `json:"fingerprint"`
}

// authFilePath returns the path to the auth cache file.
func authFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vget", "auth.json"), nil
}

// LoadAuth loads cached authentication data.
func LoadAuth() *AuthData {
	path, err := authFilePath()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var auth AuthData
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil
	}

	// Check if fingerprint matches current device
	if auth.Fingerprint != GetDeviceFingerprint() {
		return nil
	}

	return &auth
}

// SaveAuth saves authentication data to cache.
func SaveAuth(auth *AuthData) error {
	path, err := authFilePath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ClearAuth removes cached authentication data.
func ClearAuth() error {
	path, err := authFilePath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// SignedURLRequest is the request body for token endpoint.
type SignedURLRequest struct {
	Email       string `json:"email"`
	Fingerprint string `json:"fingerprint"`
	File        string `json:"file"`
}

// SignedURLResponse is the response from token endpoint.
type SignedURLResponse struct {
	URL       string `json:"url"`
	ExpiresIn int64  `json:"expires_in"` // seconds
	Error     string `json:"error,omitempty"`
}

// RequestSignedURL requests a signed CDN URL from the auth server.
func RequestSignedURL(email, file string) (*SignedURLResponse, error) {
	fingerprint := GetDeviceFingerprint()

	reqBody := SignedURLRequest{
		Email:       email,
		Fingerprint: fingerprint,
		File:        file,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", AuthEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", vgetUserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Connection error (timeout, refused, DNS failure, etc.)
		return nil, ErrAuthServerDown
	}
	defer resp.Body.Close()

	var signedResp SignedURLResponse
	if err := json.NewDecoder(resp.Body).Decode(&signedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if signedResp.Error != "" {
		return nil, fmt.Errorf("auth error: %s", signedResp.Error)
	}

	if signedResp.URL == "" {
		return nil, fmt.Errorf("no URL in response")
	}

	return &signedResp, nil
}

// PromptEmail prompts the user for their email address.
func PromptEmail(lang string) (string, error) {
	t := i18n.T(lang)
	fmt.Printf("  %s\n", t.AICLI.FakeEmailWarning)
	fmt.Printf("  %s ", t.AICLI.EnterEmail)

	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return "", errors.New(t.AICLI.EmailRequired)
	}

	// Validate email syntax
	if !IsValidEmail(email) {
		return "", errors.New(t.AICLI.InvalidEmail)
	}

	return email, nil
}

// GetSignedURL gets a signed CDN URL for downloading a file.
// Prompts for email if not registered.
func GetSignedURL(file, lang string) (string, error) {
	// Check cached auth
	auth := LoadAuth()
	var email string

	if auth != nil {
		email = auth.Email
	} else {
		// Prompt for email
		var err error
		email, err = PromptEmail(lang)
		if err != nil {
			return "", err
		}

		// Save auth for future use
		auth = &AuthData{
			Email:       email,
			Fingerprint: GetDeviceFingerprint(),
		}
		if err := SaveAuth(auth); err != nil {
			// Non-fatal, just warn
			fmt.Printf("  Warning: failed to cache auth: %v\n", err)
		}
	}

	// Request signed URL
	resp, err := RequestSignedURL(email, file)
	if err != nil {
		if errors.Is(err, ErrAuthServerDown) {
			t := i18n.T(lang)
			return "", errors.New(t.AICLI.AuthServerDown)
		}
		return "", err
	}

	return resp.URL, nil
}

// IsRegistered checks if the device is registered.
func IsRegistered() bool {
	return LoadAuth() != nil
}
