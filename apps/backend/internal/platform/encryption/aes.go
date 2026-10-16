// Package encryption provides the Encryptor port and its AES implementation: at-rest encryption, deterministic
// encryption for searchable columns and HMAC token hashing.
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

	"golang.org/x/crypto/hkdf"
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

	return newAESEncryptor(decoded, gcm)
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
	gcm    cipher.AEAD
	detGCM cipher.AEAD
	key    []byte
	encKey []byte
	macKey []byte
}

func newAESEncryptor(key []byte, gcm cipher.AEAD) (*aesEncryptor, error) {
	encKey := deriveKey(key, "encKey")
	macKey := deriveKey(key, "macKey")

	detBlock, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("create deterministic aes cipher: %w", err)
	}
	detGCM, err := cipher.NewGCM(detBlock)
	if err != nil {
		return nil, fmt.Errorf("create deterministic gcm: %w", err)
	}

	return &aesEncryptor{gcm: gcm, detGCM: detGCM, key: key, encKey: encKey, macKey: macKey}, nil
}

func deriveKey(key []byte, label string) []byte {
	dk := make([]byte, 32)
	// HKDF-SHA256 with nil salt and the label as info.
	if _, err := io.ReadFull(hkdf.New(sha256.New, key, nil, []byte(label)), dk); err != nil {
		// HKDF extraction/expand never fails for valid inputs; panic on the impossible.
		panic(fmt.Sprintf("hkdf derive %s: %v", label, err))
	}
	return dk
}

func (e *aesEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DeterministicEncrypt returns a deterministic ciphertext for the given
// plaintext. It is currently intended for phone numbers: the nonce is derived
// with HMAC-SHA256 using a domain-separated label ("\x00phone") so that
// phone-number plaintexts cannot be replayed against other deterministic
// encryption contexts that may be added later.
func (e *aesEncryptor) DeterministicEncrypt(ctx context.Context, plaintext string) (string, error) {
	nonce, err := deterministicNonce(e.macKey, plaintext)
	if err != nil {
		return "", err
	}
	ciphertext := e.detGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func deterministicNonce(macKey []byte, plaintext string) ([]byte, error) {
	mac := hmac.New(sha256.New, macKey)
	// The hash.Hash contract documents that Write never returns an error; the
	// check keeps the contract explicit instead of silently discarding it.
	if _, err := mac.Write([]byte(plaintext)); err != nil {
		return nil, fmt.Errorf("hmac nonce write: %w", err)
	}
	// Domain separator for phone-number deterministic encryption.
	if _, err := mac.Write([]byte("\x00phone")); err != nil {
		return nil, fmt.Errorf("hmac nonce write: %w", err)
	}
	return mac.Sum(nil)[:12], nil
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

	// Try the deterministic cipher first (phone encryption), then fall back to
	// the randomized cipher (legacy/token encryption). Both share the same
	// base64(nonce || ciphertext) format.
	plaintext, err := e.detGCM.Open(nil, nonce, encrypted, nil)
	if err == nil {
		return string(plaintext), nil
	}

	plaintext, err = e.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (e *aesEncryptor) HashToken(plaintext string) string {
	return hashToken(e.key, plaintext)
}

func (e *aesEncryptor) IsNoop() bool { return false }

type noopEncryptor struct{}

func (noopEncryptor) HashToken(plaintext string) string {
	return hashToken(nil, plaintext)
}

func hashToken(key []byte, plaintext string) string {
	h := hmac.New(sha256.New, key)
	// The hash.Hash contract documents that Write never returns an error; the
	// Encryptor port fixes the return to string, so a contract violation panics
	// (must-style, research #321 policy).
	if _, err := h.Write([]byte(plaintext)); err != nil {
		panic("encryption: hash token write: " + err.Error())
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (noopEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	return plaintext, nil
}

func (noopEncryptor) DeterministicEncrypt(ctx context.Context, plaintext string) (string, error) {
	return plaintext, nil
}

func (noopEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	return ciphertext, nil
}

func (noopEncryptor) IsNoop() bool { return true }
