package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"smarticky/internal/storage"
)

const (
	keyPath = "secrets/connection-key"
	prefix  = "v1:"
)

// Box encrypts provider credentials before they are stored in the database.
type Box struct {
	aead cipher.AEAD
}

// OpenBox loads or creates the local encryption key for external credentials.
func OpenBox(fs *storage.FileSystem) (*Box, error) {
	if fs == nil {
		fs = storage.NewFileSystem("")
	}

	path := filepath.Join(fs.GetDataDir(), keyPath)
	raw, err := fs.ReadFile(path)
	if err != nil {
		key := make([]byte, 32)
		if _, readErr := rand.Read(key); readErr != nil {
			return nil, fmt.Errorf("generate credential key: %w", readErr)
		}
		encoded := []byte(base64.RawStdEncoding.EncodeToString(key) + "\n")
		if writeErr := fs.WriteFile(path, encoded, 0600); writeErr != nil {
			return nil, fmt.Errorf("save credential key: %w", writeErr)
		}
		return newBox(key)
	}

	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("decode credential key: %w", err)
	}
	return newBox(key)
}

func newBox(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext and returns a versioned base64 payload.
func (b *Box) Seal(plaintext []byte) (string, error) {
	if b == nil {
		return "", errors.New("secret box is not configured")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nil, nonce, plaintext, nil)
	payload := append(nonce, sealed...)
	return prefix + base64.RawStdEncoding.EncodeToString(payload), nil
}

// Open decrypts a versioned payload.
func (b *Box) Open(value string) ([]byte, error) {
	if b == nil {
		return nil, errors.New("secret box is not configured")
	}
	if !strings.HasPrefix(value, prefix) {
		return nil, errors.New("unsupported secret format")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return nil, err
	}
	nonceSize := b.aead.NonceSize()
	if len(raw) <= nonceSize {
		return nil, errors.New("secret payload is too short")
	}
	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]
	return b.aead.Open(nil, nonce, ciphertext, nil)
}
