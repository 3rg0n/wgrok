import { gunzipSync, gzipSync } from 'node:zlib';
import { createCipheriv, createDecipheriv, randomBytes } from 'node:crypto';

export function compress(data: string): string {
  const compressed = gzipSync(Buffer.from(data, 'utf-8'));
  return compressed.toString('base64');
}

export function decompress(data: string): string {
  const compressed = Buffer.from(data, 'base64');
  return gunzipSync(compressed).toString('utf-8');
}

export function encrypt(data: string, key: Buffer): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, iv);
  const encrypted = Buffer.concat([cipher.update(data, 'utf-8'), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([iv, encrypted, tag]).toString('base64');
}

export function decrypt(data: string, key: Buffer): string {
  const raw = Buffer.from(data, 'base64');
  const iv = raw.subarray(0, 12);
  const tag = raw.subarray(raw.length - 16);
  const ciphertext = raw.subarray(12, raw.length - 16);
  const decipher = createDecipheriv('aes-256-gcm', key, iv);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString('utf-8');
}

export function chunk(data: string, maxSize: number): string[] {
  if (maxSize <= 0) {
    throw new Error('maxSize must be positive');
  }
  let total = Math.ceil(data.length / maxSize);
  if (total === 0) total = 1;
  const chunks: string[] = [];
  for (let i = 0; i < total; i++) {
    const start = i * maxSize;
    const end = start + maxSize;
    const chunkData = data.slice(start, end);
    chunks.push(chunkData);
  }
  return chunks;
}
