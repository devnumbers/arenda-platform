package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
)

// noopEnc returns the development pass-through encryptor.
func noopEnc(t *testing.T) encryption.Encryptor {
	t.Helper()
	enc, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("NewEncryptor(\"\"): %v", err)
	}
	return enc
}

func TestPhoneToColumns_NoopEncryptor(t *testing.T) {
	t.Parallel()
	enc := noopEnc(t)
	ctx := t.Context()

	encryptedPhone, phoneEncrypted, err := phoneToColumns(ctx, enc, "+79160006000")
	if err != nil {
		t.Fatalf("phoneToColumns error = %v", err)
	}
	// A noop encryptor stores the value as-is.
	if encryptedPhone != "+79160006000" {
		t.Fatalf("encryptedPhone = %q, want pass-through +79160006000", encryptedPhone)
	}
	if phoneEncrypted {
		t.Fatal("phoneEncrypted = true, want false for noop encryptor")
	}
}

func TestPhoneToColumns_RealEncryptor(t *testing.T) {
	t.Parallel()
	// A 32-byte hex key → real AES encryptor.
	enc, err := encryption.NewEncryptor("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	ctx := t.Context()

	encryptedPhone, phoneEncrypted, err := phoneToColumns(ctx, enc, "+79160006001")
	if err != nil {
		t.Fatalf("phoneToColumns error = %v", err)
	}
	if encryptedPhone == "+79160006001" {
		t.Fatal("encryptedPhone equals plaintext, want ciphertext from real encryptor")
	}
	if !phoneEncrypted {
		t.Fatal("phoneEncrypted = false, want true for real encryptor")
	}

	// Round-trip: decrypting yields the original phone.
	decrypted, err := decryptPhone(ctx, enc, encryptedPhone, true)
	if err != nil {
		t.Fatalf("decryptPhone error = %v", err)
	}
	if decrypted != "+79160006001" {
		t.Fatalf("decrypted = %q, want +79160006001", decrypted)
	}
}

func TestDecryptPhone_NotEncryptedReturnsAsIs(t *testing.T) {
	t.Parallel()
	enc := noopEnc(t)
	ctx := t.Context()

	// When encrypted=false, decryptPhone returns the value without calling Decrypt.
	got, err := decryptPhone(ctx, enc, "+79160006002", false)
	if err != nil {
		t.Fatalf("decryptPhone error = %v", err)
	}
	if got != "+79160006002" {
		t.Fatalf("decryptPhone = %q, want +79160006002", got)
	}
}

func TestEncryptPhone_WrapsError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	enc := &errEncryptor{}

	_, err := encryptPhone(ctx, enc, "+79160006003")
	if err == nil {
		t.Fatal("encryptPhone error = nil, want error from failing encryptor")
	}
	// The error should wrap the underlying encryptor error.
	if !errors.Is(err, errEncryptorSentinel) {
		t.Fatalf("encryptPhone error = %v, want wrap of sentinel", err)
	}
}

func TestDecryptPhoneField_NoopRoundTrip(t *testing.T) {
	t.Parallel()
	enc := noopEnc(t)
	ctx := t.Context()

	// With a noop encryptor, phoneToColumns stores plaintext and
	// decryptPhoneField parses it back into a domain.Phone.
	encrypted, phoneEncrypted, err := phoneToColumns(ctx, enc, "+79160006004")
	if err != nil {
		t.Fatalf("phoneToColumns: %v", err)
	}
	got, err := decryptPhoneField(ctx, enc, encrypted, phoneEncrypted)
	if err != nil {
		t.Fatalf("decryptPhoneField error = %v", err)
	}
	if got.String() != "+79160006004" {
		t.Fatalf("phone = %s, want +79160006004", got.String())
	}
}

func TestDecryptPhoneField_InvalidPhoneReturnsError(t *testing.T) {
	t.Parallel()
	enc := noopEnc(t)
	ctx := t.Context()

	_, err := decryptPhoneField(ctx, enc, "not-a-phone", false)
	if err == nil {
		t.Fatal("decryptPhoneField(invalid) error = nil, want parse error")
	}
}

// errEncryptor is an Encryptor whose DeterministicEncrypt/Decrypt always fail.
type errEncryptor struct{}

var errEncryptorSentinel = errors.New("encryption unavailable")

func (errEncryptor) Encrypt(context.Context, string) (string, error) { return "", errEncryptorSentinel }
func (errEncryptor) DeterministicEncrypt(context.Context, string) (string, error) {
	return "", errEncryptorSentinel
}
func (errEncryptor) Decrypt(context.Context, string) (string, error) { return "", errEncryptorSentinel }
func (errEncryptor) HashToken(string) string                         { return "" }
func (errEncryptor) IsNoop() bool                                    { return false }
