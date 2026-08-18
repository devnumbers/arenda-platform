// Package webpush implements the application.PushSender port with a stdlib-only
// Web Push adapter: RFC 8291 payload encryption (aes128gcm), RFC 8292 VAPID
// identification (ES256), and RFC 8030 HTTP delivery.
package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// recordSize is the aes128gcm record size (rs) advertised in the RFC 8188
// content-coding header. RFC 8291 §4 requires a single record; 4096 is the
// conventional maximum supported by push services and matches the RFC 8291
// test vectors. The plaintext budget is rs minus the 16-byte GCM tag, the
// 1-byte padding delimiter, and the 86-byte header — i.e. 3993 bytes.
const recordSize = 4096

// headerLen is the fixed size of the RFC 8188 content-coding header prepended
// to the ciphertext: salt (16) + rs (4) + idlen (1) + key (65) = 86 bytes.
const headerLen = 16 + 4 + 1 + 65

// maxPlaintextLen is the largest plaintext the single-record encoding accepts
// (research #174 §1.2, RFC 8291 §4): rs − header − 1 (padding delimiter) − 16 (tag).
const maxPlaintextLen = recordSize - headerLen - 1 - 16

// p256UncompressedSize is the size of an uncompressed P-256 public key in
// X9.62/SEC 1 form: 0x04 prefix + 32-byte X + 32-byte Y.
const p256UncompressedSize = 65

// errPayloadTooLarge is returned when the plaintext exceeds the single-record
// budget. The application payload is already capped at MaxPayloadBytes
// (3993 bytes) by PushPayload.MarshalJSON, so this is a defensive guard.
var errPayloadTooLarge = errors.New("webpush: payload exceeds 3993-byte plaintext budget")

// encryptedMessage is the complete RFC 8291 / RFC 8188 encrypted body: the
// 86-byte header followed by the AES-128-GCM ciphertext (including the tag).
type encryptedMessage struct {
	header     [headerLen]byte
	ciphertext []byte
}

// encryptPayload encrypts the plaintext for a subscription using the RFC 8291
// aes128gcm content encoding. It generates a fresh ephemeral ECDH key pair and
// salt per message, derives the content encryption key and nonce via HKDF, and
// produces the single-record RFC 8188 body.
//
// The subscription's p256dh and auth fields are base64url-encoded; both padded
// and unpadded forms are accepted because browsers and push services are not
// consistent (research #174 §1.2, edge cases).
func encryptPayload(sub domain.PushSubscription, plaintext []byte) (*encryptedMessage, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	asPrivate, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	return encryptPayloadWithKey(sub, plaintext, asPrivate, salt)
}

// encryptPayloadWithKey encrypts using an injected ephemeral private key and
// salt. It backs both the production path (random key + salt) and the
// deterministic RFC 8291 Appendix A test vectors.
func encryptPayloadWithKey(sub domain.PushSubscription, plaintext []byte, asPrivate *ecdh.PrivateKey, salt []byte) (*encryptedMessage, error) {
	if len(plaintext) > maxPlaintextLen {
		return nil, errPayloadTooLarge
	}

	uaPublic, err := decodeP256PublicKey(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("decode p256dh: %w", err)
	}
	authSecret, err := base64urlDecode(sub.Auth)
	if err != nil {
		return nil, fmt.Errorf("decode auth: %w", err)
	}
	asPublic := asPrivate.PublicKey()

	ecdhSecret, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, fmt.Errorf("ecdh shared secret: %w", err)
	}

	cek, nonce, err := deriveKeys(ecdhSecret, authSecret, uaPublic.Bytes(), asPublic.Bytes(), salt)
	if err != nil {
		return nil, err
	}

	ciphertext, err := aes128gcmSeal(cek, nonce, plaintext)
	if err != nil {
		return nil, err
	}

	return buildMessage(salt, asPublic.Bytes(), ciphertext), nil
}

