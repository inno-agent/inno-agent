// Package secretbox provides AES-256-GCM encrypt/decrypt helpers.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const nonceSize = 12 // GCM standard nonce length

// SecretBox wraps an AES-256-GCM AEAD.
type SecretBox struct {
	aead cipher.AEAD
}

// NewFromBase64Key creates a SecretBox from a base64-encoded 32-byte key.
func NewFromBase64Key(b64Key string) (*SecretBox, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		// Try RawURLEncoding as a fallback.
		key, err = base64.RawURLEncoding.DecodeString(b64Key)
		if err != nil {
			return nil, fmt.Errorf("secretbox: decode key: %w", err)
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new gcm: %w", err)
	}

	return &SecretBox{aead: aead}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a fresh random nonce.
// Returns (ciphertext, nonce, error).
func (s *SecretBox) Encrypt(plaintext []byte) (ciphertext []byte, nonce []byte, err error) {
	n := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, n); err != nil {
		return nil, nil, fmt.Errorf("secretbox: generate nonce: %w", err)
	}

	ct := s.aead.Seal(nil, n, plaintext, nil)
	return ct, n, nil
}

// Decrypt decrypts ciphertext using the provided nonce.
func (s *SecretBox) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if len(nonce) != nonceSize {
		return nil, errors.New("secretbox: invalid nonce length")
	}

	plain, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secretbox: decrypt: %w", err)
	}

	return plain, nil
}
