package encryption

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestNoopEncryptor_RoundTrip(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("NewEncryptor(\"\") error = %v", err)
	}

	plaintext := "sensitive-token"
	ciphertext, err := enc.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt error = %v", err)
	}
	if ciphertext != plaintext {
		t.Errorf("noop ciphertext = %q, want %q", ciphertext, plaintext)
	}

	got, err := enc.Decrypt(context.Background(), ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error = %v", err)
	}
	if got != plaintext {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestAESEncryptor_HexKey_RoundTrip(t *testing.T) {
	key := hex.EncodeToString(make([]byte, 32))
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor error = %v", err)
	}

	plaintext := "sensitive-token"
	ciphertext, err := enc.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt error = %v", err)
	}
	if ciphertext == plaintext {
		t.Error("ciphertext equals plaintext")
	}

	got, err := enc.Decrypt(context.Background(), ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error = %v", err)
	}
	if got != plaintext {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestAESEncryptor_Base64Key_RoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor error = %v", err)
	}

	plaintext := "sensitive-token"
	ciphertext, err := enc.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt error = %v", err)
	}

	got, err := enc.Decrypt(context.Background(), ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error = %v", err)
	}
	if got != plaintext {
		t.Errorf("decrypted = %q, want %q", got, plaintext)
	}
}

func TestAESEncryptor_HashToken(t *testing.T) {
	key := hex.EncodeToString(make([]byte, 32))
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor error = %v", err)
	}

	hash1 := enc.HashToken("same-token")
	hash2 := enc.HashToken("same-token")
	hash3 := enc.HashToken("different-token")

	if hash1 != hash2 {
		t.Errorf("HashToken not deterministic: %q != %q", hash1, hash2)
	}
	if hash1 == hash3 {
		t.Errorf("HashToken collision: %q == %q", hash1, hash3)
	}
	if len(hash1) != 64 {
		t.Errorf("HashToken length = %d, want 64", len(hash1))
	}
}

func TestNoopEncryptor_HashToken(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("NewEncryptor error = %v", err)
	}

	hash1 := enc.HashToken("token")
	hash2 := enc.HashToken("token")
	if hash1 != hash2 {
		t.Errorf("HashToken not deterministic: %q != %q", hash1, hash2)
	}
}

func TestNewEncryptor_InvalidKey(t *testing.T) {
	cases := []string{
		"short",
		"0000000000000000000000000000000000000000000000000000000000000000extra",
	}

	for _, key := range cases {
		_, err := NewEncryptor(key)
		if err == nil {
			t.Errorf("NewEncryptor(%q) expected error, got nil", key)
		}
	}
}

func TestAESEncryptor_Decrypt_TamperedCiphertext(t *testing.T) {
	key := hex.EncodeToString(make([]byte, 32))
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("NewEncryptor error = %v", err)
	}

	ciphertext, err := enc.Encrypt(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Encrypt error = %v", err)
	}

	if _, err := enc.Decrypt(context.Background(), ciphertext+"x"); err == nil {
		t.Error("Decrypt tampered ciphertext expected error, got nil")
	}
}
