export const WEBEX_API_BASE = 'https://webexapis.com/v1';
export let WEBEX_MESSAGES_URL = `${WEBEX_API_BASE}/messages`;
export let WEBEX_ATTACHMENT_ACTIONS_URL = `${WEBEX_API_BASE}/attachment/actions`;
export const ADAPTIVE_CARD_CONTENT_TYPE = 'application/vnd.microsoft.card.adaptive';

interface CardAttachment {
  contentType: string;
  content: unknown;
}

interface SendMessagePayload {
  toPersonEmail: string;
  text: string;
  attachments?: CardAttachment[];
}

export async function sendMessage(
  token: string,
  toEmail: string,
  text: string,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const payload: SendMessagePayload = { toPersonEmail: toEmail, text };
  return postMessage(token, payload, fetchFn);
}

export async function sendCard(
  token: string,
  toEmail: string,
  text: string,
  card: unknown,
  fetchFn: typeof fetch = globalThis.fetch,
): Promise<Record<string, unknown>> {
  const payload: SendMessagePayload = {
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
  const resp = await fetchFn(WEBEX_MESSAGES_URL, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  if (!resp.ok) {
    const body = await resp.text();
    throw new Error(`HTTP ${resp.status}: ${body}`);
  }
  return (await resp.json()) as Record<string, unknown>;
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
  const resp = await fetchFn(url, {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  });
  if (!resp.ok) {
    const body = await resp.text();
    throw new Error(`HTTP ${resp.status}: ${body}`);
  }
  return (await resp.json()) as Record<string, unknown>;
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
