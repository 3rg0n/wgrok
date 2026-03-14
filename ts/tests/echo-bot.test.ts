import { jest } from '@jest/globals';
import type { DecryptedMessage, MercuryActivity } from 'webex-message-handler';

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

const { WgrokEchoBot } = await import('../src/echo-bot');
import { loadCases } from './helpers';

interface EchoBotCases {
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

const CASES = loadCases<EchoBotCases>('echo_bot_cases.json');

function fakeMsg(sender: string, text: string): DecryptedMessage {
  return {
    id: 'test-msg-id',
    roomId: 'room-abc',
    personId: 'person-123',
    personEmail: sender,
    text,
    created: new Date().toISOString(),
    raw: {} as MercuryActivity,
  };
}

describe('WgrokEchoBot', () => {
  beforeEach(() => {
    mockSendMessage.mockClear();
    mockSendCard.mockClear();
  });

  it.each(CASES.cases)('$name', async (tc) => {
    const routes = tc.use_routes ? CASES.routes : {};
    const bot = new WgrokEchoBot({
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
});
