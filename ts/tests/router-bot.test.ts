import { jest } from '@jest/globals';
import type { IncomingMessage } from '../src/listener';

const mockSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, string>>>()
  .mockResolvedValue({ id: 'msg-1' });
const mockSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, string>>>()
  .mockResolvedValue({ id: 'msg-1' });

jest.unstable_mockModule('../src/webex', () => ({
  sendMessage: mockSendMessage,
  sendCard: mockSendCard,
  getMessage: jest.fn(),
  getAttachmentAction: jest.fn(),
  extractCards: jest.fn(),
  _setMessagesUrl: jest.fn(),
}));

const { WgrokRouterBot } = await import('../src/router-bot');
import { loadCases } from './helpers';

interface RouterBotCases {
  config: { domains: string[] };
  routes: Record<string, string>;
  cases: Array<{
    name: string;
    sender: string;
    text: string;
    cards: unknown[];
    expect_send: boolean;
    expected_reply_to?: string;
    expected_reply_text?: string;
    expected_reply_card?: unknown;
    use_routes?: boolean;
  }>;
}

const CASES = loadCases<RouterBotCases>('router_bot_cases.json');

function fakeMsg(sender: string, text: string, html = '', roomId = '', roomType = ''): IncomingMessage {
  return { sender, text, html, roomId, roomType, msgId: 'test-msg-id', platform: 'webex', cards: [] };
}

describe('WgrokRouterBot', () => {
  beforeEach(() => {
    mockSendMessage.mockClear();
    mockSendCard.mockClear();
  });

  it.each(CASES.cases)('$name', async (tc) => {
    const routes = tc.use_routes ? CASES.routes : {};
    const bot = new WgrokRouterBot({
      webexToken: 'fake-token',
      domains: CASES.config.domains,
      debug: false,
      routes,
      platformTokens: { webex: ['fake-token'] },
      webhookPort: null,
      webhookSecret: null,
    });

    const msg = fakeMsg(tc.sender, tc.text);
    await bot.onMessageWithCards(msg, tc.cards);

    if (tc.expect_send) {
      const called = mockSendMessage.mock.calls.length > 0 || mockSendCard.mock.calls.length > 0;
      expect(called).toBe(true);

      if (tc.expected_reply_card) {
        expect(mockSendCard).toHaveBeenCalledTimes(1);
        const [, to, text] = mockSendCard.mock.calls[0] as [string, string, string];
        expect(to).toBe(tc.expected_reply_to);
        expect(text).toBe(tc.expected_reply_text);
      } else {
        expect(mockSendMessage).toHaveBeenCalledTimes(1);
        const [, to, text] = mockSendMessage.mock.calls[0] as [string, string, string];
        expect(to).toBe(tc.expected_reply_to);
        expect(text).toBe(tc.expected_reply_text);
      }
    } else {
      expect(mockSendMessage).not.toHaveBeenCalled();
      expect(mockSendCard).not.toHaveBeenCalled();
    }
  });

  describe('pause/resume', () => {
    it('pause command is recognized', async () => {
      const bot = new WgrokRouterBot({
        webexToken: 'fake-token',
        domains: ['example.com'],
        debug: false,
        routes: {},
        platformTokens: { webex: ['fake-token'] },
        webhookPort: null,
        webhookSecret: null,
      });

      const msg = fakeMsg('endpoint@example.com', './pause');
      await bot.onMessageWithCards(msg, []);

      expect(mockSendMessage).not.toHaveBeenCalled();
    });

    it('pause buffers messages sent back to paused endpoint', async () => {
      const bot = new WgrokRouterBot({
        webexToken: 'fake-token',
        domains: ['example.com'],
        debug: false,
        routes: {},
        platformTokens: { webex: ['fake-token'] },
        webhookPort: null,
        webhookSecret: null,
      });

      // Endpoint pauses (in Mode B, this is the same as sender)
      const pauseMsg = fakeMsg('endpoint@example.com', './pause');
      await bot.onMessageWithCards(pauseMsg, []);

      mockSendMessage.mockClear();

      // Send echo from endpoint to itself (Mode B echo back)
      const echoMsg = fakeMsg('endpoint@example.com', './echo:slug:from:-:test');
      await bot.onMessageWithCards(echoMsg, []);

      // Should buffer, not send
      expect(mockSendMessage).not.toHaveBeenCalled();
    });

    it('resume flushes buffered messages', async () => {
      const bot = new WgrokRouterBot({
        webexToken: 'fake-token',
        domains: ['example.com'],
        debug: false,
        routes: {},
        platformTokens: { webex: ['fake-token'] },
        webhookPort: null,
        webhookSecret: null,
      });

      // Endpoint pauses
      const pauseMsg = fakeMsg('endpoint@example.com', './pause');
      await bot.onMessageWithCards(pauseMsg, []);

      // Buffer some messages
      const echoMsg1 = fakeMsg('endpoint@example.com', './echo:slug:from:-:msg1');
      await bot.onMessageWithCards(echoMsg1, []);

      const echoMsg2 = fakeMsg('endpoint@example.com', './echo:slug:from:-:msg2');
      await bot.onMessageWithCards(echoMsg2, []);

      mockSendMessage.mockClear();

      // Resume
      const resumeMsg = fakeMsg('endpoint@example.com', './resume');
      await bot.onMessageWithCards(resumeMsg, []);

      // Should flush both buffered messages
      expect(mockSendMessage).toHaveBeenCalledTimes(2);
      const [, to1, text1] = mockSendMessage.mock.calls[0] as [string, string, string];
      expect(to1).toBe('endpoint@example.com');
      expect(text1).toBe('slug:from:-:msg1');

      const [, to2, text2] = mockSendMessage.mock.calls[1] as [string, string, string];
      expect(to2).toBe('endpoint@example.com');
      expect(text2).toBe('slug:from:-:msg2');
    });

    it('pause does not affect messages to other endpoints', async () => {
      const bot = new WgrokRouterBot({
        webexToken: 'fake-token',
        domains: ['example.com'],
        debug: false,
        routes: {},
        platformTokens: { webex: ['fake-token'] },
        webhookPort: null,
        webhookSecret: null,
      });

      // endpoint1 pauses
      const pauseMsg = fakeMsg('endpoint1@example.com', './pause');
      await bot.onMessageWithCards(pauseMsg, []);

      mockSendMessage.mockClear();

      // Send echo from endpoint2 (not paused)
      const echoMsg = fakeMsg('endpoint2@example.com', './echo:slug:from:-:test');
      await bot.onMessageWithCards(echoMsg, []);

      // Should send normally
      expect(mockSendMessage).toHaveBeenCalledTimes(1);
      const [, to] = mockSendMessage.mock.calls[0] as [string, string];
      expect(to).toBe('endpoint2@example.com');
    });
  });
});
