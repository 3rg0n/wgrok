import { gunzipSync, gzipSync } from 'node:zlib';

export function compress(data: string): string {
  const compressed = gzipSync(Buffer.from(data, 'utf-8'));
  return compressed.toString('base64');
}

export function decompress(data: string): string {
  const compressed = Buffer.from(data, 'base64');
  return gunzipSync(compressed).toString('utf-8');
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
