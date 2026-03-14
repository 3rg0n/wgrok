export const ECHO_PREFIX = './echo:';

export function formatEcho(slug: string, payload: string): string {
  return ECHO_PREFIX + slug + ':' + payload;
}

export function parseEcho(text: string): { slug: string; payload: string } {
  if (!isEcho(text)) {
    throw new Error(`Not an echo message: "${text}"`);
  }
  const stripped = text.slice(ECHO_PREFIX.length);
  const idx = stripped.indexOf(':');
  const slug = idx < 0 ? stripped : stripped.slice(0, idx);
  const payload = idx < 0 ? '' : stripped.slice(idx + 1);
  if (!slug) {
    throw new Error(`Empty slug in echo message: "${text}"`);
  }
  return { slug, payload };
}

export function isEcho(text: string): boolean {
  return text.startsWith(ECHO_PREFIX);
}

export function formatResponse(slug: string, payload: string): string {
  return slug + ':' + payload;
}

export function parseResponse(text: string): { slug: string; payload: string } {
  const idx = text.indexOf(':');
  const slug = idx < 0 ? text : text.slice(0, idx);
  const payload = idx < 0 ? '' : text.slice(idx + 1);
  if (!slug) {
    throw new Error('Empty slug in response message');
  }
  return { slug, payload };
}
