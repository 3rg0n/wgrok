/**
 * Platform dispatcher — routes send/receive calls to the correct transport module.
 */

import { sendMessage as sendWebexMessage, sendCard as sendWebexCard } from './webex.js';
import { sendSlackMessage, sendSlackCard } from './slack.js';
import { sendDiscordMessage, sendDiscordCard } from './discord.js';
import { sendIRCMessage, sendIRCCard } from './irc.js';

export async function platformSendMessage(
  platform: string,
  token: string,
  target: string,
  text: string,
  fetchFn?: typeof fetch,
): Promise<Record<string, unknown>> {
  if (platform === 'webex') {
    return sendWebexMessage(token, target, text, fetchFn);
  }
  if (platform === 'slack') {
    return sendSlackMessage(token, target, text, fetchFn);
  }
  if (platform === 'discord') {
    return sendDiscordMessage(token, target, text, fetchFn);
  }
  if (platform === 'irc') {
    return sendIRCMessage(token, target, text);
  }
  throw new Error(`Unsupported platform: ${platform}`);
}

export async function platformSendCard(
  platform: string,
  token: string,
  target: string,
  text: string,
  card: unknown,
  fetchFn?: typeof fetch,
): Promise<Record<string, unknown>> {
  if (platform === 'webex') {
    return sendWebexCard(token, target, text, card, fetchFn);
  }
  if (platform === 'slack') {
    return sendSlackCard(token, target, text, card, fetchFn);
  }
  if (platform === 'discord') {
    return sendDiscordCard(token, target, text, card, fetchFn);
  }
  if (platform === 'irc') {
    return sendIRCCard(token, target, text, card);
  }
  throw new Error(`Unsupported platform: ${platform}`);
}
