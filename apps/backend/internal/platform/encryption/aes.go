package encryption

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// NewEncryptor creates an Encryptor from a key.
//
// The key must decode to exactly 32 bytes and may be hex-encoded (64 characters)
// or base64-encoded. If key is empty, a pass-through/no-op encryptor is returned
// for local development.
func NewEncryptor(key string) (Encryptor, error) {
	if key == "" {
		return noopEncryptor{}, nil
	}

	decoded, err := parseKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse encryption key: %w", err)
	}

	block, err := aes.NewCipher(decoded)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &aesEncryptor{gcm: gcm, key: decoded}, nil
}

func parseKey(key string) ([]byte, error) {
	// 64 hex characters encode 32 bytes exactly.
	if len(key) == 64 {
		decoded, err := hex.DecodeString(key)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("key is neither 64-char hex nor valid base64: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("key must decode to 32 bytes, got %d", len(decoded))
	}
	return decoded, nil
}

type aesEncryptor struct {
	gcm cipher.AEAD
	key []byte
}

func (e *aesEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *aesEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(data) < e.gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, encrypted := data[:e.gcm.NonceSize()], data[e.gcm.NonceSize():]
	plaintext, err := e.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (e *aesEncryptor) HashToken(plaintext string) string {
	return hashToken(e.key, plaintext)
}

type noopEncryptor struct{}

func (noopEncryptor) HashToken(plaintext string) string {
	return hashToken(nil, plaintext)
}

func hashToken(key []byte, plaintext string) string {
	h := hmac.New(sha256.New, key)
	// hmac.Write never returns an error for the hash.Hash contract.
	_, _ = h.Write([]byte(plaintext))
	return hex.EncodeToString(h.Sum(nil))
}

func (noopEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	return plaintext, nil
}

func (noopEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	return ciphertext, nil
}
