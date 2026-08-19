package webpush

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// vapidTokenTTL is how long a cached VAPID JWT is considered valid. RFC 8292
// §2 caps exp at 24h; ~12h is the recommended margin against clock skew
// (research #174 §1.1, web.dev). Apple throttles frequent re-signing, so one
// token per origin per 12h keeps push services happy.
const vapidTokenTTL = 12 * time.Hour

// vapidTokenRefreshMargin is subtracted from exp when deciding whether a cached
// token is still fresh, so the token is rotated slightly before expiry.
const vapidTokenRefreshMargin = 5 * time.Minute

// ErrInvalidVAPIDKey is returned when a VAPID key or subject is malformed.
var ErrInvalidVAPIDKey = errors.New("webpush: invalid VAPID key")

// p256OrderHex is the order n of the P-256 base point group (SEC 2 §2.4.2).
const p256OrderHex = "ffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551"

// p256Order is the ES256 modulus for scalar arithmetic. The stdlib exposes the
// value only through crypto/elliptic, which this package dropped (ADR 0043),
// so it is pinned as a constant and guarded against drift by
// TestP256Order_MatchesStdlib.
var p256Order, _ = new(big.Int).SetString(p256OrderHex, 16)

// vapidSigner signs per-origin ES256 JWTs (RFC 8292) and caches them so the
// same origin reuses one token for vapidTokenTTL instead of re-signing on every
// message.
type vapidSigner struct {
	privateKey *ecdh.PrivateKey
	// The publicKeyB64 field is the base64url-encoded uncompressed public key
	// placed in the Authorization header's k= segment.
	publicKeyB64 string
	subject      string

	now func() time.Time

	mu     sync.Mutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	jwt string
	exp time.Time
}

// newVAPIDSigner parses the base64url-encoded private key (32-byte P-256
// scalar) and the subject (mailto:/https: URI, RFC 8292 §2.1). The public key
// is validated against the one derived from the private key.
func newVAPIDSigner(subject, publicKeyB64, privateKeyB64 string) (*vapidSigner, error) {
	if subject == "" {
		return nil, fmt.Errorf("%w: subject is empty", ErrInvalidVAPIDKey)
	}
	if !strings.HasPrefix(subject, "mailto:") && !strings.HasPrefix(subject, "https:") {
		return nil, fmt.Errorf("%w: subject must be a mailto: or https: URI", ErrInvalidVAPIDKey)
	}

	dBytes, err := base64urlDecode(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: decode private key: %w", ErrInvalidVAPIDKey, err)
	}
	if len(dBytes) != 32 {
		return nil, fmt.Errorf("%w: private key must be 32 bytes, got %d", ErrInvalidVAPIDKey, len(dBytes))
	}

	// The key stays a crypto/ecdh key end-to-end (ADR 0043): NewPrivateKey
	// validates the scalar and derives the uncompressed public key.
	ecdhPriv, err := ecdh.P256().NewPrivateKey(dBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid private key scalar: %w", ErrInvalidVAPIDKey, err)
	}
	derivedPub := ecdhPriv.PublicKey().Bytes()

	// Validate the supplied public key matches the one derived from the private
	// key. A mismatch means the env vars are inconsistent — push would fail at
	// the push service with VapidPkHashMismatch (RFC 8292 §4). The length is
	// validated by ecdh.NewPublicKey, so we only compare bytes here.
	suppliedPub, err := base64urlDecode(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: decode public key: %w", ErrInvalidVAPIDKey, err)
	}
	if !bytes.Equal(suppliedPub, derivedPub) {
		return nil, fmt.Errorf("%w: public key does not match private key", ErrInvalidVAPIDKey)
	}

	return &vapidSigner{
		privateKey:   ecdhPriv,
		publicKeyB64: publicKeyB64,
		subject:      subject,
		now:          time.Now,
		tokens:       make(map[string]cachedToken),
	}, nil
}

// authorizationHeader builds the RFC 8292 Authorization header value
// "vapid t=<jwt>,k=<base64url-publickey>" for the push service origin of the
// given endpoint. The JWT is cached per origin for vapidTokenTTL.
func (v *vapidSigner) authorizationHeader(endpoint string) (string, error) {
	origin, err := originOf(endpoint)
	if err != nil {
		return "", err
	}

	jwt, err := v.tokenFor(origin)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("vapid t=%s,k=%s", jwt, v.publicKeyB64), nil
}

// tokenFor returns a cached JWT for the origin if still fresh, or mints a new
// one. The cache is keyed by origin so all endpoints of the same push service
// share one token (RFC 8292 §5).
func (v *vapidSigner) tokenFor(origin string) (string, error) {
	now := v.now()

	v.mu.Lock()
	defer v.mu.Unlock()

	if c, ok := v.tokens[origin]; ok && now.Before(c.exp.Add(-vapidTokenRefreshMargin)) {
		return c.jwt, nil
	}

	exp := now.Add(vapidTokenTTL)
	jwt, err := v.signJWT(origin, exp)
	if err != nil {
		return "", err
	}

	v.tokens[origin] = cachedToken{jwt: jwt, exp: exp}
	return jwt, nil
}

