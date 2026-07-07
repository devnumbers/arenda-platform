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
