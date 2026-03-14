import { loadCases } from './helpers';
import { extractCards } from '../src/webex';

interface WebexCases {
  extract_cards: Array<{
    name: string;
    message: Record<string, unknown>;
    expected: unknown[];
  }>;
}

const CASES = loadCases<WebexCases>('webex_cases.json');

describe('extractCards', () => {
  it.each(CASES.extract_cards)('$name', (tc) => {
    expect(extractCards(tc.message)).toEqual(tc.expected);
  });
});
