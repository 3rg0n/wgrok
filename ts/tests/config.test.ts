import { loadCases } from './helpers';
import { senderConfigFromEnv, botConfigFromEnv, receiverConfigFromEnv } from '../src/config';

interface ConfigCases {
  sender: {
    valid: { env: Record<string, string>; expected: { webex_token: string; target: string; slug: string; domains: string[]; debug: boolean } };
    missing_token: { env: Record<string, string>; error_contains: string };
    missing_target: { env: Record<string, string>; error_contains: string };
    debug_defaults_false: { env: Record<string, string>; expected_debug: boolean };
    domains_optional: { env: Record<string, string>; expected_domains: string[] };
  };
  bot: {
    valid: { env: Record<string, string>; expected: { webex_token: string; domains: string[] } };
    missing_domains: { env: Record<string, string>; error_contains: string };
  };
  receiver: {
    valid: { env: Record<string, string>; expected: { webex_token: string; slug: string; domains: string[] } };
  };
  debug_truthy_values: string[];
  debug_falsy_values: string[];
}

const CASES = loadCases<ConfigCases>('config_cases.json');

function setEnv(env: Record<string, string>): void {
  // Clear WGROK_ vars
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('WGROK_')) {
      delete process.env[key];
    }
  }
  for (const [k, v] of Object.entries(env)) {
    process.env[k] = v;
  }
}

afterEach(() => {
  for (const key of Object.keys(process.env)) {
    if (key.startsWith('WGROK_')) {
      delete process.env[key];
    }
  }
});

describe('senderConfigFromEnv', () => {
  it('valid', () => {
    setEnv(CASES.sender.valid.env);
    const cfg = senderConfigFromEnv();
    expect(cfg.webexToken).toBe(CASES.sender.valid.expected.webex_token);
    expect(cfg.target).toBe(CASES.sender.valid.expected.target);
    expect(cfg.slug).toBe(CASES.sender.valid.expected.slug);
    expect(cfg.domains).toEqual(CASES.sender.valid.expected.domains);
    expect(cfg.debug).toBe(CASES.sender.valid.expected.debug);
  });

  it('missing token', () => {
    setEnv(CASES.sender.missing_token.env);
    expect(() => senderConfigFromEnv()).toThrow(new RegExp(CASES.sender.missing_token.error_contains, 'i'));
  });

  it('missing target', () => {
    setEnv(CASES.sender.missing_target.env);
    expect(() => senderConfigFromEnv()).toThrow(new RegExp(CASES.sender.missing_target.error_contains, 'i'));
  });

  it('debug defaults false', () => {
    setEnv(CASES.sender.debug_defaults_false.env);
    expect(senderConfigFromEnv().debug).toBe(CASES.sender.debug_defaults_false.expected_debug);
  });

  it('domains optional', () => {
    setEnv(CASES.sender.domains_optional.env);
    expect(senderConfigFromEnv().domains).toEqual(CASES.sender.domains_optional.expected_domains);
  });
});

describe('botConfigFromEnv', () => {
  it('valid', () => {
    setEnv(CASES.bot.valid.env);
    const cfg = botConfigFromEnv();
    expect(cfg.webexToken).toBe(CASES.bot.valid.expected.webex_token);
    expect(cfg.domains).toEqual(CASES.bot.valid.expected.domains);
  });

  it('missing domains', () => {
    setEnv(CASES.bot.missing_domains.env);
    expect(() => botConfigFromEnv()).toThrow(new RegExp(CASES.bot.missing_domains.error_contains, 'i'));
  });
});

describe('receiverConfigFromEnv', () => {
  it('valid', () => {
    setEnv(CASES.receiver.valid.env);
    const cfg = receiverConfigFromEnv();
    expect(cfg.webexToken).toBe(CASES.receiver.valid.expected.webex_token);
    expect(cfg.slug).toBe(CASES.receiver.valid.expected.slug);
    expect(cfg.domains).toEqual(CASES.receiver.valid.expected.domains);
  });
});
