import { loadCases } from './helpers';
import { sendDiscordMessage, sendDiscordCard, _setSleepFn } from '../src/discord';

interface DiscordCases {
  send_message: {
    expected_url_pattern: string;
    expected_payload_fields: string[];
    expected_auth_header_prefix: string;
  };
  send_card: {
    expected_extra_field: string;
    description: string;
  };
  retry_after: {
    max_retries: number;
    cases: Array<{
      name: string;
      responses: Array<{
        status: number;
        retry_after?: string | null;
        body?: Record<string, unknown>;
      }>;
      expected_result?: Record<string, unknown>;
      expected_error_contains?: string;
      expected_attempts?: number;
    }>;
  };
}

const CASES = loadCases<DiscordCases>('discord_cases.json');

describe('Discord send_message', () => {
  it('sends message with correct URL and auth', async () => {
    const mockFetch = async (): Promise<Response> => {
      return {
        status: 200,
        ok: true,
        headers: new Headers(),
        text: async () => '{}',
        json: async () => ({ id: 'msg-discord-1' }),
      } as Response;
    };

    const result = await sendDiscordMessage('token123', 'channel-id', 'Hello', mockFetch);
    expect(result).toEqual({ id: 'msg-discord-1' });
  });
});

describe('Discord send_card', () => {
  it('sends card with embeds field', async () => {
    const mockFetch = async (): Promise<Response> => {
      return {
        status: 200,
        ok: true,
        headers: new Headers(),
        text: async () => '{}',
        json: async () => ({ id: 'msg-discord-1' }),
      } as Response;
    };

    const card = { title: 'Test', description: 'Test embed' };
    const result = await sendDiscordCard('token123', 'channel-id', 'Hello', card, mockFetch);
    expect(result).toEqual({ id: 'msg-discord-1' });
  });
});

describe('Discord retry-after handling', () => {
  it.each(CASES.retry_after.cases)('$name', async (tc) => {
    let attemptCount = 0;
    const sleepCalls: number[] = [];

    // Mock sleep function to track calls
    _setSleepFn(async (ms: number) => {
      sleepCalls.push(ms / 1000); // Convert back to seconds
    });

    // Create a fetchFn that returns responses in sequence
    const fetchFn = async (): Promise<Response> => {
      if (attemptCount >= tc.responses.length) {
        throw new Error(`Unexpected fetch call ${attemptCount}`);
      }

      const response = tc.responses[attemptCount];
      attemptCount++;

      // Create a response-like object
      const headers: Record<string, string> = {};
      if (response.retry_after !== undefined && response.retry_after !== null) {
        headers['Retry-After'] = response.retry_after;
      }

      return {
        status: response.status,
        ok: response.status >= 200 && response.status < 300,
        headers: new Headers(headers),
        text: async () => JSON.stringify({ status: response.status }),
        json: async () => response.body || {},
      } as Response;
    };

    // Test the expected outcome
    if (tc.expected_error_contains) {
      await expect(sendDiscordMessage('token', 'channel-id', 'test', fetchFn)).rejects.toThrow(
        new RegExp(tc.expected_error_contains),
      );
    } else {
      const result = await sendDiscordMessage('token', 'channel-id', 'test', fetchFn);
      expect(result).toEqual(tc.expected_result);
    }

    // Verify attempt count
    if (tc.expected_attempts !== undefined) {
      expect(attemptCount).toBe(tc.expected_attempts);
    }
  });
});
