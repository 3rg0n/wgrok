import { loadCases } from './helpers';
import { Allowlist } from '../src/allowlist';

interface AllowlistCases {
  cases: Array<{ name: string; patterns: string[]; email: string; expected: boolean }>;
}

const CASES = loadCases<AllowlistCases>('allowlist_cases.json');

describe('Allowlist', () => {
  it.each(CASES.cases)('$name', (tc) => {
    const al = new Allowlist(tc.patterns);
    expect(al.isAllowed(tc.email)).toBe(tc.expected);
  });
});
