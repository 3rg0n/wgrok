import { config as dotenvConfig } from 'dotenv';

export interface SenderConfig {
  webexToken: string;
  target: string;
  slug: string;
  domains: string[];
  debug: boolean;
  platform: string;
  encryptKey?: Buffer;
}

export interface BotConfig {
  webexToken: string;
  domains: string[];
  debug: boolean;
  routes: Record<string, string>;
  platformTokens: Record<string, string[]>;
  webhookPort: number | null;
  webhookSecret: string | null;
}

export interface ReceiverConfig {
  webexToken: string;
  slug: string;
  domains: string[];
  debug: boolean;
  platform: string;
  encryptKey?: Buffer;
}

function envRequire(name: string): string {
  const val = process.env[name];
  if (!val) {
    throw new Error(`Required environment variable ${name} is not set`);
  }
  return val;
}

function parseDomains(raw: string | undefined): string[] {
  if (!raw) return [];
  return raw
    .split(',')
    .map((d) => d.trim())
    .filter((d) => d.length > 0);
}

function parseDebug(raw: string | undefined): boolean {
  if (!raw) return false;
  const v = raw.trim().toLowerCase();
  return v === 'true' || v === '1' || v === 'yes';
}

export function parseRoutes(raw: string | undefined): Record<string, string> {
  if (!raw) return {};
  const routes: Record<string, string> = {};
  const pairs = raw.split(',').map((p) => p.trim()).filter((p) => p.length > 0);
  for (const pair of pairs) {
    const colonIdx = pair.indexOf(':');
    if (colonIdx > 0) {
      const slug = pair.substring(0, colonIdx).trim();
      const target = pair.substring(colonIdx + 1).trim();
      if (slug.length > 0 && target.length > 0) {
        routes[slug] = target;
      }
    }
  }
  return routes;
}

export function parsePlatformTokens(): Record<string, string[]> {
  const tokens: Record<string, string[]> = {};
  const platforms = ['webex', 'slack', 'discord', 'irc'];
  for (const platform of platforms) {
    const envVarName = `WGROK_${platform.toUpperCase()}_TOKENS`;
    const raw = process.env[envVarName];
    if (raw) {
      const tokenList = raw
        .split(',')
        .map((t) => t.trim())
        .filter((t) => t.length > 0);
      if (tokenList.length > 0) {
        tokens[platform] = tokenList;
      }
    }
  }
  return tokens;
}

function parseEncryptKey(raw: string | undefined): Buffer | undefined {
  if (!raw) return undefined;
  try {
    return Buffer.from(raw, 'base64');
  } catch {
    throw new Error('WGROK_ENCRYPT_KEY must be valid base64');
  }
}

export function senderConfigFromEnv(envFile?: string): SenderConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  return {
    webexToken: envRequire('WGROK_TOKEN'),
    target: envRequire('WGROK_TARGET'),
    slug: envRequire('WGROK_SLUG'),
    domains: parseDomains(process.env.WGROK_DOMAINS),
    debug: parseDebug(process.env.WGROK_DEBUG),
    platform: process.env.WGROK_PLATFORM || 'webex',
    encryptKey: parseEncryptKey(process.env.WGROK_ENCRYPT_KEY),
  };
}

export function botConfigFromEnv(envFile?: string): BotConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  const platformTokens = parsePlatformTokens();

  // If no platform-specific tokens exist, fall back to WGROK_TOKEN as webex token
  let webexToken: string;
  if (Object.keys(platformTokens).length === 0) {
    webexToken = envRequire('WGROK_TOKEN');
    platformTokens['webex'] = [webexToken];
  } else {
    // If platform tokens are set, WGROK_TOKEN is optional
    webexToken = process.env.WGROK_TOKEN || (platformTokens.webex?.[0] || '');
  }

  return {
    webexToken,
    domains: parseDomains(envRequire('WGROK_DOMAINS')),
    debug: parseDebug(process.env.WGROK_DEBUG),
    routes: parseRoutes(process.env.WGROK_ROUTES),
    platformTokens,
    webhookPort: process.env.WGROK_WEBHOOK_PORT ? parseInt(process.env.WGROK_WEBHOOK_PORT, 10) : null,
    webhookSecret: process.env.WGROK_WEBHOOK_SECRET || null,
  };
}

export function receiverConfigFromEnv(envFile?: string): ReceiverConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  return {
    webexToken: envRequire('WGROK_TOKEN'),
    slug: envRequire('WGROK_SLUG'),
    domains: parseDomains(envRequire('WGROK_DOMAINS')),
    debug: parseDebug(process.env.WGROK_DEBUG),
    platform: process.env.WGROK_PLATFORM || 'webex',
    encryptKey: parseEncryptKey(process.env.WGROK_ENCRYPT_KEY),
  };
}