// deriveKeys computes the RFC 8291 §2.2 / §3.4 content encryption key (CEK, 16
// bytes) and nonce (12 bytes) from the ECDH shared secret, the auth secret, the
// two uncompressed public keys, and the per-message salt. The key_info IKM
// construction (step 1) is a single HMAC — not standard HKDF — so it is computed
// with crypto/hmac to match the RFC byte-for-byte; the final CEK/nonce
// derivation (step 3) is standard HKDF Extract+Expand.
func deriveKeys(ecdhSecret, authSecret, uaPublic, asPublic, salt []byte) (cek, nonce []byte, err error) {
	// Step 1: IKM = HMAC(PRK_key, key_info || 0x01), PRK_key = HMAC(auth, secret).
	// key_info = "WebPush: info" || 0x00 || ua_public || as_public.
	prkKey := hmacSHA256(authSecret, ecdhSecret)
	keyInfo := buildKeyInfo(uaPublic, asPublic)
	ikmInput := make([]byte, 0, len(keyInfo)+1)
	ikmInput = append(ikmInput, keyInfo...)
	ikmInput = append(ikmInput, 0x01)
	ikm := hmacSHA256(prkKey, ikmInput)

	// Step 2: PRK = HKDF-Extract(salt, IKM).
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf extract: %w", err)
	}

	// Step 3: CEK and NONCE via HKDF-Expand from the same PRK with distinct info.
	// HKDF-Expand (RFC 5869) appends its own counter byte (0x01 for the first
	// block), so the info passed here excludes it; this reproduces the RFC 8291
	// §2.2 single-block HMAC: HMAC(PRK, "Content-Encoding: aes128gcm" || 0x00 || 0x01).
	cek, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf expand cek: %w", err)
	}
	nonce, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, fmt.Errorf("hkdf expand nonce: %w", err)
	}
	return cek, nonce, nil
}

// buildKeyInfo assembles the RFC 8291 §2.2 key_info context:
// "WebPush: info" || 0x00 || ua_public || as_public.
func buildKeyInfo(uaPublic, asPublic []byte) []byte {
	const info = "WebPush: info"
	out := make([]byte, 0, len(info)+1+len(uaPublic)+len(asPublic))
	out = append(out, info...)
	out = append(out, 0x00)
	out = append(out, uaPublic...)
	out = append(out, asPublic...)
	return out
}

// aes128gcmSeal encrypts the plaintext with AES-128-GCM, appending the RFC 8291
// padding delimiter (0x02) and zero padding so the plaintext+padding fills the
// single record. The returned slice includes the 16-byte GCM authentication tag.
func aes128gcmSeal(cek, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}

	// Padding: delimiter 0x02 marks the final (only) record (RFC 8188 §2). No
	// additional zero padding is needed for a single record whose plaintext
	// already fits — the delimiter alone terminates it.
	padded := make([]byte, len(plaintext)+1)
	copy(padded, plaintext)
	padded[len(plaintext)] = 0x02

	return gcm.Seal(nil, nonce, padded, nil), nil
}

// buildMessage assembles the RFC 8188 content-coding header and the ciphertext
// into the final body: salt(16) || rs(4) || idlen(1) || key(65) || ciphertext.
func buildMessage(salt, asPublic, ciphertext []byte) *encryptedMessage {
	var m encryptedMessage
	copy(m.header[0:16], salt)
	binary.BigEndian.PutUint32(m.header[16:20], recordSize)
	// asPublic is always an uncompressed P-256 key, so the idlen octet is the
	// key size constant by construction.
	m.header[20] = p256UncompressedSize
	copy(m.header[21:21+len(asPublic)], asPublic)
	m.ciphertext = ciphertext
	return &m
}

// Bytes returns the serialized RFC 8188 body (header + ciphertext).
func (m *encryptedMessage) Bytes() []byte {
	out := make([]byte, 0, headerLen+len(m.ciphertext))
	out = append(out, m.header[:]...)
	out = append(out, m.ciphertext...)
	return out
}

// hmacSHA256 returns HMAC-SHA256(key, data).
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	// hash.Hash documents that Write never returns an error; the discard is
	// explicit rather than silent.
	_, _ = h.Write(data)
	return h.Sum(nil)
}

// decodeP256PublicKey decodes a base64url-encoded uncompressed P-256 public key
// (65 bytes, 0x04 prefix) into an *ecdh.PublicKey.
func decodeP256PublicKey(b64 string) (*ecdh.PublicKey, error) {
	raw, err := base64urlDecode(b64)
	if err != nil {
		return nil, err
	}
	return ecdh.P256().NewPublicKey(raw)
}

// base64urlDecode decodes a base64url string, accepting both padded (with '=')
// and unpadded forms. Browsers and push services are inconsistent about padding
// (research #174 §1.2, edge cases), so both are tolerated.
func base64urlDecode(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("webpush: empty base64url input")
	}
	// Pad to a multiple of 4 if needed; RawURLEncoding does not require it, but
	// standard URLEncoding may have been used by the producer.
	if pad := len(s) % 4; pad != 0 {
		s += "=="[:4-pad]
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	// Fall back to raw (unpadded) URL encoding.
	return base64.RawURLEncoding.DecodeString(s)
}
