import {
  sendMessage,
  sendCard,
  getMessage,
  getAttachmentAction,
  ADAPTIVE_CARD_CONTENT_TYPE,
} from '../src/webex';

function mockFetch(status: number, body: Record<string, unknown>): typeof fetch {
  return (async (_url: string | URL | Request, _init?: RequestInit) => ({
    ok: status >= 200 && status < 300,
    status,
    text: async () => JSON.stringify(body),
    json: async () => body,
  })) as unknown as typeof fetch;
}

function capturingFetch(
  status: number,
  body: Record<string, unknown>,
): { fetchFn: typeof fetch; captured: { url: string; init: RequestInit }[] } {
  const captured: { url: string; init: RequestInit }[] = [];
  const fetchFn = (async (url: string | URL | Request, init?: RequestInit) => {
    captured.push({ url: url as string, init: init! });
    return {
      ok: status >= 200 && status < 300,
      status,
      text: async () => JSON.stringify(body),
      json: async () => body,
    };
  }) as unknown as typeof fetch;
  return { fetchFn, captured };
}

describe('sendMessage HTTP', () => {
  it('sends correct payload and headers', async () => {
    const { fetchFn, captured } = capturingFetch(200, { id: 'msg-1' });
    const result = await sendMessage('tok123', 'user@example.com', 'hello', fetchFn);

    expect(result).toEqual({ id: 'msg-1' });
    expect(captured.length).toBe(1);
    const body = JSON.parse(captured[0].init.body as string);
    expect(body.toPersonEmail).toBe('user@example.com');
    expect(body.text).toBe('hello');
    expect(captured[0].init.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer tok123' }),
    );
  });

  it('throws on HTTP error', async () => {
    const fetchFn = mockFetch(401, { message: 'unauthorized' });
    await expect(sendMessage('badtoken', 'user@example.com', 'hello', fetchFn)).rejects.toThrow(
      '401',
    );
  });
});

describe('sendCard HTTP', () => {
  it('sends card attachment', async () => {
    const card = { type: 'AdaptiveCard', body: [{ type: 'TextBlock', text: 'Hello' }] };
    const { fetchFn, captured } = capturingFetch(200, { id: 'card-1' });
    const result = await sendCard('tok', 'user@x.com', 'fallback', card, fetchFn);

    expect(result).toEqual({ id: 'card-1' });
    const body = JSON.parse(captured[0].init.body as string);
    expect(body.text).toBe('fallback');
    expect(body.toPersonEmail).toBe('user@x.com');
    expect(body.attachments).toHaveLength(1);
    expect(body.attachments[0].contentType).toBe(ADAPTIVE_CARD_CONTENT_TYPE);
    expect(body.attachments[0].content).toEqual(card);
  });
});

describe('getMessage HTTP', () => {
  it('fetches message by ID', async () => {
    const msgData = { id: 'msg-1', text: 'hello', attachments: [] };
    const fetchFn = mockFetch(200, msgData);
    const result = await getMessage('tok', 'msg-1', fetchFn);
    expect(result).toEqual(msgData);
  });

  it('throws on not found', async () => {
    const fetchFn = mockFetch(404, {});
    await expect(getMessage('tok', 'bad-id', fetchFn)).rejects.toThrow('404');
  });
});

describe('getAttachmentAction HTTP', () => {
  it('fetches action by ID', async () => {
    const actionData = { id: 'act-1', type: 'submit', inputs: { name: 'test' } };
    const fetchFn = mockFetch(200, actionData);
    const result = await getAttachmentAction('tok', 'act-1', fetchFn);
    expect(result).toEqual(actionData);
  });
});
