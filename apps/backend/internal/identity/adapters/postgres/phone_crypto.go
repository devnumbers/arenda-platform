package postgres

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
)

func encryptPhone(ctx context.Context, enc encryption.Encryptor, phone string) (string, error) {
	encrypted, err := enc.DeterministicEncrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("encrypt phone: %w", err)
	}
	return encrypted, nil
}

// phoneToColumns encrypts phone for persistence and derives the PhoneEncrypted
// flag so callers cannot forget it — the two facts are one operation. Symmetric
// to decryptPhoneField on the read side. Use this on every write site; use
// encryptPhone only for lookups/deletes where the flag is not persisted.
func phoneToColumns(ctx context.Context, enc encryption.Encryptor, phone string) (encryptedPhone string, phoneEncrypted bool, err error) {
	encryptedPhone, err = encryptPhone(ctx, enc, phone)
	if err != nil {
		return "", false, err
	}
	return encryptedPhone, !enc.IsNoop(), nil
}

func decryptPhone(ctx context.Context, enc encryption.Encryptor, phone string, encrypted bool) (string, error) {
	if !encrypted {
		return phone, nil
	}
	decrypted, err := enc.Decrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("decrypt phone: %w", err)
	}
	return decrypted, nil
}
