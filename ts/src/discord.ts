export const DISCORD_API_BASE = 'https://discord.com/api/v10';

const MAX_RETRIES = 3;

let sleepFn = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

function sleep(ms: number): Promise<void> {
  return sleepFn(ms);
}

function getMessagesUrl(channelId: string): string {
  return `${DISCORD_API_BASE}/channels/${channelId}/messages`;
}

interface DiscordMessagePayload {
  content: string;
  embeds?: unknown[];
}

export async function sendDiscordMessage(
  token: string,
  channelId: string,
  text: string,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const url = getMessagesUrl(channelId);
  const payload: DiscordMessagePayload = { content: text };
  return postMessage(token, url, payload, fetchFn);
}

export async function sendDiscordCard(
  token: string,
  channelId: string,
  text: string,
  card: unknown,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const url = getMessagesUrl(channelId);
  const embeds = Array.isArray(card) ? card : [card];
  const payload: DiscordMessagePayload = { content: text, embeds };
  return postMessage(token, url, payload, fetchFn);
}

async function postMessage(
  token: string,
  url: string,
  payload: DiscordMessagePayload,
  fetchFn: typeof fetch,
): Promise<Record<string, unknown>> {
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    const resp = await fetchFn(url, {
      method: 'POST',
      headers: {
        Authorization: `Bot ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    if (resp.status === 429) {
      if (attempt < MAX_RETRIES) {
        const retryAfter = resp.headers.get('Retry-After') || '1';
        const delaySecs = parseInt(retryAfter, 10) || 1;
        await sleep(delaySecs * 1000);
        continue;
      }
    }

    if (!resp.ok) {
      const body = await resp.text();
      throw new Error(`HTTP ${resp.status}: ${body}`);
    }
    return (await resp.json()) as Record<string, unknown>;
  }

  // This should be unreachable, but TypeScript needs it
  throw new Error('Unexpected state in postMessage retry loop');
}

/** Override sleep function for testing */
export function _setSleepFn(fn: (ms: number) => Promise<void>): void {
  sleepFn = fn;
}
