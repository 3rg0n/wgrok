import { jest } from '@jest/globals';

const mockWebexSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ id: 'webex-1' });
const mockWebexSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ id: 'webex-1' });

const mockSlackSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ ok: true, ts: '1234.5678' });
const mockSlackSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ ok: true, ts: '1234.5678' });

const mockDiscordSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ id: 'discord-1' });
const mockDiscordSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ id: 'discord-1' });

const mockIrcSendMessage = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ status: 'sent', target: 'target' });
const mockIrcSendCard = jest.fn<(...args: unknown[]) => Promise<Record<string, unknown>>>()
  .mockResolvedValue({ status: 'sent', target: 'target' });

jest.unstable_mockModule('../src/webex', () => ({
  sendMessage: mockWebexSendMessage,
  sendCard: mockWebexSendCard,
  getMessage: jest.fn(),
  getAttachmentAction: jest.fn(),
  extractCards: jest.fn(),
  _setMessagesUrl: jest.fn(),
}));

jest.unstable_mockModule('../src/slack', () => ({
  sendSlackMessage: mockSlackSendMessage,
  sendSlackCard: mockSlackSendCard,
  _setSlackUrl: jest.fn(),
  _setSleepFn: jest.fn(),
}));

jest.unstable_mockModule('../src/discord', () => ({
  sendDiscordMessage: mockDiscordSendMessage,
  sendDiscordCard: mockDiscordSendCard,
  _setSleepFn: jest.fn(),
}));

jest.unstable_mockModule('../src/irc', () => ({
  sendIRCMessage: mockIrcSendMessage,
  sendIRCCard: mockIrcSendCard,
  parseIRCConnectionString: jest.fn(),
}));

const { platformSendMessage, platformSendCard } = await import('../src/platform');
import { loadCases } from './helpers';

interface PlatformCases {
  description: string;
  cases: Array<{
    name: string;
    platform: string;
    expected_module?: string;
    expected_error?: boolean;
  }>;
}

const CASES = loadCases<PlatformCases>('platform_dispatch_cases.json');

describe('Platform dispatcher', () => {
  beforeEach(() => {
    mockWebexSendMessage.mockClear();
    mockWebexSendCard.mockClear();
    mockSlackSendMessage.mockClear();
    mockSlackSendCard.mockClear();
    mockDiscordSendMessage.mockClear();
    mockDiscordSendCard.mockClear();
    mockIrcSendMessage.mockClear();
    mockIrcSendCard.mockClear();
  });

  it.each(CASES.cases)('$name', async (tc) => {
    if (tc.expected_error) {
      await expect(platformSendMessage(tc.platform, 'token', 'target', 'text')).rejects.toThrow();
    } else if (tc.expected_module === 'webex') {
      await platformSendMessage(tc.platform, 'token', 'target', 'text');
      expect(mockWebexSendMessage).toHaveBeenCalledTimes(1);
    } else if (tc.expected_module === 'slack') {
      await platformSendMessage(tc.platform, 'token', 'target', 'text');
      expect(mockSlackSendMessage).toHaveBeenCalledTimes(1);
    } else if (tc.expected_module === 'discord') {
      await platformSendMessage(tc.platform, 'token', 'target', 'text');
      expect(mockDiscordSendMessage).toHaveBeenCalledTimes(1);
    } else if (tc.expected_module === 'irc') {
      await platformSendMessage(tc.platform, 'token', 'target', 'text');
      expect(mockIrcSendMessage).toHaveBeenCalledTimes(1);
    }
  });

  describe('send_card routing', () => {
    it('routes to correct module for cards', async () => {
      await platformSendCard('webex', 'token', 'target', 'text', { some: 'card' });
      expect(mockWebexSendCard).toHaveBeenCalledTimes(1);

      mockSlackSendCard.mockClear();
      await platformSendCard('slack', 'token', 'target', 'text', { some: 'card' });
      expect(mockSlackSendCard).toHaveBeenCalledTimes(1);

      mockDiscordSendCard.mockClear();
      await platformSendCard('discord', 'token', 'target', 'text', { some: 'card' });
      expect(mockDiscordSendCard).toHaveBeenCalledTimes(1);

      mockIrcSendCard.mockClear();
      await platformSendCard('irc', 'token', 'target', 'text', { some: 'card' });
      expect(mockIrcSendCard).toHaveBeenCalledTimes(1);
    });

    it('throws on invalid platform for cards', async () => {
      await expect(platformSendCard('teams', 'token', 'target', 'text', { some: 'card' })).rejects.toThrow();
    });
  });
});
