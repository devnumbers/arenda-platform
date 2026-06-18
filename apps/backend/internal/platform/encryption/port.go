package encryption

import "context"

// Encryptor encrypts and decrypts sensitive values at rest.
type Encryptor interface {
	Encrypt(ctx context.Context, plaintext string) (string, error)
	Decrypt(ctx context.Context, ciphertext string) (string, error)
	// HashToken returns a deterministic HMAC-SHA256 hash of the plaintext token,
	// encoded as a hex string. It is used for duplicate-token detection while
	// keeping the raw token encrypted at rest.
	HashToken(plaintext string) string
}
