import { loadCases } from './helpers';
import { extractCards, sendMessage, _setSleepFn } from '../src/webex';

interface WebexCases {
  extract_cards: Array<{
    name: string;
    message: Record<string, unknown>;
    expected: unknown[];
  }>;
  retry_after: {
    max_retries: number;
    description: string;
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
      expected_sleep_seconds?: number[];
    }>;
  };
}

const CASES = loadCases<WebexCases>('webex_cases.json');

describe('extractCards', () => {
  it.each(CASES.extract_cards)('$name', (tc) => {
    expect(extractCards(tc.message)).toEqual(tc.expected);
  });
});

describe('Retry-After handling', () => {
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
      await expect(sendMessage('token', 'test@example.com', 'test', fetchFn)).rejects.toThrow(
        new RegExp(tc.expected_error_contains),
      );
    } else {
      const result = await sendMessage('token', 'test@example.com', 'test', fetchFn);
      expect(result).toEqual(tc.expected_result);
    }

    // Verify attempt count
    if (tc.expected_attempts !== undefined) {
      expect(attemptCount).toBe(tc.expected_attempts);
    }

    // Verify sleep calls
    if (tc.expected_sleep_seconds !== undefined) {
      expect(sleepCalls).toEqual(tc.expected_sleep_seconds);
    }
  });
});
