import { loadCases } from './helpers';
import { ECHO_PREFIX, formatEcho, parseEcho, isEcho, formatResponse, parseResponse } from '../src/protocol';

interface ProtocolCases {
  echo_prefix: string;
  format_echo: Array<{ slug: string; payload: string; expected: string }>;
  parse_echo: {
    valid: Array<{ input: string; slug: string; payload: string }>;
    errors: Array<{ input: string; error_contains: string }>;
  };
  is_echo: Array<{ input: string; expected: boolean }>;
  format_response: Array<{ slug: string; payload: string; expected: string }>;
  parse_response: {
    valid: Array<{ input: string; slug: string; payload: string }>;
    errors: Array<{ input: string; error_contains: string }>;
  };
  roundtrips: {
    echo: Array<{ slug: string; payload: string }>;
    response: Array<{ slug: string; payload: string }>;
  };
}

const CASES = loadCases<ProtocolCases>('protocol_cases.json');

describe('ECHO_PREFIX', () => {
  it('matches expected value', () => {
    expect(ECHO_PREFIX).toBe(CASES.echo_prefix);
  });
});

describe('formatEcho', () => {
  it.each(CASES.format_echo)('$expected', (tc) => {
    expect(formatEcho(tc.slug, tc.payload)).toBe(tc.expected);
  });
});

describe('parseEcho', () => {
  describe('valid', () => {
    it.each(CASES.parse_echo.valid)('$input', (tc) => {
      const result = parseEcho(tc.input);
      expect(result.slug).toBe(tc.slug);
      expect(result.payload).toBe(tc.payload);
    });
  });

  describe('errors', () => {
    it.each(CASES.parse_echo.errors)('$input', (tc) => {
      expect(() => parseEcho(tc.input)).toThrow(new RegExp(tc.error_contains, 'i'));
    });
  });
});

describe('isEcho', () => {
  it.each(CASES.is_echo)('$input -> $expected', (tc) => {
    expect(isEcho(tc.input)).toBe(tc.expected);
  });
});

describe('formatResponse', () => {
  it.each(CASES.format_response)('$expected', (tc) => {
    expect(formatResponse(tc.slug, tc.payload)).toBe(tc.expected);
  });
});

describe('parseResponse', () => {
  describe('valid', () => {
    it.each(CASES.parse_response.valid)('$input', (tc) => {
      const result = parseResponse(tc.input);
      expect(result.slug).toBe(tc.slug);
      expect(result.payload).toBe(tc.payload);
    });
  });

  describe('errors', () => {
    it.each(CASES.parse_response.errors)('$input', (tc) => {
      expect(() => parseResponse(tc.input)).toThrow(new RegExp(tc.error_contains, 'i'));
    });
  });
});

describe('roundtrips', () => {
  describe('echo', () => {
    it.each(CASES.roundtrips.echo)('$slug', (tc) => {
      const text = formatEcho(tc.slug, tc.payload);
      const result = parseEcho(text);
      expect(result.slug).toBe(tc.slug);
      expect(result.payload).toBe(tc.payload);
    });
  });

  describe('response', () => {
    it.each(CASES.roundtrips.response)('$slug', (tc) => {
      const text = formatResponse(tc.slug, tc.payload);
      const result = parseResponse(text);
      expect(result.slug).toBe(tc.slug);
      expect(result.payload).toBe(tc.payload);
    });
  });
});
