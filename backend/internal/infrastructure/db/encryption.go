// infrastructure/db/encryption.go
package db

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	encOnce sync.Once
	encKey  []byte
	encErr  error
)

// getEncryptionKey loads PLAID_TOKEN_ENCRYPTION_KEY exactly once per process.
// The key must be a base64-encoded 32-byte value (generate with: openssl rand -base64 32).
func getEncryptionKey() ([]byte, error) {
	encOnce.Do(func() {
		raw := os.Getenv("PLAID_TOKEN_ENCRYPTION_KEY")
		if raw == "" {
			encErr = fmt.Errorf("encryption: PLAID_TOKEN_ENCRYPTION_KEY is not set — generate with: openssl rand -base64 32")
			return
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			encErr = fmt.Errorf("encryption: invalid base64 in PLAID_TOKEN_ENCRYPTION_KEY: %w", err)
			return
		}
		if len(key) != 32 {
			encErr = fmt.Errorf("encryption: key must be exactly 32 bytes for AES-256 (got %d) — regenerate with: openssl rand -base64 32", len(key))
			return
		}
		encKey = key
	})
	return encKey, encErr
}

// EncryptToken encrypts plaintext using AES-256-GCM.
// Returns a base64-encoded string of (nonce || ciphertext) suitable for MongoDB storage.
func EncryptToken(plaintext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encryption: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryption: generate nonce: %w", err)
	}

	// gcm.Seal appends ciphertext+tag to nonce, giving us a single blob
	blob := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(blob), nil
}

// DecryptToken decodes a base64 blob produced by EncryptToken and decrypts it.
func DecryptToken(ciphertext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}

	blob, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("encryption: decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encryption: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", fmt.Errorf("encryption: ciphertext too short")
	}

	nonce, ciphertextBytes := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("encryption: decrypt/authenticate: %w", err)
	}

	return string(plaintext), nil
}
