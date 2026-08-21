package smtp

import (
	"crypto/tls"
	"testing"
)

// TestTLSConfig_PinsTLS12Floor verifies the TLS configuration used for both
// implicit TLS (port 465) and STARTTLS (port 587): the server name must be
// carried for certificate validation, and the minimum protocol version must be
// pinned to TLS 1.2 explicitly (semgrep p/ci missing-ssl-minversion, #318) —
// Go's client default floor is TLS 1.2 today, and the pin keeps it from
// silently dropping if that default ever changes.
func TestTLSConfig_PinsTLS12Floor(t *testing.T) {
	t.Parallel()

	cfg := tlsConfig("smtp.example.com")

	if cfg.ServerName != "smtp.example.com" {
		t.Fatalf("ServerName = %q, want %q", cfg.ServerName, "smtp.example.com")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want %d (tls.VersionTLS12)", cfg.MinVersion, tls.VersionTLS12)
	}
}
