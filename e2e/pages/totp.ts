import { createHmac } from 'crypto';

// Decodes an RFC 4648 base32 string (no padding) into raw bytes.
function base32Decode(input: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const clean = input.toUpperCase().replace(/[^A-Z2-7]/g, '');

  let bits = '';
  for (const char of clean) {
    const value = alphabet.indexOf(char);
    if (value === -1) continue;
    bits += value.toString(2).padStart(5, '0');
  }

  const bytes: number[] = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) {
    bytes.push(parseInt(bits.slice(i, i + 8), 2));
  }
  return Buffer.from(bytes);
}

// Generates the current 6-digit TOTP code for a base32 secret, matching the
// server's fixed config (internal/core/auth/totp_service.go): 30s period,
// SHA-1, 6 digits. `at` lets tests generate a code for a specific instant
// (e.g. one period in the past) to exercise clock-skew tolerance.
export function generateTotpCode(base32Secret: string, at: number = Date.now()): string {
  const key = base32Decode(base32Secret);
  const counter = Math.floor(at / 1000 / 30);

  const counterBuffer = Buffer.alloc(8);
  counterBuffer.writeBigUInt64BE(BigInt(counter));

  const hmac = createHmac('sha1', key).update(counterBuffer).digest();
  const offset = hmac[hmac.length - 1] & 0x0f;
  const binCode =
    ((hmac[offset] & 0x7f) << 24) |
    ((hmac[offset + 1] & 0xff) << 16) |
    ((hmac[offset + 2] & 0xff) << 8) |
    (hmac[offset + 3] & 0xff);

  return (binCode % 1_000_000).toString().padStart(6, '0');
}