// signJWT builds and signs an ES256 JWT with claims aud (origin), exp, sub.
func (v *vapidSigner) signJWT(aud string, exp time.Time) (string, error) {
	header := map[string]string{"alg": "ES256", "typ": "JWT"}
	claims := map[string]any{
		"aud": aud,
		"exp": exp.Unix(),
		"sub": v.subject,
	}

	headerB64, err := joseBase64Encode(header)
	if err != nil {
		return "", fmt.Errorf("encode jwt header: %w", err)
	}
	claimsB64, err := joseBase64Encode(claims)
	if err != nil {
		return "", fmt.Errorf("encode jwt claims: %w", err)
	}

	signingInput := headerB64 + "." + claimsB64
	sig, err := v.es256Sign([]byte(signingInput))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// errES256ZeroComponent marks a nonce whose signature component r or s came
// out zero. FIPS 186 §6.4.1 requires rejecting such signatures; the chance is
// ~2^-256, so es256Sign just draws a fresh nonce.
var errES256ZeroComponent = errors.New("webpush: es256 nonce produced a zero signature component")

// es256Sign signs the SHA-256 digest of data with ECDSA P-256 and returns the
// raw R‖S concatenation (64 bytes, big-endian, zero-padded) as required by JOSE
// (RFC 7518 §3.4). The signer key stays a crypto/ecdh key end-to-end (ADR
// 0043): stdlib offers no ES256 signing over ecdh keys, so the signature
// equation is implemented here, while the one curve operation it needs — the
// nonce point k·G — goes through an ecdh public key.
func (v *vapidSigner) es256Sign(data []byte) ([]byte, error) {
	digest := sha256.Sum256(data)
	for {
		// Nonce: 32 uniform random bytes, rejection-sampled into [1, n-1] so
		// the scalar reaching the modular equation is unbiased.
		nonce := make([]byte, 32)
		if _, err := rand.Read(nonce); err != nil {
			return nil, fmt.Errorf("generate es256 nonce: %w", err)
		}
		k := new(big.Int).SetBytes(nonce)
		if k.Sign() == 0 || k.Cmp(p256Order) >= 0 {
			continue
		}

		sig, err := v.es256SignDigest(digest[:], k)
		if errors.Is(err, errES256ZeroComponent) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return sig, nil
	}
}

// es256SignDigest computes the ECDSA signature for an explicit digest and
// nonce scalar; es256Sign passes a random k, the known-answer test a fixed
// one. The nonce k must be in [1, n-1], and a nonce must never be reused
// across two signatures — reusing one exposes the private key. The nonce
// point k·G is derived through crypto/ecdh, and r and s then follow the plain
// modular equation s = k⁻¹(e + r·d) mod n over big.Int.
func (v *vapidSigner) es256SignDigest(digest []byte, k *big.Int) ([]byte, error) {
	kBytes := make([]byte, 32)
	k.FillBytes(kBytes)
	noncePriv, err := ecdh.P256().NewPrivateKey(kBytes)
	if err != nil {
		return nil, fmt.Errorf("es256 nonce rejected: %w", err)
	}
	// Uncompressed point: 0x04 || X || Y. The r component is the x coordinate modulo n.
	point := noncePriv.PublicKey().Bytes()

	r := new(big.Int).SetBytes(point[1:33])
	r.Mod(r, p256Order)
	if r.Sign() == 0 {
		return nil, errES256ZeroComponent
	}

	// The s value is k⁻¹(e + r·d) mod n (FIPS 186 §6.4.1). The SHA-256 digest
	// is used whole: its bit length matches the P-256 order, so only the final
	// reduction modulo n applies.
	d := new(big.Int).SetBytes(v.privateKey.Bytes())
	e := new(big.Int).SetBytes(digest)
	e.Mod(e, p256Order)

	num := new(big.Int).Mul(r, d)
	num.Add(e, num)
	num.Mod(num, p256Order)
	kInv := new(big.Int).ModInverse(k, p256Order)
	if kInv == nil {
		// Unreachable for k in [1, n-1] since n is prime; the guard turns a
		// violated precondition into an error instead of a nil dereference.
		return nil, errors.New("es256 nonce not invertible")
	}
	s := new(big.Int).Mul(kInv, num)
	s.Mod(s, p256Order)
	if s.Sign() == 0 {
		return nil, errES256ZeroComponent
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig, nil
}

// joseBase64Encode marshals v as compact JSON and base64url-encodes it without
// padding (JOSE, RFC 7515 §2).
func joseBase64Encode(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// originOf extracts the Unicode serialization of the origin (scheme://host)
// from a push endpoint URL (RFC 8292 §2). The aud claim MUST be the origin, not
// the full endpoint URL. The endpoint is a secret (RFC 8030 §8.3), so it is
// never included verbatim in returned errors.
func originOf(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse push endpoint: %s", sanitize.Error(err))
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("invalid push endpoint origin")
	}
	return u.Scheme + "://" + u.Host, nil
}
