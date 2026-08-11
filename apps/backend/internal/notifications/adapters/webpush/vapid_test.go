package webpush

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

// generateTestVAPIDKeys produces a P-256 key pair for tests. The private key is
// returned as the raw 32-byte scalar (base64url), the public key as the
// 65-byte uncompressed X9.62 form (base64url). Uses crypto/ecdh to avoid the
// deprecated crypto/elliptic API.
func generateTestVAPIDKeys(t *testing.T) (privB64, pubB64 string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes())
}

func TestNewVAPIDSigner_ValidKeys(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signer.subject != "mailto:test@example.com" {
		t.Errorf("subject = %q", signer.subject)
	}
	if signer.publicKeyB64 != pubB64 {
		t.Errorf("publicKeyB64 mismatch")
	}
}

func TestNewVAPIDSigner_InvalidSubject(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	cases := []struct {
		name    string
		subject string
	}{
		{"empty", ""},
		{"not uri", "just a string"},
		{"http scheme", "http://example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newVAPIDSigner(tc.subject, pubB64, privB64)
			if !errors.Is(err, ErrInvalidVAPIDKey) {
				t.Errorf("expected ErrInvalidVAPIDKey, got %v", err)
			}
		})
	}
}

func TestNewVAPIDSigner_KeyMismatch(t *testing.T) {
	privB64, _ := generateTestVAPIDKeys(t)
	_, otherPubB64 := generateTestVAPIDKeys(t)
	_, err := newVAPIDSigner("mailto:test@example.com", otherPubB64, privB64)
	if !errors.Is(err, ErrInvalidVAPIDKey) {
		t.Errorf("expected ErrInvalidVAPIDKey for mismatched keys, got %v", err)
	}
}

func TestNewVAPIDSigner_BadPrivateKeyLength(t *testing.T) {
	short := base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})
	_, pubB64 := generateTestVAPIDKeys(t)
	_, err := newVAPIDSigner("mailto:test@example.com", pubB64, short)
	if !errors.Is(err, ErrInvalidVAPIDKey) {
		t.Errorf("expected ErrInvalidVAPIDKey, got %v", err)
	}
}

func TestAuthorizationHeader_Structure(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	header, err := signer.authorizationHeader("https://fcm.googleapis.com/fcm/send/abc")
	if err != nil {
		t.Fatalf("authorization header: %v", err)
	}
	if !strings.HasPrefix(header, "vapid t=") {
		t.Errorf("header does not start with 'vapid t=': %q", header)
	}
	if !strings.Contains(header, ",k="+pubB64) {
		t.Errorf("header missing public key segment: %q", header)
	}
}

func TestSignJWT_ClaimsAndSignature(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	fixedNow := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return fixedNow }
	origin := "https://fcm.googleapis.com"

	jwt, err := signer.tokenFor(origin)
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT must have 3 segments, got %d", len(parts))
	}

	// Decode and verify header.
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	if header["alg"] != "ES256" || header["typ"] != "JWT" {
		t.Errorf("unexpected header: %v", header)
	}

	// Decode and verify claims.
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims["aud"] != origin {
		t.Errorf("aud = %v, want %q", claims["aud"], origin)
	}
	if claims["sub"] != "mailto:test@example.com" {
		t.Errorf("sub = %v", claims["sub"])
	}
	wantExp := fixedNow.Add(vapidTokenTTL).Unix()
	if exp, ok := claims["exp"].(float64); !ok || int64(exp) != wantExp {
		t.Errorf("exp = %v, want %d", claims["exp"], wantExp)
	}

	// Verify the ES256 signature.
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("signature must be 64 bytes (R‖S), got %d", len(sig))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	signingInput := parts[0] + "." + parts[1]
	sum := sha256.Sum256([]byte(signingInput))
	if !ecdsa.Verify(&signer.privateKey.PublicKey, sum[:], r, s) {
		t.Error("ES256 signature verification failed")
	}
}

func TestTokenFor_CachesPerOrigin(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signer.now = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

	jwt1, err := signer.tokenFor("https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("token 1: %v", err)
	}
	jwt2, err := signer.tokenFor("https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("token 2: %v", err)
	}
	if jwt1 != jwt2 {
		t.Error("cached token for the same origin should be identical")
	}
	if len(signer.tokens) != 1 {
		t.Errorf("expected 1 cached token, got %d", len(signer.tokens))
	}
}

func TestTokenFor_DistinctOrigins(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signer.now = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }

	jwtFCM, err := signer.tokenFor("https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("token fcm: %v", err)
	}
	jwtApple, err := signer.tokenFor("https://web.push.apple.com")
	if err != nil {
		t.Fatalf("token apple: %v", err)
	}
	if jwtFCM == jwtApple {
		t.Error("tokens for different origins should differ (aud claim)")
	}
	if len(signer.tokens) != 2 {
		t.Errorf("expected 2 cached tokens, got %d", len(signer.tokens))
	}
}

func TestTokenFor_RefreshesAfterExpiry(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return now }

	jwt1, err := signer.tokenFor("https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("token 1: %v", err)
	}

	// Advance past the refresh margin; the cached token must be rotated.
	now = now.Add(vapidTokenTTL)
	jwt2, err := signer.tokenFor("https://fcm.googleapis.com")
	if err != nil {
		t.Fatalf("token 2: %v", err)
	}
	if jwt1 == jwt2 {
		t.Error("token should be refreshed after expiry")
	}
}

func TestOriginOf(t *testing.T) {
	cases := []struct {
		endpoint string
		origin   string
		wantErr  bool
	}{
		{"https://fcm.googleapis.com/fcm/send/abc", "https://fcm.googleapis.com", false},
		{"https://web.push.apple.com/v1/push/xyz", "https://web.push.apple.com", false},
		{"http://localhost:8080/push/abc", "http://localhost:8080", false},
		{"ftp://example.com/push", "", true},
		{"/relative/path", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.endpoint, func(t *testing.T) {
			got, err := originOf(tc.endpoint)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.origin {
				t.Errorf("origin = %q, want %q", got, tc.origin)
			}
		})
	}
}
