import { describe, expect, it } from 'vitest';
import { base64UrlToUint8Array } from './base64url';

describe('base64UrlToUint8Array', () => {
    it('decodes a canonical base64 string (padding tolerant)', () => {
        // "Hello" in base64
        const bytes = base64UrlToUint8Array('SGVsbG8=');
        expect(Array.from(bytes)).toEqual([72, 101, 108, 108, 111]);
    });

    it('decodes a base64url string with - and _ (RFC 4648 §5)', () => {
        // Bytes 0xFB 0xFF 0xBF encode to base64 "+/+/" which in the URL-safe
        // alphabet becomes "-_-_". Build the raw bytes directly and encode to
        // base64url so the assertion stays portable across environments.
        const raw = Uint8Array.of(0xfb, 0xff, 0xbf);
        const canonical = Buffer.from(raw).toString('base64');
        const urlSafe = canonical.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
        // Sanity: the URL-safe alphabet must have been applied.
        expect(urlSafe).not.toContain('+');
        expect(urlSafe).not.toContain('/');
        expect(urlSafe).not.toContain('=');

        const decoded = base64UrlToUint8Array(urlSafe);
        expect(Array.from(decoded)).toEqual([0xfb, 0xff, 0xbf]);
    });

    it('decodes a realistic 65-byte VAPID P-256 public key', () => {
        // A 65-byte uncompressed P-256 point (0x04 || X || Y). Encoded as
        // base64url without padding it is 87 characters.
        const key = new Uint8Array(65);
        for (let i = 0; i < key.length; i += 1) {
            key[i] = (i * 7 + 3) % 256;
        }
        const b64url = Buffer.from(key)
            .toString('base64')
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=+$/, '');

        const decoded = base64UrlToUint8Array(b64url);
        expect(decoded.length).toBe(65);
        expect(Array.from(decoded)).toEqual(Array.from(key));
    });

    it('decodes an empty string to an empty Uint8Array', () => {
        const decoded = base64UrlToUint8Array('');
        expect(decoded.length).toBe(0);
    });

    it('round-trips arbitrary byte sequences through base64url', () => {
        const cases = [
            Uint8Array.of(0),
            Uint8Array.of(1, 2),
            Uint8Array.of(1, 2, 3),
            Uint8Array.of(255, 0, 128, 64, 32),
        ];
        for (const raw of cases) {
            const b64url = Buffer.from(raw)
                .toString('base64')
                .replace(/\+/g, '-')
                .replace(/\//g, '_')
                .replace(/=+$/, '');
            const decoded = base64UrlToUint8Array(b64url);
            expect(Array.from(decoded), `round-trip for [${Array.from(raw).join(',')}]`).toEqual(
                Array.from(raw),
            );
        }
    });
});
