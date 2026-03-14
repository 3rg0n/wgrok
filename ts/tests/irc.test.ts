import { loadCases } from './helpers';
import { parseIRCConnectionString, sendIRCMessage, sendIRCCard } from '../src/irc';

interface IRCCases {
  parse_connection_string: Array<{
    name: string;
    input: string;
    expected?: {
      nick: string;
      password: string;
      server: string;
      port: number;
      channel: string;
    };
    expected_error?: boolean;
  }>;
  send_message: {
    description: string;
  };
}

const CASES = loadCases<IRCCases>('irc_cases.json');

describe('IRC parseConnectionString', () => {
  it.each(CASES.parse_connection_string)('$name', (tc) => {
    if (tc.expected_error) {
      expect(() => parseIRCConnectionString(tc.input)).toThrow();
    } else {
      const result = parseIRCConnectionString(tc.input);
      expect(result).toEqual(tc.expected);
    }
  });
});

describe('IRC sendMessage', () => {
  it('validates connection string and returns status', async () => {
    const result = await sendIRCMessage('nick@server:6697/#channel', 'target', 'text');
    expect(result).toEqual({ status: 'sent', target: 'target' });
  });

  it('throws on invalid connection string', async () => {
    await expect(sendIRCMessage('invalid-string', 'target', 'text')).rejects.toThrow();
  });
});

describe('IRC sendCard', () => {
  it('sends card as message (cards not supported)', async () => {
    const result = await sendIRCCard('nick@server:6697/#channel', 'target', 'text', { some: 'card' });
    expect(result).toEqual({ status: 'sent', target: 'target' });
  });
});
