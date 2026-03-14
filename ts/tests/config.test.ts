import { loadCases } from './helpers';
import { senderConfigFromEnv, botConfigFromEnv, receiverConfigFromEnv } from '../src/config';

interface ConfigCases {
  sender: {
    valid: { env: Record<string, string>; expected: { webex_token: string; target: string; slug: string; domains: string[]; debug: boolean; platform: string } };
    missing_token: { env: Record<string, string>; error_contains: string };
    missing_target: { env: Record<string, string>; error_contains: string };
    debug_defaults_false: { env: Record<string, string>; expected_debug: boolean };
    domains_optional: { env: Record<string, string>; expected_domains: string[] };
    platform_defaults_webex: { env: Record<string, string>; expected_platform: string };
    platform_explicit: { env: Record<string, string>; expected_platform: string };
  };
  bot: {
    valid: { env: Record<string, string>; expected: { webex_token: string; domains: string[] } };
    missing_domains: { env: Record<string, string>; error_contains: string };
    with_routes: { env: Record<string, string>; expected_routes: Record<string, string> };
    routes_empty_when_not_set: { env: Record<string, string>; expected_routes: Record<string, string> };
    with_webhook: { env: Record<string, string>; expected_webhook_port: number; expected_webhook_secret: string };
    webhook_disabled_by_default: { env: Record<string, string>; expected_webhook_port: null; expected_webhook_secret: null };
    with_platform_tokens: { env: Record<string, string>; expected_platform_tokens: Record<string, string[]> };
    fallback_single_token: { env: Record<string, string>; expected_platform_tokens: Record<string, string[]> };
  };
  receiver: {
    valid: { env: Record<string, string>; expected: { webex_token: string; slug: string; domains: string[]; platform: string } };
    platform_explicit: { env: Record<string, string>; expected_platform: string };
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
    expect(cfg.platform).toBe(CASES.sender.valid.expected.platform);
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

  it('platform defaults webex', () => {
    setEnv(CASES.sender.platform_defaults_webex.env);
    expect(senderConfigFromEnv().platform).toBe(CASES.sender.platform_defaults_webex.expected_platform);
  });

  it('platform explicit', () => {
    setEnv(CASES.sender.platform_explicit.env);
    expect(senderConfigFromEnv().platform).toBe(CASES.sender.platform_explicit.expected_platform);
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

  it('with routes', () => {
    setEnv(CASES.bot.with_routes.env);
    const cfg = botConfigFromEnv();
    expect(cfg.routes).toEqual(CASES.bot.with_routes.expected_routes);
  });

  it('routes empty when not set', () => {
    setEnv(CASES.bot.routes_empty_when_not_set.env);
    const cfg = botConfigFromEnv();
    expect(cfg.routes).toEqual(CASES.bot.routes_empty_when_not_set.expected_routes);
  });

  it('with webhook', () => {
    setEnv(CASES.bot.with_webhook.env);
    const cfg = botConfigFromEnv();
    expect(cfg.webhookPort).toBe(CASES.bot.with_webhook.expected_webhook_port);
    expect(cfg.webhookSecret).toBe(CASES.bot.with_webhook.expected_webhook_secret);
  });

  it('webhook disabled by default', () => {
    setEnv(CASES.bot.webhook_disabled_by_default.env);
    const cfg = botConfigFromEnv();
    expect(cfg.webhookPort).toBe(CASES.bot.webhook_disabled_by_default.expected_webhook_port);
    expect(cfg.webhookSecret).toBe(CASES.bot.webhook_disabled_by_default.expected_webhook_secret);
  });

  it('with platform tokens', () => {
    setEnv(CASES.bot.with_platform_tokens.env);
    const cfg = botConfigFromEnv();
    expect(cfg.platformTokens).toEqual(CASES.bot.with_platform_tokens.expected_platform_tokens);
  });

  it('fallback single token', () => {
    setEnv(CASES.bot.fallback_single_token.env);
    const cfg = botConfigFromEnv();
    expect(cfg.platformTokens).toEqual(CASES.bot.fallback_single_token.expected_platform_tokens);
  });
});

describe('receiverConfigFromEnv', () => {
  it('valid', () => {
    setEnv(CASES.receiver.valid.env);
    const cfg = receiverConfigFromEnv();
    expect(cfg.webexToken).toBe(CASES.receiver.valid.expected.webex_token);
    expect(cfg.slug).toBe(CASES.receiver.valid.expected.slug);
    expect(cfg.domains).toEqual(CASES.receiver.valid.expected.domains);
    expect(cfg.platform).toBe(CASES.receiver.valid.expected.platform);
  });

  it('platform explicit', () => {
    setEnv(CASES.receiver.platform_explicit.env);
    const cfg = receiverConfigFromEnv();
    expect(cfg.platform).toBe(CASES.receiver.platform_explicit.expected_platform);
  });
});
