package webpush

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"encoding/base64"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// aes128gcmOpen reverses aes128gcmSeal; test-only helper to verify round-trip.
func aes128gcmOpen(cek, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// RFC 8291 Appendix A test vectors (base64url, no padding). Sourced verbatim
// from the RFC; the implementation must reproduce the exact ciphertext.
const (
	rfc8291UAPublic  = "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"
	rfc8291UAPrivate = "q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"
	rfc8291Auth      = "BTBZMqHH6r4Tts7J_aSIgg"
	rfc8291ASPublic  = "BP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A8"
	rfc8291ASPrivate = "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"
	rfc8291Salt      = "DGv6ra1nlYgDCS1FRnbzlw"

	// The rfc8291Plaintext value is "When I grow up, I want to be a watermelon".
	rfc8291Plaintext = "When I grow up, I want to be a watermelon"

	// The rfc8291Ciphertext value is the complete RFC 8188 body
	// (86-byte header + ciphertext + tag), split to fit the line limit.
	rfc8291Ciphertext = "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6Tlz" +
		"AC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
)

func TestEncryptPayload_RFC8291TestVector(t *testing.T) {
	salt, err := base64.RawURLEncoding.DecodeString(rfc8291Salt)
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	asPrivateRaw, err := base64.RawURLEncoding.DecodeString(rfc8291ASPrivate)
	if err != nil {
		t.Fatalf("decode as private: %v", err)
	}
	asPrivate, err := ecdh.P256().NewPrivateKey(asPrivateRaw)
	if err != nil {
		t.Fatalf("parse as private key: %v", err)
	}

	sub := domain.PushSubscription{
		P256dh: rfc8291UAPublic,
		Auth:   rfc8291Auth,
	}

	msg, err := encryptPayloadWithKey(sub, []byte(rfc8291Plaintext), asPrivate, salt)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	expected, err := base64.RawURLEncoding.DecodeString(rfc8291Ciphertext)
	if err != nil {
		t.Fatalf("decode expected ciphertext: %v", err)
	}

	got := msg.Bytes()
	if !bytes.Equal(got, expected) {
		t.Errorf("ciphertext does not match RFC 8291 Appendix A test vector\n got (len=%d): %x\nwant (len=%d): %x",
			len(got), got, len(expected), expected)
	}
}

func TestDeriveKeys_RFC8291IntermediateValues(t *testing.T) {
	// RFC 8291 Appendix A intermediate values (hex), used to pin each step of
	// the derivation independently so a regression is easy to localise.
	const (
		// The rfc8291ECDHOutput value is the hex of the ECDH shared secret
		// from the RFC appendix; named without "Secret" so gosec G101 does not
		// misread the published test vector as a hardcoded credential.
		rfc8291ECDHOutput = "932acbd63208387133837b0cd995911c3441eb66000998614a592727aef6912b"
		rfc8291IKM        = "4b895831bfcbd05c427aad16843c7cd772a0498a94dba90ecb359476c5d8cab8"
		rfc8291CEK        = "a088555b4e0c45dcb65cdf4288a2f14e"
		rfc8291Nonce      = "e21ffde6495727913faa7a0d"
	)

	uaPrivateRaw, err := base64.RawURLEncoding.DecodeString(rfc8291UAPrivate)
	if err != nil {
		t.Fatalf("decode ua private: %v", err)
	}
	uaPrivate, err := ecdh.P256().NewPrivateKey(uaPrivateRaw)
	if err != nil {
		t.Fatalf("parse ua private: %v", err)
	}
	asPrivateRaw, err := base64.RawURLEncoding.DecodeString(rfc8291ASPrivate)
	if err != nil {
		t.Fatalf("decode as private: %v", err)
	}
	asPrivate, err := ecdh.P256().NewPrivateKey(asPrivateRaw)
	if err != nil {
		t.Fatalf("parse as private: %v", err)
	}

	// ECDH shared secret.
	shared, err := uaPrivate.ECDH(asPrivate.PublicKey())
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	if hexEncode(shared) != rfc8291ECDHOutput {
		t.Errorf("ECDH shared secret\n got %s\nwant %s", hexEncode(shared), rfc8291ECDHOutput)
	}

	// IKM = HMAC(PRK_key, key_info || 0x01).
	auth, err := base64urlDecode(rfc8291Auth)
	if err != nil {
		t.Fatalf("decode auth: %v", err)
	}
	prkKey, err := hmacSHA256(auth, shared)
	if err != nil {
		t.Fatalf("hmac prf key: %v", err)
	}
	keyInfo := buildKeyInfo(uaPrivate.PublicKey().Bytes(), asPrivate.PublicKey().Bytes())
	ikmInput := make([]byte, 0, len(keyInfo)+1)
	ikmInput = append(ikmInput, keyInfo...)
	ikmInput = append(ikmInput, 0x01)
	ikm, err := hmacSHA256(prkKey, ikmInput)
	if err != nil {
		t.Fatalf("hmac ikm: %v", err)
	}
	if hexEncode(ikm) != rfc8291IKM {
		t.Errorf("IKM\n got %s\nwant %s", hexEncode(ikm), rfc8291IKM)
	}

	// CEK and nonce via the full deriveKeys path.
	salt, err := base64.RawURLEncoding.DecodeString(rfc8291Salt)
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	cek, nonce, err := deriveKeys(shared, auth, uaPrivate.PublicKey().Bytes(), asPrivate.PublicKey().Bytes(), salt)
	if err != nil {
		t.Fatalf("derive keys: %v", err)
	}
	if hexEncode(cek) != rfc8291CEK {
		t.Errorf("CEK\n got %s\nwant %s", hexEncode(cek), rfc8291CEK)
	}
	if hexEncode(nonce) != rfc8291Nonce {
		t.Errorf("nonce\n got %s\nwant %s", hexEncode(nonce), rfc8291Nonce)
	}
}

// hexEncode returns the lowercase hex encoding of b.
func hexEncode(b []byte) string {
	const hexdigit = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[2*i] = hexdigit[v>>4]
		out[2*i+1] = hexdigit[v&0xf]
	}
	return string(out)
}

