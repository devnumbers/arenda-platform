package webpush

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// ecdsaPublicKeyFromB64 rebuilds an ecdsa.PublicKey from the base64url
// uncompressed point, so signatures produced by the hand-rolled ES256 signer
// can be checked with the stdlib verifier — an independent implementation of
// the same ECDSA algebra.
func ecdsaPublicKeyFromB64(t *testing.T, b64 string) *ecdsa.PublicKey {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(raw) != 65 || raw[0] != 0x04 {
		t.Fatalf("public key must be a 65-byte uncompressed point, got %d bytes", len(raw))
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(raw[1:33]),
		Y:     new(big.Int).SetBytes(raw[33:65]),
	}
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
	if !ecdsa.Verify(ecdsaPublicKeyFromB64(t, pubB64), sum[:], r, s) {
		t.Error("ES256 signature verification failed")
	}
}

// ES256 known-answer vector: fixed P-256 scalar, nonce, and digest with the
// signature computed by an independent Python implementation over the same
// SEC 2 parameters. The values are test data, not credentials. They pin the
// modular equation, the nonce-point derivation, and the R‖S encoding.
const (
	es256VectorPrivB64   = "otzP3B2URjIzFBddDfyeFB51MotdU1JRhaEU5JsKk0I"
	es256VectorPubB64    = "BHS7trsUlvEqY76PT8ae6cAjyqH-80CkV4E5I4I8BEFb3oMM9XVodyFopZWwFZM1HM6JNk89I4mezIYFEUdM4EY"
	es256VectorKHex      = "b1524c4db2b61195b470e6a22fa86d786edb77a7163952223c89135d76e335c3"
	es256VectorDigestHex = "babb529527791c5956ec524633cccd7020f484c605d54d0e3dc8061110f20790"
	es256VectorSigHex    = "b4462b8031b1883c5b62118cfb7d076e08a805ec7f6077a355d1bc07dd8510ba1a8650647e8fa619c26e21ed550dbe6aa0a13f5036ca318d912ace0cb49ce652"
)

func TestES256Sign_VerifiesWithStdlib(t *testing.T) {
	// The signer draws a fresh nonce per call, so the output is not
	// reproducible; what must hold for every draw is that stdlib ECDSA accepts
	// it. Several rounds over fresh key pairs exercise the rejection-sampling
	// path and the R‖S encoding.
	const rounds = 25
	for range rounds {
		privB64, pubB64 := generateTestVAPIDKeys(t)
		signer, err := newVAPIDSigner("mailto:test@example.com", pubB64, privB64)
		if err != nil {
			t.Fatalf("new signer: %v", err)
		}

		data := []byte("es256 cross-verification round")
		sig, err := signer.es256Sign(data)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if len(sig) != 64 {
			t.Fatalf("signature length = %d, want 64", len(sig))
		}

		digest := sha256.Sum256(data)
		r := new(big.Int).SetBytes(sig[:32])
		s := new(big.Int).SetBytes(sig[32:])
		if !ecdsa.Verify(ecdsaPublicKeyFromB64(t, pubB64), digest[:], r, s) {
			t.Fatal("stdlib ecdsa.Verify rejected the signature")
		}
	}
}

func TestES256SignDigest_KnownAnswerVector(t *testing.T) {
	signer, err := newVAPIDSigner("mailto:test@example.com", es256VectorPubB64, es256VectorPrivB64)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	k, err := hex.DecodeString(es256VectorKHex)
	if err != nil {
		t.Fatalf("decode nonce: %v", err)
	}
	digest, err := hex.DecodeString(es256VectorDigestHex)
	if err != nil {
		t.Fatalf("decode digest: %v", err)
	}

	sig, err := signer.es256SignDigest(digest, new(big.Int).SetBytes(k))
	if err != nil {
		t.Fatalf("sign with fixed nonce: %v", err)
	}

	if got := hex.EncodeToString(sig); got != es256VectorSigHex {
		t.Errorf("signature mismatch\n got: %s\nwant: %s", got, es256VectorSigHex)
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(ecdsaPublicKeyFromB64(t, es256VectorPubB64), digest, r, s) {
		t.Error("stdlib ecdsa.Verify rejected the known-answer signature")
	}
}

func TestP256Order_MatchesStdlib(t *testing.T) {
	// The order constant is hardcoded because stdlib exposes it only through
	// crypto/elliptic, which the signer dropped (ADR 0043); the pin catches a
	// typo against the stdlib value.
	if p256Order.Cmp(elliptic.P256().Params().N) != 0 {
		t.Errorf("p256Order = %x, want stdlib %x", p256Order, elliptic.P256().Params().N)
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
