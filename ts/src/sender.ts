import type { Logger } from 'webex-message-handler';
import type { SenderConfig } from './config.js';
import { compress as codecCompress, encrypt as codecEncrypt, chunk as codecChunk } from './codec.js';
import { getLogger } from './logging.js';
import { ECHO_PREFIX, formatEcho, formatFlags, PAUSE_CMD, RESUME_CMD } from './protocol.js';
import { platformSendMessage, platformSendCard } from './platform.js';

export const PLATFORM_LIMITS: Record<string, number> = {
  webex: 7439,
  slack: 4000,
  discord: 2000,
  irc: 400,
};

export interface SendResult {
  messageId: string | null;
  messageIds: string[];
  platformResponse: Record<string, unknown> | Record<string, unknown>[] | null;
  buffered: boolean;
}

function extractMessageId(platform: string, response: Record<string, unknown>): string | null {
  if (platform === 'webex' || platform === 'discord') {
    return (response.id as string) ?? null;
  }
  if (platform === 'slack') {
    return (response.ts as string) ?? null;
  }
  return null;
}

export class WgrokSender {
  private config: SenderConfig;
  private logger: Logger;
  private paused = false;
  private buffer: Array<{ payload: string; card?: unknown; compress: boolean; fromSlug?: string }> = [];

  constructor(config: SenderConfig) {
    this.config = config;
    this.logger = getLogger(config.debug, 'wgrok.sender');
  }

  async send(payload: string, card?: unknown, compress = false, fromSlug?: string): Promise<SendResult> {
    if (this.paused) {
      this.logger.info('Sender is paused, buffering message');
      if (this.buffer.length >= 1000) {
        this.logger.warn('Pause buffer full (1000), dropping oldest message');
        this.buffer.shift();
      }
      this.buffer.push({ payload, card, compress, fromSlug });
      return { messageId: null, messageIds: [], platformResponse: null, buffered: true };
    }

    const from = fromSlug ?? this.config.slug;
    const encrypted = !!this.config.encryptKey;
    let processedPayload = payload;

    if (compress) {
      processedPayload = codecCompress(payload);
    }

    if (encrypted) {
      processedPayload = codecEncrypt(processedPayload, this.config.encryptKey!);
    }

    const flags = formatFlags(compress, encrypted);
    const text = formatEcho(this.config.slug, from, flags, processedPayload);
    const limit = PLATFORM_LIMITS[this.config.platform] ?? 7439;

    if (Buffer.byteLength(text, 'utf-8') > limit && !card) {
      const overhead =
        Buffer.byteLength(ECHO_PREFIX, 'utf-8') +
        Buffer.byteLength(this.config.slug, 'utf-8') +
        Buffer.byteLength(from, 'utf-8') +
        Buffer.byteLength(flags, 'utf-8') +
        3; // 3 colons
      const maxPayload = limit - overhead;
      const chunks = codecChunk(processedPayload, maxPayload);
      this.logger.info(`Payload exceeds ${limit}B limit, sending ${chunks.length} chunks to ${this.config.target}`);
      const results: Record<string, unknown>[] = [];
      const msgIds: string[] = [];
      for (let i = 0; i < chunks.length; i++) {
        const chunkFlags = formatFlags(compress, encrypted, i + 1, chunks.length);
        const chunkText = formatEcho(this.config.slug, from, chunkFlags, chunks[i]);
        const resp = await platformSendMessage(this.config.platform, this.config.webexToken, this.config.target, chunkText);
        results.push(resp);
        const mid = extractMessageId(this.config.platform, resp);
        if (mid) msgIds.push(mid);
      }
      return {
        messageId: msgIds[0] ?? null,
        messageIds: msgIds,
        platformResponse: results,
        buffered: false,
      };
    }

    this.logger.info(`Sending to ${this.config.target} [slug=${this.config.slug}, len=${processedPayload.length}]`);
    let resp: Record<string, unknown>;
    if (card) {
      this.logger.info('Including card attachment');
      resp = await platformSendCard(this.config.platform, this.config.webexToken, this.config.target, text, card);
    } else {
      resp = await platformSendMessage(this.config.platform, this.config.webexToken, this.config.target, text);
    }
    const mid = extractMessageId(this.config.platform, resp);
    return {
      messageId: mid,
      messageIds: mid ? [mid] : [],
      platformResponse: resp,
      buffered: false,
    };
  }

  async pause(notify = true): Promise<void> {
    this.paused = true;
    if (notify) {
      this.logger.info(`Sending pause command to ${this.config.target}`);
      await platformSendMessage(this.config.platform, this.config.webexToken, this.config.target, PAUSE_CMD);
    } else {
      this.logger.info('Sender paused (no notification)');
    }
  }

  async resume(notify = true): Promise<void> {
    this.paused = false;
    if (notify) {
      this.logger.info(`Sending resume command to ${this.config.target}`);
      await platformSendMessage(this.config.platform, this.config.webexToken, this.config.target, RESUME_CMD);
    } else {
      this.logger.info('Sender resumed (no notification)');
    }

    // Flush buffer
    const buffered = this.buffer.splice(0, this.buffer.length);
    this.logger.info(`Flushing ${buffered.length} buffered messages`);
    for (const item of buffered) {
      await this.send(item.payload, item.card, item.compress, item.fromSlug);
    }
  }
}