func TestEncryptPayload_RoundTrip(t *testing.T) {
	// Generate a fresh subscription key pair to exercise the full path and
	// verify we can decrypt our own output with AES-128-GCM.
	uaPrivate, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ua key: %v", err)
	}
	uaPublic := uaPrivate.PublicKey()
	auth := make([]byte, 16)
	sub := domain.PushSubscription{
		P256dh: base64.RawURLEncoding.EncodeToString(uaPublic.Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(auth),
	}

	plaintext := []byte(`{"title":"Test","body":"Hello push"}`)
	msg, err := encryptPayload(sub, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	body := msg.Bytes()
	if len(body) <= headerLen {
		t.Fatalf("body too short: %d bytes", len(body))
	}
	if body[20] != p256UncompressedSize {
		t.Errorf("idlen byte = %d, want %d", body[20], p256UncompressedSize)
	}

	// Recover the key material from the header and decrypt to prove round-trip.
	salt := body[0:16]
	asPublic := body[21 : 21+p256UncompressedSize]
	ciphertext := body[21+p256UncompressedSize:]

	asPublicECDH, err := ecdh.P256().NewPublicKey(asPublic)
	if err != nil {
		t.Fatalf("parse as public from header: %v", err)
	}
	ecdhSecret, err := uaPrivate.ECDH(asPublicECDH)
	if err != nil {
		t.Fatalf("ecdh: %v", err)
	}
	cek, nonce, err := deriveKeys(ecdhSecret, auth, uaPublic.Bytes(), asPublic, salt)
	if err != nil {
		t.Fatalf("derive keys: %v", err)
	}

	decrypted, err := aes128gcmOpen(cek, nonce, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	// The aes128gcmOpen helper returns plaintext + padding delimiter (0x02).
	if len(decrypted) < 1 || decrypted[len(decrypted)-1] != 0x02 {
		t.Fatalf("padding delimiter missing: %x", decrypted)
	}
	got := decrypted[:len(decrypted)-1]
	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted plaintext mismatch\n got: %s\nwant: %s", got, plaintext)
	}
}

func TestEncryptPayload_TooLarge(t *testing.T) {
	uaPrivate, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ua key: %v", err)
	}
	sub := domain.PushSubscription{
		P256dh: base64.RawURLEncoding.EncodeToString(uaPrivate.PublicKey().Bytes()),
		Auth:   base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
	}
	oversized := make([]byte, maxPlaintextLen+1)
	if _, err := encryptPayload(sub, oversized); err == nil {
		t.Fatal("expected error for oversized payload, got nil")
	}
}

func TestBase64urlDecode_PaddedAndUnpadded(t *testing.T) {
	// The same 16 bytes as padded and unpadded base64url must both decode.
	raw := []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
	}
	padded := base64.URLEncoding.EncodeToString(raw)
	unpadded := base64.RawURLEncoding.EncodeToString(raw)

	got1, err := base64urlDecode(padded)
	if err != nil {
		t.Fatalf("decode padded: %v", err)
	}
	got2, err := base64urlDecode(unpadded)
	if err != nil {
		t.Fatalf("decode unpadded: %v", err)
	}
	if !bytes.Equal(got1, raw) || !bytes.Equal(got2, raw) {
		t.Errorf("base64url decode mismatch")
	}
}
