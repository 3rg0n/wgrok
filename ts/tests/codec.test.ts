import { compress, decompress, chunk, encrypt, decrypt } from '../src/codec';
import { loadCases } from './helpers';

interface CodecCases {
  roundtrips: Array<{ input: string }>;
  encrypt_roundtrips: Array<{ input: string }>;
  encrypt_test_key: string;
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

describe('encrypt/decrypt roundtrip', () => {
  const key = Buffer.from(CASES.encrypt_test_key, 'base64');
  it.each(CASES.encrypt_roundtrips)('roundtrip: $input', (tc) => {
    const encrypted = encrypt(tc.input, key);
    expect(typeof encrypted).toBe('string');
    expect(decrypt(encrypted, key)).toBe(tc.input);
  });
});

describe('encrypt/decrypt wrong key throws', () => {
  const key = Buffer.from(CASES.encrypt_test_key, 'base64');
  const wrongKey = Buffer.from('pqQ0sgdvfOtafARe3QjM93YE2Qkj0EaBmhCIzEzf2fM=', 'base64');
  it('wrong key throws', () => {
    const encrypted = encrypt('test data', key);
    expect(() => decrypt(encrypted, wrongKey)).toThrow();
  });
});

describe('chunking', () => {
  it.each(CASES.chunking)('$description', (tc) => {
    const chunks = chunk(tc.input, tc.max_size);
    expect(chunks.length).toBe(tc.expected_count);
    expect(chunks).toEqual(tc.expected_chunks);
  });
});
