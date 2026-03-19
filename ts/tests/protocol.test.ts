import { loadCases } from './helpers';
import { ECHO_PREFIX, formatEcho, parseEcho, isEcho, isPause, isResume, formatResponse, parseResponse, parseFlags, formatFlags } from '../src/protocol';

interface ProtocolCases {
  echo_prefix: string;
  format_echo: Array<{ to: string; from: string; flags: string; payload: string; expected: string }>;
  parse_echo: {
    valid: Array<{ input: string; to: string; from: string; flags: string; payload: string }>;
    errors: Array<{ input: string; error_contains: string }>;
  };
  is_echo: Array<{ input: string; expected: boolean }>;
  is_pause: Array<{ input: string; expected: boolean }>;
  is_resume: Array<{ input: string; expected: boolean }>;
  format_response: Array<{ to: string; from: string; flags: string; payload: string; expected: string }>;
  parse_response: {
    valid: Array<{ input: string; to: string; from: string; flags: string; payload: string }>;
    errors: Array<{ input: string; error_contains: string }>;
  };
  parse_flags: Array<{ input: string; compressed: boolean; encrypted: boolean; chunk_seq: number | null; chunk_total: number | null }>;
  format_flags: Array<{ compressed: boolean; encrypted: boolean; chunk_seq: number | null; chunk_total: number | null; expected: string }>;
  roundtrips: {
    echo: Array<{ to: string; from: string; flags: string; payload: string }>;
    response: Array<{ to: string; from: string; flags: string; payload: string }>;
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
    expect(formatEcho(tc.to, tc.from, tc.flags, tc.payload)).toBe(tc.expected);
  });
});

describe('parseEcho', () => {
  describe('valid', () => {
    it.each(CASES.parse_echo.valid)('$input', (tc) => {
      const result = parseEcho(tc.input);
      expect(result.to).toBe(tc.to);
      expect(result.from).toBe(tc.from);
      expect(result.flags).toBe(tc.flags);
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

describe('isPause', () => {
  it.each(CASES.is_pause)('$input -> $expected', (tc) => {
    expect(isPause(tc.input)).toBe(tc.expected);
  });
});

describe('isResume', () => {
  it.each(CASES.is_resume)('$input -> $expected', (tc) => {
    expect(isResume(tc.input)).toBe(tc.expected);
  });
});

describe('formatResponse', () => {
  it.each(CASES.format_response)('$expected', (tc) => {
    expect(formatResponse(tc.to, tc.from, tc.flags, tc.payload)).toBe(tc.expected);
  });
});

describe('parseResponse', () => {
  describe('valid', () => {
    it.each(CASES.parse_response.valid)('$input', (tc) => {
      const result = parseResponse(tc.input);
      expect(result.to).toBe(tc.to);
      expect(result.from).toBe(tc.from);
      expect(result.flags).toBe(tc.flags);
      expect(result.payload).toBe(tc.payload);
    });
  });

  describe('errors', () => {
    it.each(CASES.parse_response.errors)('$input', (tc) => {
      expect(() => parseResponse(tc.input)).toThrow(new RegExp(tc.error_contains, 'i'));
    });
  });
});

describe('parseFlags', () => {
  it.each(CASES.parse_flags)('$input', (tc) => {
    const result = parseFlags(tc.input);
    expect(result.compressed).toBe(tc.compressed);
    expect(result.encrypted).toBe(tc.encrypted);
    expect(result.chunkSeq).toBe(tc.chunk_seq);
    expect(result.chunkTotal).toBe(tc.chunk_total);
  });
});

describe('formatFlags', () => {
  it.each(CASES.format_flags)('compressed=$compressed encrypted=$encrypted seq=$chunk_seq total=$chunk_total', (tc) => {
    expect(formatFlags(tc.compressed, tc.encrypted, tc.chunk_seq, tc.chunk_total)).toBe(tc.expected);
  });
});

describe('roundtrips', () => {
  describe('echo', () => {
    it.each(CASES.roundtrips.echo)('$to:$from:$flags', (tc) => {
      const text = formatEcho(tc.to, tc.from, tc.flags, tc.payload);
      const result = parseEcho(text);
      expect(result.to).toBe(tc.to);
      expect(result.from).toBe(tc.from);
      expect(result.flags).toBe(tc.flags);
      expect(result.payload).toBe(tc.payload);
    });
  });

  describe('response', () => {
    it.each(CASES.roundtrips.response)('$to:$from:$flags', (tc) => {
      const text = formatResponse(tc.to, tc.from, tc.flags, tc.payload);
      const result = parseResponse(text);
      expect(result.to).toBe(tc.to);
      expect(result.from).toBe(tc.from);
      expect(result.flags).toBe(tc.flags);
      expect(result.payload).toBe(tc.payload);
    });
  });
});
