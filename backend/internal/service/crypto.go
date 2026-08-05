package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// CryptoService handles AES-256-GCM encryption and decryption for
// storing account passwords securely in the database.
type CryptoService struct {
	gcm cipher.AEAD
}

// NewCryptoService creates a new CryptoService from a 32-byte AES key.
func NewCryptoService(key []byte) (*CryptoService, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	return &CryptoService{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64-encoded
// ciphertext string (nonce prepended to the ciphertext).
func (c *CryptoService) Encrypt(plaintext string) (string, error) {
	// Generate a random nonce (12 bytes for GCM)
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto.Encrypt: failed to generate nonce: %w", err)
	}

	// Encrypt: nonce is prepended to the ciphertext
	ciphertext := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64 for safe storage in PostgreSQL TEXT column
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext back to plaintext.
func (c *CryptoService) Decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: invalid base64: %w", err)
	}

	nonceSize := c.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("crypto.Decrypt: ciphertext too short")
	}

	// Extract nonce and actual ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto.Decrypt: decryption failed: %w", err)
	}

	return string(plaintext), nil
}
