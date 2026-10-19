// Crypto helpers of the frontend e2e seed, mirroring the backend's
// internal/platform/encryption so SQL-seeded rows are indistinguishable from
// rows written by the app:
//
//   node e2e-crypto.mjs hash-token <64-hex-key> <raw-token>
//     → HMAC-SHA256(token, key) hex — sessions.token_hash (hashToken).
import { createHmac } from 'node:crypto';

const [mode, keyHex, value] = process.argv.slice(2);

if (!/^[0-9a-fA-F]{64}$/.test(keyHex ?? '') || value === undefined || value === '') {
  console.error('Usage: node e2e-crypto.mjs hash-token <64-hex-key> <value>');
  process.exit(1);
}

const key = Buffer.from(keyHex, 'hex');

if (mode === 'hash-token') {
  console.log(createHmac('sha256', key).update(value).digest('hex'));
} else {
  console.error(`unknown mode ${mode}: use hash-token`);
  process.exit(1);
}
