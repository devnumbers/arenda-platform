package config

import (
	"strings"
	"testing"
)

func setRequiredProductionEnv(t *testing.T) {
	t.Helper()

	setRequiredLocalEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_BASE_URL", "https://rentlee.ru")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SMS_SENDER", "disabled")
	t.Setenv("PAYMENT_PROVIDER", "tkassa")
	t.Setenv("T_KASSA_TERMINAL_KEY", "term")
	t.Setenv("T_KASSA_PASSWORD", "pass")
	t.Setenv("T_KASSA_BASE_URL", "https://securepay.tinkoff.ru/v2/")
}

func clearPhotoStorageEnv(t *testing.T) {
	t.Helper()

	t.Setenv("REGRU_S3_ENDPOINT", "")
	t.Setenv("REGRU_S3_REGION", "")
	t.Setenv("REGRU_S3_BUCKET", "")
	t.Setenv("REGRU_S3_ACCESS_KEY", "")
	t.Setenv("REGRU_S3_SECRET_KEY", "")
	t.Setenv("REGRU_S3_PUBLIC_BASE_URL", "")
}

func TestPhotoStorageProviderFakeAllowedInProduction(t *testing.T) {
	setRequiredProductionEnv(t)
	clearPhotoStorageEnv(t)
	t.Setenv("PHOTO_STORAGE_PROVIDER", "fake")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.PhotoStorageProvider != "fake" {
		t.Fatalf("expected photo storage provider fake, got %q", cfg.PhotoStorageProvider)
	}
	if cfg.PhotoStorageS3Enabled {
		t.Fatal("expected S3 photo storage to be disabled")
	}
	if cfg.PhotoStoragePublicBaseURL != "https://rentlee.ru/uploads" {
		t.Fatalf("expected fake public base URL, got %q", cfg.PhotoStoragePublicBaseURL)
	}
}

func TestPhotoStorageProviderS3StillRequiredInProductionByDefault(t *testing.T) {
	setRequiredProductionEnv(t)
	clearPhotoStorageEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing production S3 configuration")
	}
	if !strings.Contains(err.Error(), "PHOTO_STORAGE_PROVIDER=fake") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPhotoStorageProviderInvalid(t *testing.T) {
	setRequiredLocalEnv(t)
	t.Setenv("PHOTO_STORAGE_PROVIDER", "memory")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PHOTO_STORAGE_PROVIDER")
	}
	if !strings.Contains(err.Error(), "PHOTO_STORAGE_PROVIDER") {
		t.Fatalf("unexpected error: %v", err)
	}
}
