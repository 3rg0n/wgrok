export const ECHO_PREFIX = './echo:';
export const PAUSE_CMD = './pause';
export const RESUME_CMD = './resume';

const SPARK_MENTION_RE = /<spark-mention[^>]*>([^<]+)<\/spark-mention>/;

export function stripBotMention(text: string, html: string | null): string {
  if (!html) {
    return text;
  }
  const match = SPARK_MENTION_RE.exec(html);
  if (!match) {
    return text;
  }
  const displayName = match[1];
  if (text.startsWith(displayName)) {
    return text.slice(displayName.length).trimStart();
  }
  return text;
}

export function isPause(text: string): boolean {
  return text.trim() === PAUSE_CMD;
}

export function isResume(text: string): boolean {
  return text.trim() === RESUME_CMD;
}

export function formatEcho(to: string, from: string, flags: string, payload: string): string {
  return ECHO_PREFIX + to + ':' + from + ':' + flags + ':' + payload;
}

export function parseEcho(text: string): { to: string; from: string; flags: string; payload: string } {
  if (!isEcho(text)) {
    throw new Error(`Not an echo message: "${text}"`);
  }
  const stripped = text.slice(ECHO_PREFIX.length);
  const i1 = stripped.indexOf(':');
  const i2 = stripped.indexOf(':', i1 + 1);
  const i3 = stripped.indexOf(':', i2 + 1);

  const to = i1 < 0 ? stripped : stripped.slice(0, i1);
  const from = i1 < 0 ? '' : i2 < 0 ? stripped.slice(i1 + 1) : stripped.slice(i1 + 1, i2);
  const flags = i2 < 0 ? '' : i3 < 0 ? stripped.slice(i2 + 1) : stripped.slice(i2 + 1, i3);
  const payload = i3 < 0 ? '' : stripped.slice(i3 + 1);

  if (!to) {
    throw new Error(`Empty to in echo message: "${text}"`);
  }
  return { to, from, flags, payload };
}

export function isEcho(text: string): boolean {
  return text.startsWith(ECHO_PREFIX);
}

export function formatResponse(to: string, from: string, flags: string, payload: string): string {
  return to + ':' + from + ':' + flags + ':' + payload;
}

export function parseResponse(text: string): { to: string; from: string; flags: string; payload: string } {
  const i1 = text.indexOf(':');
  const i2 = text.indexOf(':', i1 + 1);
  const i3 = text.indexOf(':', i2 + 1);

  const to = i1 < 0 ? text : text.slice(0, i1);
  const from = i1 < 0 ? '' : i2 < 0 ? text.slice(i1 + 1) : text.slice(i1 + 1, i2);
  const flags = i2 < 0 ? '' : i3 < 0 ? text.slice(i2 + 1) : text.slice(i2 + 1, i3);
  const payload = i3 < 0 ? '' : text.slice(i3 + 1);

  if (!to) {
    throw new Error('Empty to in response message');
  }
  return { to, from, flags, payload };
}

export function parseFlags(flags: string): { compressed: boolean; encrypted: boolean; chunkSeq: number | null; chunkTotal: number | null } {
  if (flags === '-') {
    return { compressed: false, encrypted: false, chunkSeq: null, chunkTotal: null };
  }

  let compressed = false;
  let encrypted = false;
  let remaining = flags;

  if (flags.startsWith('z')) {
    compressed = true;
    remaining = flags.slice(1);
  }

  if (remaining.startsWith('e')) {
    encrypted = true;
    remaining = remaining.slice(1);
  }

  if (remaining === '') {
    return { compressed, encrypted, chunkSeq: null, chunkTotal: null };
  }

  const slashIdx = remaining.indexOf('/');
  if (slashIdx < 0) {
    throw new Error(`Invalid flags format: "${flags}"`);
  }

  const seq = parseInt(remaining.slice(0, slashIdx), 10);
  const total = parseInt(remaining.slice(slashIdx + 1), 10);

  if (isNaN(seq) || isNaN(total) || seq < 1 || total < 1) {
    throw new Error(`Invalid chunk numbers in flags: "${flags}"`);
  }

  return { compressed, encrypted, chunkSeq: seq, chunkTotal: total };
}

export function formatFlags(compressed: boolean, encrypted: boolean = false, chunkSeq: number | null = null, chunkTotal: number | null = null): string {
  let result = '';

  if (compressed) {
    result += 'z';
  }

  if (encrypted) {
    result += 'e';
  }

  if (chunkSeq !== null && chunkTotal !== null) {
    result += `${chunkSeq}/${chunkTotal}`;
  } else if (result === '') {
    result = '-';
  }

  return result;
}
