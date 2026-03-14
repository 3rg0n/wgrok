import { jest } from '@jest/globals';

const mockPlatformSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, string>>>()
  .mockResolvedValue({ id: 'msg-1' });
const mockPlatformSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, string>>>()
  .mockResolvedValue({ id: 'msg-1' });

jest.unstable_mockModule('../src/platform', () => ({
  platformSendMessage: mockPlatformSendMessage,
  platformSendCard: mockPlatformSendCard,
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
    mockPlatformSendMessage.mockClear();
    mockPlatformSendCard.mockClear();
  });

  it.each(CASES.cases)('$name', async (tc) => {
    const sender = new WgrokSender({
      webexToken: CASES.config.token,
      target: CASES.config.target,
      slug: CASES.config.slug,
      domains: ['example.com'],
      debug: false,
      platform: 'webex',
    });

    await sender.send(tc.payload, tc.card ?? undefined);

    if (tc.expected_uses_card) {
      expect(mockPlatformSendCard).toHaveBeenCalledTimes(1);
      const [platform, , target, text] = mockPlatformSendCard.mock.calls[0] as [string, string, string, string];
      expect(platform).toBe('webex');
      expect(text).toBe(tc.expected_text);
      expect(target).toBe(tc.expected_target);
    } else {
      expect(mockPlatformSendMessage).toHaveBeenCalledTimes(1);
      const [platform, , target, text] = mockPlatformSendMessage.mock.calls[0] as [string, string, string, string];
      expect(platform).toBe('webex');
      expect(text).toBe(tc.expected_text);
      expect(target).toBe(tc.expected_target);
    }
  });
});
