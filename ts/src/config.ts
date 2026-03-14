import { config as dotenvConfig } from 'dotenv';

export interface SenderConfig {
  webexToken: string;
  target: string;
  slug: string;
  domains: string[];
  debug: boolean;
}

export interface BotConfig {
  webexToken: string;
  domains: string[];
  debug: boolean;
}

export interface ReceiverConfig {
  webexToken: string;
  slug: string;
  domains: string[];
  debug: boolean;
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

export function senderConfigFromEnv(envFile?: string): SenderConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  return {
    webexToken: envRequire('WGROK_TOKEN'),
    target: envRequire('WGROK_TARGET'),
    slug: envRequire('WGROK_SLUG'),
    domains: parseDomains(process.env.WGROK_DOMAINS),
    debug: parseDebug(process.env.WGROK_DEBUG),
  };
}

export function botConfigFromEnv(envFile?: string): BotConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  return {
    webexToken: envRequire('WGROK_TOKEN'),
    domains: parseDomains(envRequire('WGROK_DOMAINS')),
    debug: parseDebug(process.env.WGROK_DEBUG),
  };
}

export function receiverConfigFromEnv(envFile?: string): ReceiverConfig {
  if (envFile) dotenvConfig({ path: envFile, override: true });
  return {
    webexToken: envRequire('WGROK_TOKEN'),
    slug: envRequire('WGROK_SLUG'),
    domains: parseDomains(envRequire('WGROK_DOMAINS')),
    debug: parseDebug(process.env.WGROK_DEBUG),
  };
}
