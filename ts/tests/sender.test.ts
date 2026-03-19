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
  config: { token: string; target: string; slug: string; from_slug: string };
  cases: Array<{
    name: string;
    payload: string;
    card: unknown;
    compress: boolean;
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

    await sender.send(tc.payload, tc.card ?? undefined, tc.compress);

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

  describe('pause/resume', () => {
    it('buffers messages when paused', async () => {
      const sender = new WgrokSender({
        webexToken: CASES.config.token,
        target: CASES.config.target,
        slug: CASES.config.slug,
        domains: ['example.com'],
        debug: false,
        platform: 'webex',
      });

      await sender.pause(false);
      const result = await sender.send('test payload');

      expect(result).toEqual({ buffered: true });
      expect(mockPlatformSendMessage).not.toHaveBeenCalled();
    });

    it('pause with notify sends pause command', async () => {
      const sender = new WgrokSender({
        webexToken: CASES.config.token,
        target: CASES.config.target,
        slug: CASES.config.slug,
        domains: ['example.com'],
        debug: false,
        platform: 'webex',
      });

      await sender.pause(true);

      expect(mockPlatformSendMessage).toHaveBeenCalledTimes(1);
      const [, , target, text] = mockPlatformSendMessage.mock.calls[0] as [string, string, string, string];
      expect(target).toBe(CASES.config.target);
      expect(text).toBe('./pause');
    });

    it('resume flushes buffered messages', async () => {
      const sender = new WgrokSender({
        webexToken: CASES.config.token,
        target: CASES.config.target,
        slug: CASES.config.slug,
        domains: ['example.com'],
        debug: false,
        platform: 'webex',
      });

      await sender.pause(false);
      await sender.send('msg1');
      await sender.send('msg2');

      mockPlatformSendMessage.mockClear();
      await sender.resume(false);

      // Should send resume command (1) + 2 buffered messages = 2 calls total
      // But since notify=false for resume, it should just flush = 2 calls for buffered messages
      expect(mockPlatformSendMessage).toHaveBeenCalledTimes(2);
    });

    it('resume with notify sends resume command then flushes', async () => {
      const sender = new WgrokSender({
        webexToken: CASES.config.token,
        target: CASES.config.target,
        slug: CASES.config.slug,
        domains: ['example.com'],
        debug: false,
        platform: 'webex',
      });

      await sender.pause(false);
      await sender.send('msg1');

      mockPlatformSendMessage.mockClear();
      await sender.resume(true);

      // Should send resume command (1) + 1 buffered message = 2 calls
      expect(mockPlatformSendMessage).toHaveBeenCalledTimes(2);
      const [, , , text1] = mockPlatformSendMessage.mock.calls[0] as [string, string, string, string];
      expect(text1).toBe('./resume');
    });
  });
});
