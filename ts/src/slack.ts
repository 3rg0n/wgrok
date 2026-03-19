export const SLACK_API_BASE = 'https://slack.com/api';
export let SLACK_POST_MESSAGE_URL = `${SLACK_API_BASE}/chat.postMessage`;

const MAX_RETRIES = 3;

let sleepFn = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

function sleep(ms: number): Promise<void> {
  return sleepFn(ms);
}

interface SlackMessagePayload {
  channel: string;
  text: string;
  blocks?: unknown[];
}

export async function sendSlackMessage(
  token: string,
  channel: string,
  text: string,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const payload: SlackMessagePayload = { channel, text };
  return postMessage(token, payload, fetchFn);
}

export async function sendSlackCard(
  token: string,
  channel: string,
  text: string,
  card: unknown,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const blocks = Array.isArray(card) ? card : [card];
  const payload: SlackMessagePayload = { channel, text, blocks };
  return postMessage(token, payload, fetchFn);
}

async function postMessage(
  token: string,
  payload: SlackMessagePayload,
  fetchFn: typeof fetch,
): Promise<Record<string, unknown>> {
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    const resp = await fetchFn(SLACK_POST_MESSAGE_URL, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });

    if (resp.status === 429) {
      if (attempt < MAX_RETRIES) {
        const retryAfter = resp.headers.get('Retry-After') || '1';
        const delaySecs = Math.min(parseInt(retryAfter, 10) || 1, 300);
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

/** Override Slack URL for testing */
export function _setSlackUrl(url: string): void {
  SLACK_POST_MESSAGE_URL = url;
}

/** Override sleep function for testing */
export function _setSleepFn(fn: (ms: number) => Promise<void>): void {
  sleepFn = fn;
}
