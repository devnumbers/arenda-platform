// Crypto helpers of the frontend e2e seed, mirroring the backend's
// internal/platform/encryption so SQL-seeded rows are indistinguishable from
// rows written by the app:
//
//   node e2e-crypto.mjs hash-token <64-hex-key> <raw-token>
//     → HMAC-SHA256(token, key) hex — sessions.token_hash (hashToken).
//
//   node e2e-crypto.mjs det-phone <64-hex-key> <phone>
//     → deterministic phone ciphertext (base64) — users.phone
//       (DeterministicEncrypt): HKDF-derived encKey/macKey, HMAC-SHA256 nonce
//       with the "\x00phone" domain separator, AES-256-GCM over the phone.
import { createCipheriv, createHmac, hkdfSync } from 'node:crypto';

const [mode, keyHex, value] = process.argv.slice(2);

if (!/^[0-9a-fA-F]{64}$/.test(keyHex ?? '') || value === undefined || value === '') {
  console.error('Usage: node e2e-crypto.mjs <hash-token|det-phone> <64-hex-key> <value>');
  process.exit(1);
}

const key = Buffer.from(keyHex, 'hex');

// x/crypto/hkdf treats a nil salt as HashLen zero bytes (RFC 5869).
function deriveKey(label) {
  return Buffer.from(hkdfSync('sha256', key, Buffer.alloc(32), Buffer.from(label), 32));
}

if (mode === 'hash-token') {
  console.log(createHmac('sha256', key).update(value).digest('hex'));
} else if (mode === 'det-phone') {
  const macKey = deriveKey('macKey');
  const nonce = createHmac('sha256', macKey)
    .update(value)
    .update('\x00phone')
    .digest()
    .subarray(0, 12);
  const cipher = createCipheriv('aes-256-gcm', deriveKey('encKey'), nonce);
  const sealed = Buffer.concat([cipher.update(value, 'utf8'), cipher.final(), cipher.getAuthTag()]);
  console.log(Buffer.concat([nonce, sealed]).toString('base64'));
} else {
  console.error(`unknown mode ${mode}: use hash-token or det-phone`);
  process.exit(1);
}
