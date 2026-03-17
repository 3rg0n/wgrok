import { compress, decompress, chunk } from '../src/codec';
import { loadCases } from './helpers';

interface CodecCases {
  roundtrips: Array<{ input: string }>;
  chunking: Array<{
    input: string;
    max_size: number;
    expected_count: number;
    expected_chunks: string[];
    description: string;
  }>;
}

const CASES = loadCases<CodecCases>('codec_cases.json');

describe('compress/decompress roundtrip', () => {
  it.each(CASES.roundtrips)('roundtrip: $input', (tc) => {
    const compressed = compress(tc.input);
    expect(typeof compressed).toBe('string');
    expect(decompress(compressed)).toBe(tc.input);
  });
});

describe('chunking', () => {
  it.each(CASES.chunking)('$description', (tc) => {
    const chunks = chunk(tc.input, tc.max_size);
    expect(chunks.length).toBe(tc.expected_count);
    expect(chunks).toEqual(tc.expected_chunks);
  });
});
