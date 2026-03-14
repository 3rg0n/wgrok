import { jest } from '@jest/globals';

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

const { WgrokSender } = await import('../src/sender');
import { loadCases } from './helpers';

interface SenderCases {
  config: { token: string; target: string; slug: string };
  cases: Array<{
    name: string;
    payload: string;
    card: unknown;
    expected_text: string;
    expected_target: string;
    expected_uses_card: boolean;
  }>;
}

const CASES = loadCases<SenderCases>('sender_cases.json');

describe('WgrokSender', () => {
  beforeEach(() => {
    mockSendMessage.mockClear();
    mockSendCard.mockClear();
  });

  it.each(CASES.cases)('$name', async (tc) => {
    const sender = new WgrokSender({
      webexToken: CASES.config.token,
      target: CASES.config.target,
      slug: CASES.config.slug,
      domains: ['example.com'],
      debug: false,
    });

    await sender.send(tc.payload, tc.card ?? undefined);

    if (tc.expected_uses_card) {
      expect(mockSendCard).toHaveBeenCalledTimes(1);
      const [, target, text] = mockSendCard.mock.calls[0] as [string, string, string];
      expect(text).toBe(tc.expected_text);
      expect(target).toBe(tc.expected_target);
    } else {
      expect(mockSendMessage).toHaveBeenCalledTimes(1);
      const [, target, text] = mockSendMessage.mock.calls[0] as [string, string, string];
      expect(text).toBe(tc.expected_text);
      expect(target).toBe(tc.expected_target);
    }
  });
});
