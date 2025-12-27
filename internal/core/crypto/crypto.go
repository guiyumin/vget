// Package crypto provides encryption utilities for sensitive data like API keys.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// SaltSize is the size of the salt in bytes
	SaltSize = 16

	// NonceSize is the size of the nonce for AES-GCM
	NonceSize = 12

	// KeySize is the size of the derived key (AES-256)
	KeySize = 32

	// PBKDF2Iterations is the number of iterations for key derivation
	PBKDF2Iterations = 100000
)

var (
	// ErrInvalidPIN is returned when the PIN format is invalid
	ErrInvalidPIN = errors.New("PIN must be exactly 4 digits")

	// ErrDecryptionFailed is returned when decryption fails (wrong PIN or corrupted data)
	ErrDecryptionFailed = errors.New("decryption failed: wrong PIN or corrupted data")

	// ErrInvalidData is returned when the encrypted data format is invalid
	ErrInvalidData = errors.New("invalid encrypted data format")

	// pinRegex validates 4-digit PINs
	pinRegex = regexp.MustCompile(`^\d{4}$`)
)

// ValidatePIN checks if the PIN is exactly 4 digits.
func ValidatePIN(pin string) error {
	if !pinRegex.MatchString(pin) {
		return ErrInvalidPIN
	}
	return nil
}

// deriveKey derives an AES key from a PIN using PBKDF2.
func deriveKey(pin string, salt []byte) []byte {
	return pbkdf2.Key([]byte(pin), salt, PBKDF2Iterations, KeySize, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM with a key derived from the PIN.
// Returns base64-encoded string containing: salt + nonce + ciphertext.
func Encrypt(plaintext, pin string) (string, error) {
	if err := ValidatePIN(pin); err != nil {
		return "", err
	}

	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive key from PIN
	key := deriveKey(pin, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Combine salt + nonce + ciphertext
	combined := make([]byte, SaltSize+NonceSize+len(ciphertext))
	copy(combined[:SaltSize], salt)
	copy(combined[SaltSize:SaltSize+NonceSize], nonce)
	copy(combined[SaltSize+NonceSize:], ciphertext)

	// Encode as base64
	return base64.StdEncoding.EncodeToString(combined), nil
}

// Decrypt decrypts base64-encoded ciphertext using the PIN.
// Returns the original plaintext.
func Decrypt(encrypted, pin string) (string, error) {
	if err := ValidatePIN(pin); err != nil {
		return "", err
	}

	// Decode base64
	combined, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrInvalidData
	}

	// Minimum size: salt + nonce + at least 16 bytes of ciphertext (GCM tag)
	if len(combined) < SaltSize+NonceSize+16 {
		return "", ErrInvalidData
	}

	// Extract components
	salt := combined[:SaltSize]
	nonce := combined[SaltSize : SaltSize+NonceSize]
	ciphertext := combined[SaltSize+NonceSize:]

	// Derive key from PIN
	key := deriveKey(pin, salt)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}
