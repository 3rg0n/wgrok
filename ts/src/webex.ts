export const WEBEX_API_BASE = 'https://webexapis.com/v1';
export let WEBEX_MESSAGES_URL = `${WEBEX_API_BASE}/messages`;
export let WEBEX_ATTACHMENT_ACTIONS_URL = `${WEBEX_API_BASE}/attachment/actions`;
export const ADAPTIVE_CARD_CONTENT_TYPE = 'application/vnd.microsoft.card.adaptive';

const MAX_RETRIES = 3;

let sleepFn = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

function sleep(ms: number): Promise<void> {
  return sleepFn(ms);
}

interface CardAttachment {
  contentType: string;
  content: unknown;
}

interface SendMessagePayload {
  roomId?: string;
  toPersonEmail?: string;
  text: string;
  attachments?: CardAttachment[];
}

export async function sendMessage(
  token: string,
  toEmail: string,
  text: string,
  fetchFn: typeof fetch = globalThis.fetch,
  roomId = '',
): Promise<Record<string, unknown>> {
  const payload: SendMessagePayload = roomId
    ? { roomId, text }
    : { toPersonEmail: toEmail, text };
  return postMessage(token, payload, fetchFn);
}

export async function sendCard(
  token: string,
  toEmail: string,
  text: string,
  card: unknown,
  fetchFn: typeof fetch = globalThis.fetch,
  roomId = '',
): Promise<Record<string, unknown>> {
  const payload: SendMessagePayload = roomId
    ? {
        roomId,
        text,
        attachments: [{ contentType: ADAPTIVE_CARD_CONTENT_TYPE, content: card }],
      }
    : {
        toPersonEmail: toEmail,
        text,
        attachments: [{ contentType: ADAPTIVE_CARD_CONTENT_TYPE, content: card }],
      };
  return postMessage(token, payload, fetchFn);
}

async function postMessage(
  token: string,
  payload: SendMessagePayload,
  fetchFn: typeof fetch,
): Promise<Record<string, unknown>> {
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    const resp = await fetchFn(WEBEX_MESSAGES_URL, {
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

export async function getMessage(
  token: string,
  messageId: string,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  return getJSON(token, `${WEBEX_MESSAGES_URL}/${messageId}`, fetchFn);
}

export async function getAttachmentAction(
  token: string,
  actionId: string,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  return getJSON(token, `${WEBEX_ATTACHMENT_ACTIONS_URL}/${actionId}`, fetchFn);
}

async function getJSON(
  token: string,
  url: string,
  fetchFn: typeof fetch,
): Promise<Record<string, unknown>> {
  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    const resp = await fetchFn(url, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
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
  throw new Error('Unexpected state in getJSON retry loop');
}

export function extractCards(message: Record<string, unknown>): unknown[] {
  const attachments = message.attachments;
  if (!Array.isArray(attachments)) return [];
  const cards: unknown[] = [];
  for (const att of attachments) {
    if (att && typeof att === 'object' && 'contentType' in att && 'content' in att) {
      if ((att as Record<string, unknown>).contentType === ADAPTIVE_CARD_CONTENT_TYPE) {
        cards.push((att as Record<string, unknown>).content);
      }
    }
  }
  return cards;
}

/** Override message URL for testing */
export function _setMessagesUrl(url: string): void {
  WEBEX_MESSAGES_URL = url;
}

export function _setAttachmentActionsUrl(url: string): void {
  WEBEX_ATTACHMENT_ACTIONS_URL = url;
}

/** Override sleep function for testing */
export function _setSleepFn(fn: (ms: number) => Promise<void>): void {
  sleepFn = fn;
}
